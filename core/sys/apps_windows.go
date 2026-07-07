//go:build windows

// Package sys: installed-application scanner for the app-picker UI.
// Scans Start Menu shortcuts (.lnk), resolves them to .exe targets via COM
// (IShellLinkW/IPersistFile) and extracts PNG icons for display.
package sys

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"

	"goclashz/core/logger"
)

// AppInfo describes one installed application for the UI app picker.
type AppInfo struct {
	Name    string `json:"name"`    // человекочитаемое имя (из имени ярлыка)
	Exe     string `json:"exe"`     // basename экзешника в нижнем регистре, напр. "telegram.exe" — для правила PROCESS-NAME
	Path    string `json:"path"`    // полный путь к .exe
	IconPNG string `json:"iconPng"` // PNG-иконка в base64 (без префикса data:), "" если не удалось извлечь
}

var (
	appsShell32 = windows.NewLazySystemDLL("shell32.dll")
	appsUser32  = windows.NewLazySystemDLL("user32.dll")
	appsGdi32   = windows.NewLazySystemDLL("gdi32.dll")

	procSHGetFileInfoW     = appsShell32.NewProc("SHGetFileInfoW")
	procGetIconInfo        = appsUser32.NewProc("GetIconInfo")
	procDestroyIcon        = appsUser32.NewProc("DestroyIcon")
	procGetObjectW         = appsGdi32.NewProc("GetObjectW")
	procGetDIBits          = appsGdi32.NewProc("GetDIBits")
	procCreateCompatibleDC = appsGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = appsGdi32.NewProc("DeleteDC")
	procDeleteObject       = appsGdi32.NewProc("DeleteObject")
)

var (
	clsidShellLink  = ole.NewGUID("{00021401-0000-0000-C000-000000000046}")
	iidIShellLinkW  = ole.NewGUID("{000214F9-0000-0000-C000-000000000046}")
	iidIPersistFile = ole.NewGUID("{0000010B-0000-0000-C000-000000000046}")
)

const (
	hrSFalse         = 0x00000001 // S_FALSE
	hrRPCChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
	slgpRawPath      = 0x4        // SLGP_RAWPATH
	stgmRead         = 0x0        // STGM_READ
	shgfiIcon        = 0x100      // SHGFI_ICON
	shgfiLargeIcon   = 0x0        // SHGFI_LARGEICON
	biRGB            = 0          // BI_RGB
	dibRGBColors     = 0          // DIB_RGB_COLORS
	appsMaxPath      = 260
	maxIconDim       = 1024
	vtblSlotRelease  = 2 // IUnknown::Release
	vtblSlotGetPath  = 3 // IShellLinkW::GetPath
	vtblSlotLoad     = 5 // IPersistFile::Load
)

