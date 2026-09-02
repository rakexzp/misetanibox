package clash

import "runtime"

// coreExeName — имя бинаря ядра в core/bin по платформе. На Windows «clash.exe»,
// на macOS/Linux — «clash» (без расширения). Раньше «clash.exe» был захардкожен и
// на darwin ядро не находилось → «Mihomo не установлено» при лежащем бинаре.
func coreExeName() string {
	if runtime.GOOS == "windows" {
		return "clash.exe"
	}
	return "clash"
}
