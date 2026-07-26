<!-- markdownlint-disable MD033 -->
# <img src="docs/assets/logo.png" width="45" alt="Misetanibox Logo" style="vertical-align: middle; margin-right: 10px;"> Misetanibox
<!-- markdownlint-enable MD033 -->

Быстрый клиент Mihomo (Clash Meta) на базе Wails — **Windows и Linux**, с русским интерфейсом, простым (Lite) и продвинутым (Про) режимами, умным выбором сервера и маршрутизацией по приложениям.

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&style=flat-square) ![Wails](https://img.shields.io/badge/Wails-v2.12-red?style=flat-square) ![Vue.js](https://img.shields.io/badge/Vue.js-3.x-4FC08D?logo=vue.js&style=flat-square) ![Windows](https://img.shields.io/badge/Windows-amd64-0078D6?logo=windows&style=flat-square) ![Linux](https://img.shields.io/badge/Linux-amd64-FCC624?logo=linux&logoColor=black&style=flat-square) ![License](https://img.shields.io/badge/License-MIT-black?style=flat-square)

---

Форк [Zzz-IT/GoclashZ](https://github.com/Zzz-IT/GoclashZ). Нативный рендеринг Wails и системная многопоточность Go — минимальный след в памяти вместо Electron.

## Возможности

### Два режима интерфейса
* **Lite** — простой мобильный вид: одна кнопка подключения, список серверов, добавление подписки в один тап. Переключение с «Про» — кнопкой в шапке окна.
* **Про** — полный интерфейс: консоль, узлы, правила, соединения, журнал, редактор конфигурации.

### Подключение
* **TUN** — прозрачный перехват всего трафика. На Windows — через Wintun + изолированную helper-службу (без UAC-окон в работе). На Linux — ядро с `CAP_NET_ADMIN` (разово через pkexec), без root на всё приложение.
* **Системный прокси** — Windows (реестр) и Linux (GNOME gsettings).
* **Умное ядро (Smart)** — опциональная ML-сборка (vernesong/mihomo): выбор сервера по обученной модели с учётом задержки, успешности и нагрузки; `sticky-sessions` не меняет сервер посреди игры.

### Подписки и маршрутизация
* **Подписки** — по ссылке или **через DNS TXT-запись** (обход блокировки адреса подписки); имя тянется из заголовков.
* **Маршрутизация по приложениям** — выбранные приложения мимо VPN или только они через VPN (`PROCESS-NAME`).
* **Идентификация устройства** — заголовки `x-hwid`, `x-device-os`, `x-ver-os`, `x-device-model` при загрузке подписки.
* **Правила и узлы** — редактор правил, флаги стран у серверов, анимация замера задержек, скрытие служебных групп.

### Персонализация и прочее
* Кастомные фоны карточек Консоли (свои картинки, галерея, пресеты-градиенты, редактор положения/масштаба/затемнения).
* Резервные копии `.gocz`, автозапуск, обновления из приложения.

## Установка

| Платформа | Файл |
| :-- | :-- |
| **Windows** | `Misetanibox_win_amd64_Setup.exe` или портативный `.zip`/`.7z` — [Releases](../../releases) |
| **Linux** | `Misetanibox_linux_amd64.tar.gz` — [Releases](../../releases). Нужны `libwebkit2gtk-4.1` и `libgtk-3`. Распаковать и запустить `./Misetanibox/Misetanibox` |

Обновления приходят внутри приложения. Для TUN при первом включении будет запрос прав (helper-служба на Windows / pkexec на Linux).

## Сборка

```bash
cd frontend && npm ci && npm run build   # фронтенд

wails build                              # Windows
wails build -platform linux/amd64 -tags webkit2_41   # Linux (нужны gtk3 + webkit2gtk-4.1)
```

Релизы собираются автоматически: тег `vX.Y.Z` → Windows (`.github/workflows/release.yml`), тег `linux-v*` → Linux (`.github/workflows/linux.yml`).

## Благодарности

Логотип — [@whxteangel](https://t.me/whxteangel). Ядро — [MetaCubeX/mihomo](https://github.com/MetaCubeX/mihomo) и [vernesong/mihomo](https://github.com/vernesong/mihomo) (Smart).

Конвертер конфигураций Xray → mihomo — [@Gleb-pro-admin](https://github.com/Gleb-pro-admin) (проект XrayMi), использован с разрешения автора.

## Лицензия

MIT. Основано на [GoclashZ](https://github.com/Zzz-IT/GoclashZ) © Zzz-IT.