// ListInstalledApps сканирует ярлыки меню Пуск (для всех пользователей и текущего),
// резолвит их в .exe, дедуплицирует по Path, возвращает отсортированный по Name список.
func ListInstalledApps() ([]AppInfo, error) {
	var roots []string
	if pd := os.Getenv("ProgramData"); pd != "" {
		roots = append(roots, filepath.Join(pd, `Microsoft\Windows\Start Menu\Programs`))
	}
	if ad := os.Getenv("APPDATA"); ad != "" {
		roots = append(roots, filepath.Join(ad, `Microsoft\Windows\Start Menu\Programs`))
	}
	if len(roots) == 0 {
		return nil, errors.New("apps: neither ProgramData nor APPDATA is set")
	}

	// Collect .lnk files first (no COM needed for the walk).
	var lnks []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(p), ".lnk") {
				lnks = append(lnks, p)
			}
			return nil
		})
		if err != nil {
			logger.Warnf("apps: walk %s: %v", root, err)
		}
	}

	var (
		apps   []AppInfo
		retErr error
	)
	runOnComThread(func() {
		res, err := newLnkResolver()
		if err != nil {
			retErr = fmt.Errorf("apps: create IShellLink: %w", err)
			return
		}
		defer res.release()

		seen := make(map[string]struct{}, len(lnks))
		for _, lnk := range lnks {
			target, err := res.resolve(lnk)
			if err != nil {
				continue // broken / non-file shortcut
			}
			target = expandEnvWin(strings.TrimSpace(target))
			if target == "" || !strings.EqualFold(filepath.Ext(target), ".exe") {
				continue
			}
			target = filepath.Clean(target)
			exe := strings.ToLower(filepath.Base(target))
			if strings.HasPrefix(exe, "unins") || strings.Contains(exe, "uninstall") {
				continue
			}
			if isSystem32Path(target) {
				continue
			}
			key := strings.ToLower(target)
			if _, dup := seen[key]; dup {
				continue
			}
			if st, err := os.Stat(target); err != nil || st.IsDir() {
				continue // stale shortcut
			}
			seen[key] = struct{}{}

			name := strings.TrimSuffix(filepath.Base(lnk), filepath.Ext(lnk))
			apps = append(apps, AppInfo{
				Name:    name,
				Exe:     exe,
				Path:    target,
				IconPNG: iconPNGBase64(target),
			})
		}
	})
	if retErr != nil {
		return nil, retErr
	}

	sort.Slice(apps, func(i, j int) bool {
		li, lj := strings.ToLower(apps[i].Name), strings.ToLower(apps[j].Name)
		if li != lj {
			return li < lj
		}
		return apps[i].Path < apps[j].Path
	})
	return apps, nil
}

// AppInfoForExe строит AppInfo для произвольного пути к .exe (кнопка "выбрать вручную").
func AppInfoForExe(path string) AppInfo {
	path = expandEnvWin(strings.TrimSpace(path))
	if path != "" {
		path = filepath.Clean(path)
	}
	base := filepath.Base(path)
	info := AppInfo{
		Name: strings.TrimSuffix(base, filepath.Ext(base)),
		Exe:  strings.ToLower(base),
		Path: path,
	}
	runOnComThread(func() {
		info.IconPNG = iconPNGBase64(path)
	})
	return info
}

// --- COM plumbing -----------------------------------------------------------

// runOnComThread runs fn on a dedicated OS thread with COM initialized
// (apartment-threaded) and waits for completion.
func runOnComThread(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		needUninit := true
		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			var oleErr *ole.OleError
			if errors.As(err, &oleErr) {
				switch oleErr.Code() {
				case hrSFalse:
					// already initialized on this thread: still balance with CoUninitialize
				case hrRPCChangedMode:
					needUninit = false // initialized in another mode: usable, do not uninit
				default:
					logger.Warnf("apps: CoInitializeEx: %v", err)
					needUninit = false
				}
			} else {
				logger.Warnf("apps: CoInitializeEx: %v", err)
				needUninit = false
			}
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warnf("apps: COM worker panicked: %v", r)
				}
			}()
			fn()
		}()
		if needUninit {
			ole.CoUninitialize()
		}
	}()
	<-done
}

// comCall invokes a method on a raw COM interface pointer by vtable slot.
func comCall(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	vtbl := *(**[64]uintptr)(obj)
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(obj))
	callArgs = append(callArgs, args...)
	ret, _, _ := syscall.SyscallN(vtbl[slot], callArgs...)
	return ret
}

type win32FindDataW struct {
	FileAttributes    uint32
	CreationTime      windows.Filetime
	LastAccessTime    windows.Filetime
	LastWriteTime     windows.Filetime
	FileSizeHigh      uint32
	FileSizeLow       uint32
	Reserved0         uint32
	Reserved1         uint32
	FileName          [appsMaxPath]uint16
	AlternateFileName [14]uint16
}

// lnkResolver wraps a reusable ShellLink COM object.
// Must only be used on the thread that created it (STA).
type lnkResolver struct {
	link    *ole.IUnknown  // IShellLinkW
	persist unsafe.Pointer // IPersistFile
}

