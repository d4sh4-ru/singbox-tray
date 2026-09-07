#!/usr/bin/env bash
# package-app.sh — собирает singbox-tray и упаковывает в SingboxTray.app,
# чтобы запуск шёл без консольного окна и без иконки в Dock.
#
# Запускать на macOS из папки singbox-tray:
#   chmod +x package-app.sh
#   ./package-app.sh
#
# Результат: ./SingboxTray.app — можно перетащить в /Applications
# или добавить в Login Items.

set -euo pipefail

APP_NAME="SingboxTray"
BIN_NAME="singbox-tray"
APP_DIR="${APP_NAME}.app"

# Версию берём из git (тег или короткий хэш), чтобы можно было понять,
# из какого коммита собран установленный .app. Переопределяется через
# переменную окружения: VERSION=1.2.3 ./package-app.sh
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

echo "==> go mod tidy"
go mod tidy

echo "==> сборка бинарника (version=${VERSION})"
# -trimpath      убирает из бинарника абсолютные пути сборки
#                (/Users/<имя>/go/pkg/mod/...) — и чище, и не светит юзернейм.
# -s -w          выкидывают таблицу символов и DWARF — бинарник примерно
#                на треть меньше (для трея отладочные символы не нужны).
# CGO_ENABLED=1  обязателен: getlantern/systray под macOS завязан на cgo.
CGO_ENABLED=1 go build \
  -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${BIN_NAME}" .

echo "==> сборка .app бандла"
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

cp "${BIN_NAME}" "${APP_DIR}/Contents/MacOS/${BIN_NAME}"
chmod +x "${APP_DIR}/Contents/MacOS/${BIN_NAME}"
cp Info.plist "${APP_DIR}/Contents/Info.plist"

# Проставляем версию в Info.plist собранного бандла (исходный файл не трогаем).
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${VERSION}" \
  "${APP_DIR}/Contents/Info.plist" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${VERSION}" \
  "${APP_DIR}/Contents/Info.plist" 2>/dev/null || true

# Ad-hoc подпись. На Apple Silicon немодифицированный неподписанный бинарник
# система убивает при запуске (codesign требуется всегда). Своей учётки
# разработчика тут не нужно — "-" это ad-hoc подпись, её достаточно, чтобы
# .app вообще стартовал; Gatekeeper при первом запуске всё равно спросит.
echo "==> ad-hoc codesign"
codesign --force --deep --sign - "${APP_DIR}"

echo "==> готово: ${APP_DIR} (version=${VERSION})"
echo "Можно перетащить в /Applications или добавить в Login Items."
