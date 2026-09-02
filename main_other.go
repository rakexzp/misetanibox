//go:build !windows

package main

import (
	"embed"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"goclashz/core/logger"
	"goclashz/core/runtimeassets"
	"goclashz/core/sys"
	"goclashz/core/utils"
)

//go:embed all:frontend/dist
var assets embed.FS

// bgAlpha — на macOS фон окна прозрачный (виден системный материал), на Linux
// webkit без композитора рисует чёрное вместо прозрачного → непрозрачный.
func bgAlpha() uint8 {
	if runtime.GOOS == "darwin" {
		return 0
	}
	return 255
}

var singleInstanceLockFile *os.File

func hasFlag(flag string) bool {
	for _, a := range os.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func acquireSingleInstanceLock() bool {
	lockPath := filepath.Join(os.TempDir(), "misetanibox.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		logger.Errorf("не удалось открыть lock-файл %s: %v", lockPath, err)

		return true
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}

	singleInstanceLockFile = f
	return true
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("обнаружен Panic программы: %v\nэкстренно очищаем системный прокси...", r)
			sys.ClearOwnedSystemProxy()
			panic(r)
		}
	}()

	// На Linux Wails по умолчанию ставит WebviewGpuPolicyNever (GPU-ускорение вебвью выключено),
	// так что отдельная разгрузка GPU не требуется — проблема нагрева актуальна для Windows.

	exePath, _ := os.Executable()
	isDebugMode := false
	if strings.Contains(filepath.Base(exePath), "-dev") || len(os.Getenv("WAILS_DEV_SERVER")) > 0 {
		isDebugMode = true
		logger.Infof("👉 режим разработки Wails, проверка единственного экземпляра пропущена")
	}

	if err := utils.MigrateLegacyAppDataToInstallData(); err != nil {
		logger.Errorf("не удалось мигрировать старые данные: %v", err)
	}

	if hasFlag("--shutdown-existing") {

		os.Exit(0)
	}

	if hasFlag("--repair-permissions") {
		if err := utils.RepairDataDirPermission(); err != nil {
			logger.Errorf("не удалось исправить права каталога данных: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ права каталога данных успешно исправлены")
		os.Exit(0)
	}

	if hasFlag("--repair-core-layout") {
		runtimeassets.MigrateLegacyAssets()
		if err := utils.RepairCoreBinPermission(); err != nil {
			logger.Errorf("не удалось исправить права каталога ядра: %v", err)
			os.Exit(1)
		}
		logger.Infof("✅ структура и права каталога ядра успешно исправлены")
		os.Exit(0)
	}

	if hasFlag("--install-helper") || hasFlag("--uninstall-helper") || hasFlag("--restart-helper") {
		logger.Infof("управление Helper-службой не поддерживается на Linux")
		os.Exit(0)
	}

	if !isDebugMode {
		if !acquireSingleInstanceLock() {
			logger.Infof("⚠️ Misetanibox уже запущен, выходим...")
			os.Exit(0)
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
		// Linux: безрамочное окно с нашими кнопками. macOS: нативные «светофоры»,
		// заголовок скрыт, контент под титлбаром (TitleBarHiddenInset) — по-Apple.
		Frameless: runtime.GOOS != "darwin",
		// Прозрачное окно + прозрачный webview: сквозь интерфейс виден системный
		// материал macOS (vibrancy), поверх него фронт рисует стекло (mac.css).
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.DefaultAppearance,
			WindowIsTranslucent:  true,
			WebviewIsTransparent: true,
		},

		HideWindowOnClose: false,
		StartHidden:       true,

		BackgroundColour: &options.RGBA{R: r, G: g, B: b, A: bgAlpha()},

		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
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
}
