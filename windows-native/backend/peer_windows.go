package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"net"
)

func verifyPeer(conn net.Conn, expectedSID string) error {
	file, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return errors.New("pipe handle unavailable")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(file.Fd()), &pid); err != nil {
		return err
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	if user.User.Sid.String() != expectedSID {
		return errors.New("wrong pipe user")
	}
	return nil
}
