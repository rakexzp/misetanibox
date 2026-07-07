//go:build windows

package sys

// EnsureTunPrivilege — на Windows TUN поднимает helper-служба, отдельных прав ядру не нужно.
func EnsureTunPrivilege() error { return nil }
