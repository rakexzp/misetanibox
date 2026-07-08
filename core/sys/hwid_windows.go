//go:build windows

package sys

import (
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"
)

type DeviceInfo struct {
	HWID  string
	OS    string
	OSVer string
	Model string
}

var (
	deviceInfoOnce   sync.Once
	cachedDeviceInfo DeviceInfo
)

func GetDeviceInfo() DeviceInfo {
	deviceInfoOnce.Do(func() {
		cachedDeviceInfo = DeviceInfo{
			HWID:  readMachineGuid(),
			OS:    "Windows",
			OSVer: readWindowsVersion(),
			Model: readDeviceModel(),
		}
	})
	return cachedDeviceInfo
}

func SubscriptionHeaders() map[string]string {
	info := GetDeviceInfo()
	h := map[string]string{}
	if info.HWID != "" {
		h["x-hwid"] = info.HWID
	}
	h["x-device-os"] = info.OS
	if info.OSVer != "" {
		h["x-ver-os"] = info.OSVer
	}
	if info.Model != "" {
		h["x-device-model"] = info.Model
	}
	return h
}

func readRegString(root registry.Key, path, name string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

func readMachineGuid() string {

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err == nil {
		defer k.Close()
		if v, _, err := k.GetStringValue("MachineGuid"); err == nil {
			return strings.TrimSpace(v)
		}
	}
	return readRegString(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, "MachineGuid")
}

func readWindowsVersion() string {
	const cv = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	product := readRegString(registry.LOCAL_MACHINE, cv, "ProductName")
	display := readRegString(registry.LOCAL_MACHINE, cv, "DisplayVersion")
	build := readRegString(registry.LOCAL_MACHINE, cv, "CurrentBuildNumber")

	if build != "" {
		var buildNum int
		fmt.Sscanf(build, "%d", &buildNum)
		if buildNum >= 22000 && strings.Contains(product, "Windows 10") {
			product = strings.Replace(product, "Windows 10", "Windows 11", 1)
		}
	}

	parts := []string{}
	if product != "" {
		parts = append(parts, product)
	}
	if display != "" {
		parts = append(parts, display)
	}
	if build != "" {
		parts = append(parts, "build "+build)
	}
	return strings.Join(parts, " ")
}

func readDeviceModel() string {
	const bios = `HARDWARE\DESCRIPTION\System\BIOS`
	manufacturer := readRegString(registry.LOCAL_MACHINE, bios, "SystemManufacturer")
	model := readRegString(registry.LOCAL_MACHINE, bios, "SystemProductName")
	switch {
	case manufacturer != "" && model != "" && !strings.HasPrefix(strings.ToLower(model), strings.ToLower(manufacturer)):
		return manufacturer + " " + model
	case model != "":
		return model
	default:
		return manufacturer
	}
}
