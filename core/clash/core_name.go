package clash

import "runtime"

// имя бинаря ядра по платформе
func coreExeName() string {
	if runtime.GOOS == "windows" {
		return "clash.exe"
	}
	return "clash"
}
