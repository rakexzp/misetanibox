package main

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

// The backend and all directly spawned cores share a kill-on-close job. The job
// handle is non-inheritable: a hard backend crash closes it and kills child core
// processes. This does not restore persistent WinINet proxy settings on crash.
func ownProcessTree() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
