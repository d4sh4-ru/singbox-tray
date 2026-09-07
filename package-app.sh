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

echo "==> go mod tidy"
go mod tidy

echo "==> сборка бинарника"
go build -o "${BIN_NAME}" .

echo "==> сборка .app бандла"
rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

cp "${BIN_NAME}" "${APP_DIR}/Contents/MacOS/${BIN_NAME}"
chmod +x "${APP_DIR}/Contents/MacOS/${BIN_NAME}"
cp Info.plist "${APP_DIR}/Contents/Info.plist"

echo "==> готово: ${APP_DIR}"
echo "Можно перетащить в /Applications или добавить в Login Items."
