# singbox-tray

Трей-иконка для macOS: запуск/остановка `sing-box` через `brew services`.

## Требования

- macOS
- Xcode Command Line Tools (`xcode-select --install`) — нужны для cgo, который
  использует `getlantern/systray`
- Go 1.22+
- `sing-box` установлен через brew (`brew install sing-box`), т.е. доступна
  команда `brew services start|stop|restart sing-box`

## Структура проекта

```
cmd/singbox-tray/    — точка входа (main), только systray.Run(...)
internal/brew/        — обёртка над `brew services` / `brew --prefix`
internal/sbconfig/     — список конфигов и переключение symlink'а (без знания о brew)
internal/notify/       — уведомления macOS и «открыть папку в приложении»
internal/tray/          — меню трея, оркестрирует три пакета выше
```

`internal/brew` и `internal/sbconfig` не трогают глобальное состояние и не
завязаны друг на друга напрямую, поэтому у них есть юнит-тесты
(`go test ./...`) — без реального brew и без настоящего `~/.config`.

## Сборка (как консольный бинарник — для отладки)

```bash
cd singbox-tray
go mod tidy
go build -o singbox-tray ./cmd/singbox-tray
./singbox-tray
```

Так запущенный из терминала бинарник и будет висеть привязанным к этому
терминалу. Для нормального использования — собери `.app`-бандл (ниже).

## Сборка в .app (без консоли, без иконки в Dock)

```bash
cd singbox-tray
chmod +x package-app.sh
./package-app.sh
```

Скрипт собирает бинарник и упаковывает его в `SingboxTray.app` со своим
`Info.plist` (ключ `LSUIElement` прячет Dock-иконку — остаётся только
значок в строке меню). Сборка идёт с `-trimpath` (в бинарнике не остаётся
абсолютных путей `/Users/<имя>/…`) и `-ldflags "-s -w"` (без отладочных
символов — бинарник примерно на треть меньше). Версия берётся из
`git describe` и вшивается в бинарник и `Info.plist`; переопределяется
через `VERSION=1.2.3 ./package-app.sh`. В конце `.app` получает ad-hoc
подпись (`codesign --sign -`) — без неё на Apple Silicon приложение не
стартует. Результат — `SingboxTray.app`, который можно:

- перетащить в `/Applications`
- запускать двойным кликом — никакого окна терминала не появится
- добавить в Login Items для автозапуска при входе в систему

При первом запуске Gatekeeper может показать предупреждение о
неподписанном приложении — жми правой кнопкой → «Открыть» → «Открыть»
в диалоге (обходится один раз).

В строке меню появится иконка `SB ○` (остановлен) / `SB ●` (запущен).
Если активный конфиг — из папки `~/.config/singbox-tray/configs/`, рядом
показывается его имя: `SB ● home`. Клик по иконке открывает меню:

- **Статус** — текущее состояние (обновляется раз в 10 сек и после каждого действия)
- **Запустить** — `brew services start sing-box`
- **Остановить** — `brew services stop sing-box`
- **Перезапустить** — `brew services restart sing-box`
- **Конфигурация** — выбор активного конфига (если файлы есть в папке)
- **Версия** — версия сборки (из `git describe`)
- **Выход** — закрыть трей-приложение (сам sing-box не трогает)

Ошибки `brew` (например, неудачный restart) показываются нативным
уведомлением macOS — в режиме `.app` консоли нет, и иначе сбой прошёл бы
незаметно.

## Автоматический релиз (GitHub Actions)

При push тега вида `v*` workflow [`.github/workflows/release.yml`](.github/workflows/release.yml)
сам собирает `SingboxTray.app` (`package-app.sh`) и публикует его как
GitHub Release с приложенным `SingboxTray-<тег>.app.zip`:

```bash
git tag v1.1.2
git push origin v1.1.2
```

Сборка идёт на `macos-latest`, поэтому бинарник в релизе — под архитектуру
раннера GitHub (сейчас это Apple Silicon/arm64); под Intel `.app` придётся
собирать локально.

## Автозапуск при входе в систему

Системные настройки → Основные → Элементы входа → добавить бинарник
`singbox-tray`. Либо через `launchd` — если нужно, могу накидать `.plist`.

## Если brew не находится (`brew недоступен` / ошибка при Start/Stop)

Приложение, запущенное через Finder/Login Items, не видит твой шелловый
`PATH` — поэтому в коде `brew` ищется по фиксированным путям:
`/opt/homebrew/bin/brew` (Apple Silicon) и `/usr/local/bin/brew` (Intel).

Проверь свой путь командой в терминале:

```bash
which brew
```

Если он отличается от этих двух — добавь его в `candidatePaths` в
`internal/brew/brew.go` и пересобери (`./package-app.sh`).

## Возможные доработки

- Показывать реальную цветную иконку вместо текстовой (SetIcon/SetTemplateIcon
  с PNG/ICNS через `go:embed`)
- Уведомления через `terminal-notifier` при падении сервиса
- Пункт меню "Открыть логи" (`brew services info sing-box` / `~/Library/Logs`)