func newLnkResolver() (*lnkResolver, error) {
	unk, err := ole.CreateInstance(clsidShellLink, iidIShellLinkW)
	if err != nil {
		return nil, err
	}
	pf, err := unk.QueryInterface(iidIPersistFile)
	if err != nil {
		unk.Release()
		return nil, err
	}
	// pf is typed *ole.IDispatch but is really a raw IPersistFile pointer;
	// only its address (vtable) is used below.
	return &lnkResolver{link: unk, persist: unsafe.Pointer(pf)}, nil
}

func (r *lnkResolver) release() {
	if r.persist != nil {
		comCall(r.persist, vtblSlotRelease)
		r.persist = nil
	}
	if r.link != nil {
		r.link.Release()
		r.link = nil
	}
}

// resolve loads a .lnk file and returns its raw target path.
func (r *lnkResolver) resolve(lnkPath string) (string, error) {
	wpath, err := windows.UTF16PtrFromString(lnkPath)
	if err != nil {
		return "", err
	}
	if hr := comCall(r.persist, vtblSlotLoad, uintptr(unsafe.Pointer(wpath)), stgmRead); hr != 0 {
		return "", fmt.Errorf("IPersistFile.Load(%s): hr=0x%08x", lnkPath, hr)
	}
	var (
		buf [appsMaxPath]uint16
		fd  win32FindDataW
	)
	hr := comCall(unsafe.Pointer(r.link), vtblSlotGetPath,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&fd)), slgpRawPath)
	if hr != 0 { // S_FALSE: shortcut has no file-system target (UWP, URL, CLSID...)
		return "", fmt.Errorf("IShellLinkW.GetPath(%s): hr=0x%08x", lnkPath, hr)
	}
	return windows.UTF16ToString(buf[:]), nil
}

// --- path helpers -----------------------------------------------------------

// expandEnvWin expands %VAR% references using the Windows API.
func expandEnvWin(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	src, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return s
	}
	buf := make([]uint16, 1024)
	n, err := windows.ExpandEnvironmentStrings(src, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return s
	}
	if int(n) > len(buf) {
		buf = make([]uint16, n)
		n, err = windows.ExpandEnvironmentStrings(src, &buf[0], uint32(len(buf)))
		if err != nil || n == 0 || int(n) > len(buf) {
			return s
		}
	}
	return windows.UTF16ToString(buf)
}

// isSystem32Path reports whether p resides under %WINDIR%\System32.
func isSystem32Path(p string) bool {
	windir := os.Getenv("WINDIR")
	if windir == "" {
		windir = os.Getenv("SystemRoot")
	}
	if windir == "" {
		return false
	}
	sys32 := strings.ToLower(filepath.Clean(filepath.Join(windir, "System32"))) + string(filepath.Separator)
	return strings.HasPrefix(strings.ToLower(filepath.Clean(p))+string(filepath.Separator), sys32) ||
		strings.HasPrefix(strings.ToLower(filepath.Clean(p)), sys32)
}

// --- icon extraction --------------------------------------------------------

type shFileInfoW struct {
	HIcon       windows.Handle
	IIcon       int32
	Attributes  uint32
	DisplayName [appsMaxPath]uint16
	TypeName    [80]uint16
}

type appsIconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  windows.Handle
	HbmColor windows.Handle
}

