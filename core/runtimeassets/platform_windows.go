//go:build windows

package runtimeassets

import "goclashz/core/utils"

const coreBinName = "clash.exe"
const wintunNeeded = true

// validateCoreBinary проверяет, что файл ядра — валидный исполняемый (Windows PE amd64).
func validateCoreBinary(path string) error {
	return utils.ValidateWindowsPE(path, 5*1024*1024, 300*1024*1024)
}

// findSystemCore — на Windows системного ядра из пакетного менеджера не бывает.
func findSystemCore() string { return "" }
