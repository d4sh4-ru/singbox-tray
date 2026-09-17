// Package notify показывает нативные уведомления macOS и открывает папки во
// внешних приложениях через Launch Services.
package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// Show показывает нативное уведомление macOS. В режиме .app у приложения
// нет консоли, поэтому это основной способ сообщить пользователю об ошибке.
func Show(text string) error {
	// osascript всегда есть в macOS; кавычки в тексте экранируем.
	safe := strings.ReplaceAll(text, `"`, `'`)
	script := fmt.Sprintf(`display notification "%s" with title "SingboxTray"`, safe)
	return exec.Command("osascript", "-e", script).Run()
}

// OpenInApp открывает dir в приложении appName через Launch Services
// (`open -a`), так что не зависит от того, стоит ли CLI приложения в PATH —
// та же проблема, что и с поиском brew у GUI-процессов.
func OpenInApp(dir, appName string) error {
	cmd := exec.Command("open", "-a", appName, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("open -a %s %s: %w\n%s", appName, dir, err, out)
	}
	return nil
}
