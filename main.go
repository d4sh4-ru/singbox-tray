// singbox-tray — иконка в трее macOS для управления sing-box через brew services.
//
// Сборка (только на macOS, нужен Xcode CLT для cgo):
//
//	go mod tidy
//	go build -o singbox-tray .
//
// Запуск:
//
//	./singbox-tray
//
// При желании автозапуска — положи бинарник куда удобно и добавь в
// Login Items (Системные настройки → Основные → Элементы входа).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

const serviceName = "sing-box"

// GUI-процессы (запущенные через Finder / Login Items, а не из терминала)
// не наследуют PATH из твоего .zshrc/.zprofile, поэтому обычный
// exec.Command("brew", ...) его не находит. Ищем brew явно по типичным
// путям установки Homebrew.
var brewCandidates = []string{
	"/opt/homebrew/bin/brew", // Apple Silicon
	"/usr/local/bin/brew",    // Intel
}

var brewPath = resolveBrewPath()

func resolveBrewPath() string {
	for _, p := range brewCandidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// На случай нестандартной установки — пробуем LookPath с расширенным PATH.
	extendedPath := "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	if cur := os.Getenv("PATH"); cur != "" {
		extendedPath = cur + ":" + extendedPath
	}
	os.Setenv("PATH", extendedPath)
	if p, err := exec.LookPath("brew"); err == nil {
		return p
	}
	return "" // не нашли — покажем ошибку пользователю при первом действии
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("SB ○")
	systray.SetTooltip("sing-box: проверка статуса…")

	mStatus := systray.AddMenuItem("Статус: …", "Текущий статус sing-box")
	mStatus.Disable()
	systray.AddSeparator()

	mStart := systray.AddMenuItem("Запустить", "brew services start sing-box")
	mStop := systray.AddMenuItem("Остановить", "brew services stop sing-box")
	mRestart := systray.AddMenuItem("Перезапустить", "brew services restart sing-box")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "Закрыть трей-иконку")

	refresh := func() {
		running, detail := checkStatus()
		if running {
			systray.SetTitle("SB ●")
			systray.SetTooltip("sing-box: запущен")
			mStatus.SetTitle("Статус: запущен")
		} else {
			systray.SetTitle("SB ○")
			systray.SetTooltip("sing-box: остановлен")
			mStatus.SetTitle("Статус: остановлен")
		}
		if detail != "" {
			systray.SetTooltip("sing-box: " + detail)
		}
	}

	refresh()

	// Периодическое обновление статуса (на случай, если сервис
	// упал сам или его подняли/погасили в обход трея).
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refresh()
		}
	}()

	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				runBrew("start")
				refresh()

			case <-mStop.ClickedCh:
				runBrew("stop")
				refresh()

			case <-mRestart.ClickedCh:
				runBrew("restart")
				refresh()

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// Здесь можно, например, гарантированно остановить sing-box при выходе:
	// runBrew("stop")
}

// runBrew выполняет `brew services <action> sing-box`.
func runBrew(action string) {
	if brewPath == "" {
		fmt.Println("brew не найден (проверь brewCandidates в коде под свою установку)")
		return
	}
	cmd := exec.Command(brewPath, "services", action, serviceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("brew services %s %s: ошибка: %v\n%s\n", action, serviceName, err, out)
		return
	}
	fmt.Printf("brew services %s %s: ok\n%s\n", action, serviceName, out)
}

// checkStatus парсит `brew services list` и определяет, запущен ли sing-box.
func checkStatus() (running bool, detail string) {
	if brewPath == "" {
		return false, "brew не найден"
	}
	cmd := exec.Command(brewPath, "services", "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, "brew недоступен"
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != serviceName {
			continue
		}
		status := ""
		if len(fields) > 1 {
			status = fields[1]
		}
		return status == "started", status
	}
	return false, "не найден в brew services"
}
