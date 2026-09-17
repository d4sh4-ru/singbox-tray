// Command singbox-tray — иконка в трее macOS для управления sing-box через
// brew services. Сборка, использование и переключение конфигураций описаны
// в README.md репозитория.
package main

import (
	"github.com/getlantern/systray"

	"singbox-tray/internal/tray"
)

// version подставляется при сборке через -ldflags "-X main.version=...".
// Без флага (`go build ./cmd/singbox-tray` напрямую) остаётся "dev".
var version = "dev"

func main() {
	systray.Run(func() { tray.OnReady(version) }, tray.OnExit)
}
