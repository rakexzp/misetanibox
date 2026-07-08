//go:build windows

package sys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func CheckAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		return false
	}
	defer token.Close()

	return token.IsElevated()
}

func IsAdmin() bool {
	return CheckAdmin()
}

func RequestAdmin() error {
	if CheckAdmin() {
		return nil
	}

	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()

	var safeArgs []string
	for _, arg := range os.Args[1:] {
		safeArgs = append(safeArgs, syscall.EscapeArg(arg))
	}
	args := strings.Join(safeArgs, " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		return err
	}

	os.Exit(0)
	return nil
}

func RequestAdminWithArgs(extraArgs string) error {
	if CheckAdmin() {
		return nil
	}

	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(extraArgs)

	var showCmd int32 = 1

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		return err
	}

	os.Exit(0)
	return nil
}

type ShellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         windows.Handle
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     windows.Handle
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    windows.Handle
	HotKey       uint32
	Union        uintptr
	HProcess     windows.Handle
}

var (
	shell32            = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = shell32.NewProc("ShellExecuteExW")
)

const (
	SEE_MASK_NOCLOSEPROCESS = 0x00000040
)

func RunElevatedWithArgsWait(args ...string) error {
	if CheckAdmin() {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	escaped := make([]string, 0, len(args))
	for _, arg := range args {
		escaped = append(escaped, syscall.EscapeArg(arg))
	}
	param := strings.Join(escaped, " ")

	taskID := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	os.Setenv("GOCLASHZ_ADMIN_TASK_ID", taskID)

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argPtr, _ := syscall.UTF16PtrFromString(param)

	var sei ShellExecuteInfo
	sei.CbSize = uint32(unsafe.Sizeof(sei))
	sei.FMask = SEE_MASK_NOCLOSEPROCESS
	sei.LpVerb = verbPtr
	sei.LpFile = exePtr
	sei.LpParameters = argPtr
	sei.NShow = 0

	r1, _, err := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&sei)))
	if r1 == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return fmt.Errorf("не удалось запросить права администратора или запрос отменён: %w", err)
		}
		return fmt.Errorf("не удалось запросить права администратора или запрос отменён")
	}
	if sei.HProcess == 0 {
		return fmt.Errorf("пустой дескриптор административного подпроцесса")
	}
	defer windows.CloseHandle(sei.HProcess)

	_, err = windows.WaitForSingleObject(sei.HProcess, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("не удалось дождаться административного подпроцесса: %w", err)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(sei.HProcess, &exitCode); err != nil {
		return fmt.Errorf("не удалось получить код выхода административного подпроцесса: %w", err)
	}

	if exitCode != 0 {
		if msg := readAdminTaskError(); msg != "" {
			return fmt.Errorf("административный подпроцесс завершился с ошибкой: %s", msg)
		}
		return fmt.Errorf("административный подпроцесс завершился с ошибкой, код выхода: %d", exitCode)
	}

	return nil
}

type AdminTaskResult struct {
	OK        bool   `json:"ok"`
	Operation string `json:"operation"`
	Error     string `json:"error,omitempty"`
}

func getAdminTaskResultPath() string {
	id := os.Getenv("GOCLASHZ_ADMIN_TASK_ID")
	if id == "" {
		id = "default"
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("goclashz_admin_task_%s.json", id))
}

func WriteAdminTaskResult(operation string, err error) {
	res := AdminTaskResult{
		OK:        err == nil,
		Operation: operation,
	}
	if err != nil {
		res.Error = err.Error()
	}
	data, _ := json.Marshal(res)
	_ = os.WriteFile(getAdminTaskResultPath(), data, 0666)
}

func readAdminTaskError() string {
	path := getAdminTaskResultPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_ = os.Remove(path)
	var res AdminTaskResult
	if err := json.Unmarshal(data, &res); err == nil && !res.OK {
		return res.Error
	}
	return ""
}
