// Package xrayconv конвертирует Xray-JSON-конфигурации в mihomo (Clash Meta) YAML.
//
// Конвертер — авторства Глеба Гребенщикова (github.com/Gleb-pro-admin), проект XrayMi.
// Использован с разрешения автора. Внутренние пакаджи вендорены сюда, т.к. Go не
// разрешает импортировать чужой internal/. Обёртка повторяет конвейер оригинального
// CLI: ParseConfigs → Convert → Build → Render.
package xrayconv

import (
	"fmt"

	"goclashz/core/xrayconv/convert"
	"goclashz/core/xrayconv/mihomo"
	"goclashz/core/xrayconv/xray"
)

// LooksLikeXray — быстрая проверка, что байты это конвертируемый Xray-JSON.
// Нужно, чтобы отличить «панель отдала Xray-конфиг» от «пришёл мусор/HTML».
func LooksLikeXray(raw []byte) bool {
	configs, err := xray.ParseConfigs(raw)
	return err == nil && len(configs) > 0
}

// ToMihomoYAML конвертирует Xray-JSON в mihomo-YAML.
func ToMihomoYAML(raw []byte) (string, error) {
	configs, err := xray.ParseConfigs(raw)
	if err != nil {
		return "", fmt.Errorf("не похоже на Xray-конфигурацию: %w", err)
	}
	if len(configs) == 0 {
		return "", fmt.Errorf("в данных нет Xray-конфигураций")
	}

	result, err := convert.Convert(configs, convert.DefaultOptions())
	if err != nil {
		return "", err
	}
	if len(result.Proxies) == 0 {
		return "", fmt.Errorf("не удалось сконвертировать ни одного узла")
	}

	doc := mihomo.Build(result.Proxies, result.Groups, result.Selectable(), mihomo.DefaultDocumentOptions())
	return mihomo.Render(doc, []string{"Сконвертировано из Xray клиентом Misetanibox"})
}
