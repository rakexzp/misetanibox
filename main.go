//go:build windows

package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"goclashz/core/logger"
	"goclashz/core/runtimeassets"
	"goclashz/core/sys"
	"goclashz/core/utils"
	syswin "golang.org/x/sys/windows"
	"os/signal"
	"syscall"
)

//go:embed all:frontend/dist
var assets embed.FS

func hasFlag(flag string) bool {
	for _, a := range os.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("обнаружен Panic программы: %v\nэкстренно очищаем системный прокси...", r)
			sys.ClearOwnedSystemProxy()
			panic(r)
		}
	}()

	// разгрузка GPU: точечно отключить композитинг WebView2 (не весь GPU), если включено в настройках
	if v, err := utils.LoadSetting("gpu_saver", false); err == nil && v != nil && *v {
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-gpu-compositing")
	}

	exePath, _ := os.Executable()
	isDebugMode := false
	if filepath.Base(exePath) == "Misetanibox-dev.exe" || len(os.Getenv("WAILS_DEV_SERVER")) > 0 {
		isDebugMode = true
		logger.Infof("👉 режим разработки Wails, проверка единственного экземпляра пропущена")
	}

	if err := utils.MigrateLegacyAppDataToInstallData(); err != nil {
		logger.Errorf("не удалось мигрировать старые данные: %v", err)
	}

	if hasFlag("--shutdown-existing") {
		sys.RequestExistingInstanceQuit()
		os.Exit(0)
	}

	if hasFlag("--repair-permissions") {
		if !sys.CheckAdmin() {
			err := sys.RequestAdmin()
			if err != nil {
				logger.Errorf("не удалось запросить права администратора: %v", err)
			}
			os.Exit(0)
		}

		err := utils.RepairDataDirPermission()
		if err != nil {
			logger.Errorf("не удалось исправить права каталога данных: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ права каталога данных успешно исправлены")
		os.Exit(0)
	}

	if hasFlag("--repair-core-layout") {
		if !sys.CheckAdmin() {
			err := sys.RequestAdmin()
			if err != nil {
				logger.Errorf("не удалось запросить права администратора: %v", err)
			}
			os.Exit(0)
		}

		runtimeassets.MigrateLegacyAssets()
		err := utils.RepairCoreBinPermission()
		if err != nil {
			logger.Errorf("не удалось исправить права каталога ядра: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ структура и права каталога ядра успешно исправлены")
		os.Exit(0)
	}

	if hasFlag("--install-helper") {
		if !sys.CheckAdmin() {
			if err := sys.RequestAdmin(); err != nil {
				logger.Errorf("не удалось запросить права администратора: %v", err)
			}
			os.Exit(0)
		}

		allowedSid := ""
		for i, arg := range os.Args {
			if arg == "--allowed-sid" && i+1 < len(os.Args) {
				allowedSid = os.Args[i+1]
				break
			}
		}

		helperPath := filepath.Join(filepath.Dir(exePath), "GoclashZHelper.exe")
		if err := sys.RecoverHelperServiceForUser(helperPath, allowedSid); err != nil {
			logger.Errorf("не удалось исправить службу Helper: %v", err)
			sys.WriteAdminTaskResult("install-helper", err)
			os.Exit(1)
		}

		logger.Infof("✅ служба Helper успешно установлена/исправлена и запущена")
		os.Exit(0)
	}

	if hasFlag("--uninstall-helper") {
		if !sys.CheckAdmin() {
			if err := sys.RequestAdmin(); err != nil {
				logger.Errorf("не удалось запросить права администратора: %v", err)
			}
			os.Exit(0)
		}
		if err := sys.UninstallHelperService(); err != nil {
			logger.Errorf("не удалось удалить службу Helper: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ служба Helper успешно удалена")
		os.Exit(0)
	}

	if hasFlag("--restart-helper") {
		if !sys.CheckAdmin() {
			if err := sys.RequestAdmin(); err != nil {
				logger.Errorf("не удалось запросить права администратора: %v", err)
			}
			os.Exit(0)
		}
		if err := sys.StopHelperService(); err != nil {
			logger.Warnf("не удалось остановить службу Helper: %v", err)
		}
		if err := sys.StartHelperService(); err != nil {
			logger.Errorf("не удалось запустить службу Helper: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ служба Helper успешно перезапущена")
		os.Exit(0)
	}

	if !isDebugMode {
		mutexName, _ := syswin.UTF16PtrFromString("Global\\GoclashZ_Single_Instance_Mutex")
		mutexHandle, err := syswin.CreateMutex(nil, false, mutexName)

		if err != nil {
			if err == syswin.ERROR_ALREADY_EXISTS {
				logger.Infof("⚠️ Misetanibox уже запущен в фоне, активируем существующее окно...")

				sys.FocusMainWindowAndFlashTwiceWin32Only()

				if mutexHandle != 0 {
					syswin.CloseHandle(mutexHandle)
				}
				os.Exit(0)
			} else {
				logger.Errorf("ошибка при создании мьютекса: %v", err)
			}
		}

		if mutexHandle != 0 {
			defer syswin.CloseHandle(mutexHandle)
		}

		installEmergencyProxyCleanup()
	}

	app := NewApp()

	var r, g, b uint8 = 17, 17, 17

	themeFile := filepath.Join(utils.GetDataDir(), "theme_setting.txt")

	func() {
		if f, err := os.Open(themeFile); err == nil {
			defer f.Close()

			buf := make([]byte, 16)
			n, _ := f.Read(buf)

			if n > 0 && strings.TrimSpace(string(buf[:n])) == "light" {
				r, g, b = 242, 242, 242
			}
		}
	}()

	err := wails.Run(&options.App{
		Title:     "Misetanibox",
		Width:     1024,
		Height:    768,
		MinWidth:  900,
		MinHeight: 600,
		Frameless: true,

		HideWindowOnClose: true,
		StartHidden:       true,

		BackgroundColour: &options.RGBA{R: r, G: g, B: b, A: 255},

		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme:             windows.SystemDefault,
			DisableWindowIcon: false,
		},
	})

	if err != nil {
		logger.Errorf("Error: %v", err)
	}
}

func installEmergencyProxyCleanup() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		logger.Infof("получен сигнал завершения, очищаем системный прокси, установленный Misetanibox...")
		sys.ClearOwnedSystemProxy()
		os.Exit(0)
	}()

	go func() {
		sys.ListenForShutdownEvent()
		logger.Infof("получен сигнал завершения от установщика, экстренно очищаем и выходим...")
		sys.ClearOwnedSystemProxy()
		os.Exit(0)
	}()
}
