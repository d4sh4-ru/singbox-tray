# singbox-tray

Трей-иконка для macOS: запуск/остановка `sing-box` через `brew services`.

## Требования

- macOS
- Xcode Command Line Tools (`xcode-select --install`) — нужны для cgo, который
  использует `getlantern/systray`
- Go 1.22+
- `sing-box` установлен через brew (`brew install sing-box`), т.е. доступна
  команда `brew services start|stop|restart sing-box`

## Сборка (как консольный бинарник — для отладки)

```bash
cd singbox-tray
go mod tidy
go build -o singbox-tray .
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
значок в строке меню). Результат — `SingboxTray.app`, который можно:

- перетащить в `/Applications`
- запускать двойным кликом — никакого окна терминала не появится
- добавить в Login Items для автозапуска при входе в систему

При первом запуске Gatekeeper может показать предупреждение о
неподписанном приложении — жми правой кнопкой → «Открыть» → «Открыть»
в диалоге (обходится один раз).

В строке меню появится иконка `SB ○` (остановлен) / `SB ●` (запущен).
Клик по иконке открывает меню:

- **Статус** — текущее состояние (обновляется раз в 10 сек и после каждого действия)
- **Запустить** — `brew services start sing-box`
- **Остановить** — `brew services stop sing-box`
- **Перезапустить** — `brew services restart sing-box`
- **Выход** — закрыть трей-приложение (сам sing-box не трогает)

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

Если он отличается от этих двух — добавь его в срез `brewCandidates` в
начале `main.go` и пересобери (`./package-app.sh`).

## Возможные доработки

- Показывать реальную цветную иконку вместо текстовой (SetIcon/SetTemplateIcon
  с PNG/ICNS через `go:embed`)
- Уведомления через `terminal-notifier` при падении сервиса
- Пункт меню "Открыть логи" (`brew services info sing-box` / `~/Library/Logs`)
