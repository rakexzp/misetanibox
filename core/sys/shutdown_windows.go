//go:build windows

package sys

import (
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procCreateEventW = kernel32.NewProc("CreateEventW")
	procSetEvent     = kernel32.NewProc("SetEvent")
)

func RequestExistingInstanceQuit() {
	eventName, _ := syscall.UTF16PtrFromString("Global\\GoclashZ_Shutdown_Event")
	handle, _, _ := procCreateEventW.Call(
		0,
		1,
		0,
		uintptr(unsafe.Pointer(eventName)),
	)
	if handle != 0 {
		procSetEvent.Call(handle)
		windows.CloseHandle(windows.Handle(handle))
	}
}

func ListenForShutdownEvent() {
	eventName, _ := syscall.UTF16PtrFromString("Global\\GoclashZ_Shutdown_Event")
	handle, _, _ := procCreateEventW.Call(
		0,
		1,
		0,
		uintptr(unsafe.Pointer(eventName)),
	)
	if handle != 0 {
		defer windows.CloseHandle(windows.Handle(handle))
		windows.WaitForSingleObject(windows.Handle(handle), windows.INFINITE)
	}
}
