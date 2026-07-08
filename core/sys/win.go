//go:build windows

package sys

import (
	"time"
	"unsafe"

	syswin "golang.org/x/sys/windows"
)

var (
	user32                  = syswin.NewLazySystemDLL("user32.dll")
	procFindWindow          = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procFlashWindowEx       = user32.NewProc("FlashWindowEx")
)

type FLASHWINFO struct {
	CbSize    uint32
	Hwnd      syswin.Handle
	DwFlags   uint32
	UCount    uint32
	DwTimeout uint32
}

const (
	FLASHW_STOP      = 0x00000000
	FLASHW_CAPTION   = 0x00000001
	FLASHW_TRAY      = 0x00000002
	FLASHW_ALL       = 0x00000003
	FLASHW_TIMERNOFG = 0x0000000C

	SW_RESTORE = 9
)

type MainWindowState struct {
	Hwnd       uintptr
	Visible    bool
	Minimized  bool
	Foreground bool
}

func FindMainWindow() uintptr {
	windowName, _ := syswin.UTF16PtrFromString("Misetanibox")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(windowName)))
	return hwnd
}

func GetMainWindowState() MainWindowState {
	hwnd := FindMainWindow()
	if hwnd == 0 {
		return MainWindowState{}
	}

	isVisible, _, _ := procIsWindowVisible.Call(hwnd)
	isMinimized, _, _ := procIsIconic.Call(hwnd)
	fgHwnd, _, _ := procGetForegroundWindow.Call()

	return MainWindowState{
		Hwnd:       hwnd,
		Visible:    isVisible != 0,
		Minimized:  isMinimized != 0,
		Foreground: hwnd == fgHwnd,
	}
}

func IsMainWindowShowing() bool {
	s := GetMainWindowState()
	return s.Hwnd != 0 && s.Visible && !s.Minimized
}

func StopTaskbarFlash(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	finfo := FLASHWINFO{
		CbSize:    uint32(unsafe.Sizeof(FLASHWINFO{})),
		Hwnd:      syswin.Handle(hwnd),
		DwFlags:   FLASHW_STOP,
		UCount:    0,
		DwTimeout: 0,
	}
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&finfo)))
}

func FocusWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	procShowWindow.Call(hwnd, SW_RESTORE)

	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
}

func FlashWindowTwice(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	StopTaskbarFlash(hwnd)

	finfo := FLASHWINFO{
		CbSize:    uint32(unsafe.Sizeof(FLASHWINFO{})),
		Hwnd:      syswin.Handle(hwnd),
		DwFlags:   FLASHW_TRAY,
		UCount:    2,
		DwTimeout: 150,
	}
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&finfo)))

	go func() {
		time.Sleep(800 * time.Millisecond)
		StopTaskbarFlash(hwnd)
	}()
}

func WaitMainWindowHandle() uintptr {
	for i := 0; i < 10; i++ {
		if hwnd := FindMainWindow(); hwnd != 0 {
			return hwnd
		}
		time.Sleep(30 * time.Millisecond)
	}
	return 0
}

func FocusMainWindowAndFlashTwiceWin32Only() {
	hwnd := WaitMainWindowHandle()
	if hwnd == 0 {
		return
	}

	FocusWindow(hwnd)
	FlashWindowTwice(hwnd)
}