type gdiBitmap struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// iconPNGBase64 extracts the large icon of an executable and encodes it as
// base64 PNG. Best effort: any failure returns "".
func iconPNGBase64(exePath string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warnf("apps: icon extraction panicked for %s: %v", exePath, r)
			out = ""
		}
	}()
	if exePath == "" {
		return ""
	}
	wpath, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return ""
	}
	var sfi shFileInfoW
	ret, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(wpath)), 0,
		uintptr(unsafe.Pointer(&sfi)), unsafe.Sizeof(sfi),
		shgfiIcon|shgfiLargeIcon)
	if ret == 0 || sfi.HIcon == 0 {
		return ""
	}
	defer procDestroyIcon.Call(uintptr(sfi.HIcon))

	img, err := iconToImage(sfi.HIcon)
	if err != nil {
		logger.Warnf("apps: icon convert %s: %v", exePath, err)
		return ""
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		logger.Warnf("apps: png encode %s: %v", exePath, err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// iconToImage converts an HICON into an image via GetIconInfo + GetDIBits.
func iconToImage(hIcon windows.Handle) (image.Image, error) {
	var ii appsIconInfo
	ret, _, _ := procGetIconInfo.Call(uintptr(hIcon), uintptr(unsafe.Pointer(&ii)))
	if ret == 0 {
		return nil, errors.New("GetIconInfo failed")
	}
	if ii.HbmMask != 0 {
		defer procDeleteObject.Call(uintptr(ii.HbmMask))
	}
	if ii.HbmColor != 0 {
		defer procDeleteObject.Call(uintptr(ii.HbmColor))
	}
	if ii.HbmColor == 0 {
		// Monochrome icon (mask holds stacked AND/XOR planes) — not worth supporting.
		return nil, errors.New("monochrome icon not supported")
	}

	var bm gdiBitmap
	ret, _, _ = procGetObjectW.Call(uintptr(ii.HbmColor), unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
	if ret == 0 {
		return nil, errors.New("GetObject failed")
	}
	w, h := int(bm.Width), int(bm.Height)
	if w <= 0 || h <= 0 || w > maxIconDim || h > maxIconDim {
		return nil, fmt.Errorf("suspicious icon size %dx%d", w, h)
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdc)

	getBits := func(hbm windows.Handle) ([]byte, error) {
		bih := bitmapInfoHeader{
			Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			Width:       int32(w),
			Height:      -int32(h), // negative = top-down
			Planes:      1,
			BitCount:    32,
			Compression: biRGB,
		}
		buf := make([]byte, w*h*4)
		r, _, _ := procGetDIBits.Call(hdc, uintptr(hbm), 0, uintptr(h),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bih)), dibRGBColors)
		if r == 0 {
			return nil, errors.New("GetDIBits failed")
		}
		return buf, nil
	}

	pix, err := getBits(ii.HbmColor) // BGRA rows, top-down
	if err != nil {
		return nil, err
	}

	hasAlpha := false
	premultiplied := true // stays true only if every channel <= alpha
	for i := 0; i < len(pix); i += 4 {
		a := pix[i+3]
		if a != 0 {
			hasAlpha = true
		}
		if pix[i] > a || pix[i+1] > a || pix[i+2] > a {
			premultiplied = false
		}
	}

	var maskPix []byte
	if !hasAlpha && ii.HbmMask != 0 {
		// 32bpp icon without alpha channel: transparency lives in the AND mask
		// (converted by GetDIBits to 32bpp: white = transparent).
		if mp, err := getBits(ii.HbmMask); err == nil {
			maskPix = mp
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		b, g, r, a := pix[i*4], pix[i*4+1], pix[i*4+2], pix[i*4+3]
		if !hasAlpha {
			a = 0xFF
			if maskPix != nil && (maskPix[i*4] != 0 || maskPix[i*4+1] != 0 || maskPix[i*4+2] != 0) {
				a = 0 // masked out => transparent
			}
		} else if premultiplied && a != 0 && a != 0xFF {
			// un-premultiply back to straight alpha for NRGBA
			b = uint8(uint32(b) * 0xFF / uint32(a))
			g = uint8(uint32(g) * 0xFF / uint32(a))
			r = uint8(uint32(r) * 0xFF / uint32(a))
		}
		o := i * 4
		img.Pix[o+0] = r
		img.Pix[o+1] = g
		img.Pix[o+2] = b
		img.Pix[o+3] = a
	}
	return img, nil
}
