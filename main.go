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
//
// Переключение конфигураций:
//
//	Положи несколько *.json файлов в ~/.config/singbox-tray/configs/,
//	например: home.json, work-vpn.json, discord-only.json.
//	В трее появится подменю "Конфигурация" со списком этих файлов.
//	При выборе трей создаёт симлинк
//	  $(brew --prefix)/etc/sing-box/config.json -> выбранный файл
//	и перезапускает sing-box через brew services, чтобы применить конфиг.
//
//	Список конфигов сканируется один раз при старте — если добавил
//	новый файл, перезапусти трей-приложение.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

const serviceName = "sing-box"

// version подставляется при сборке через -ldflags "-X main.version=...".
// В `go run` / `go build` без флагов останется "dev".
var version = "dev"

// GUI-процессы (запущенные через Finder / Login Items, а не из терминала)
// не наследуют PATH из твоего .zshrc/.zprofile, поэтому обычный
// exec.Command("brew", ...) его не находит. Ищем brew явно по типичным
// путям установки Homebrew.
var brewCandidates = []string{
	"/opt/homebrew/bin/brew", // Apple Silicon
	"/usr/local/bin/brew",    // Intel
}

var brewPath = resolveBrewPath()

// brewPrefix кэшируется, т.к. `brew --prefix` дергает Ruby и не бесплатен.
var brewPrefixCache string

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

// brewPrefix возвращает результат `brew --prefix` (например /opt/homebrew).
func brewPrefix() (string, error) {
	if brewPrefixCache != "" {
		return brewPrefixCache, nil
	}
	if brewPath == "" {
		return "", fmt.Errorf("brew не найден")
	}
	out, err := exec.Command(brewPath, "--prefix").Output()
	if err != nil {
		return "", fmt.Errorf("brew --prefix: %w", err)
	}
	brewPrefixCache = strings.TrimSpace(string(out))
	return brewPrefixCache, nil
}

// activeConfigPath — путь, который читает установленный через brew sing-box.
func activeConfigPath() (string, error) {
	prefix, err := brewPrefix()
	if err != nil {
		return "", err
	}
	return filepath.Join(prefix, "etc", serviceName, "config.json"), nil
}

// configsDir — папка с твоими именованными конфигами.
//
// Намеренно не используем os.UserConfigDir(): на macOS он возвращает
// ~/Library/Application Support, а не ~/.config, что неочевидно и
// расходится с тем, куда обычно кладут конфиги руками.
func configsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "singbox-tray", "configs")
}

type configEntry struct {
	name string // без .json, показывается в меню
	path string // абсолютный путь к файлу
}

// loadConfigs сканирует configsDir() на *.json, сортирует по имени.
func loadConfigs() []configEntry {
	dir := configsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []configEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		result = append(result, configEntry{
			name: strings.TrimSuffix(e.Name(), ".json"),
			path: filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

// currentActiveConfigName определяет, на какой конфиг сейчас указывает симлинк.
func currentActiveConfigName(configs []configEntry) string {
	target, err := activeConfigPath()
	if err != nil {
		return ""
	}
	linkDest, err := os.Readlink(target)
	if err != nil {
		return "" // не симлинк (например, дефолтный config.json от формулы) — ничего не подсвечиваем
	}
	for _, c := range configs {
		if linkDest == c.path {
			return c.name
		}
	}
	return ""
}

// openConfigsInZed открывает папку с конфигами в Zed через Launch Services
// (`open -a Zed`), так что не зависит от того, стоит ли Zed CLI в PATH —
// та же логика, что и с поиском brew у GUI-процессов.
func openConfigsInZed() error {
	dir := configsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	cmd := exec.Command("open", "-a", "Zed", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("open -a Zed %s: %w\n%s", dir, err, out)
	}
	return nil
}

// notify показывает нативное уведомление macOS. В режиме .app у приложения нет
// консоли, поэтому fmt.Printf с ошибками никто не видит — дублируем важное сюда.
func notify(text string) {
	// osascript всегда есть в macOS; кавычки в тексте экранируем.
	safe := strings.ReplaceAll(text, `"`, `'`)
	script := fmt.Sprintf(`display notification "%s" with title "SingboxTray"`, safe)
	_ = exec.Command("osascript", "-e", script).Run()
}

// switchConfig переключает симлинк на выбранный конфиг и перезапускает sing-box.
func switchConfig(c configEntry) error {
	target, err := activeConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}
	// Удаляем текущий файл/симлинк (если есть) и создаём новый симлинк.
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("remove %s: %w", target, err)
		}
	}
	if err := os.Symlink(c.path, target); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", target, c.path, err)
	}
	return runBrew("restart")
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

	configs := loadConfigs()
	configItems := make(map[string]*systray.MenuItem, len(configs))
	var mConfigRoot *systray.MenuItem

	if len(configs) > 0 {
		mConfigRoot = systray.AddMenuItem("Конфигурация", "Выбор активного конфига sing-box")
		for _, c := range configs {
			item := mConfigRoot.AddSubMenuItem(c.name, c.path)
			configItems[c.name] = item
		}
	} else {
		note := systray.AddMenuItem("Конфигурация: нет файлов", configsDir())
		note.Disable()
	}
	mOpenZed := systray.AddMenuItem("Открыть папку конфигов в Zed", configsDir())
	systray.AddSeparator()

	mVersion := systray.AddMenuItem("Версия: "+version, "Версия сборки singbox-tray")
	mVersion.Disable()
	mQuit := systray.AddMenuItem("Выход", "Закрыть трей-иконку")

	updateConfigChecks := func() {
		active := currentActiveConfigName(configs)
		for name, item := range configItems {
			if name == active {
				item.Check()
			} else {
				item.Uncheck()
			}
		}
	}

	refresh := func() {
		running, detail := checkStatus()
		active := currentActiveConfigName(configs)

		// В строке меню показываем и статус, и активный конфиг:
		//   "SB ● home"  — запущен на конфиге home
		//   "SB ○"       — остановлен, конфиг не из нашего списка
		dot := "○"
		word := "остановлен"
		if running {
			dot = "●"
			word = "запущен"
		}
		title := "SB " + dot
		if active != "" {
			title += " " + active
		}
		systray.SetTitle(title)

		tooltip := "sing-box: " + word
		if active != "" {
			tooltip += " (" + active + ")"
		}
		if detail != "" {
			tooltip = "sing-box: " + detail
			if active != "" {
				tooltip += " (" + active + ")"
			}
		}
		systray.SetTooltip(tooltip)

		if running {
			mStatus.SetTitle("Статус: запущен")
		} else {
			mStatus.SetTitle("Статус: остановлен")
		}

		updateConfigChecks()
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

	// Отдельная горутина на каждый пункт конфигурации — ClickedCh у
	// каждого MenuItem свой, и его нужно слушать индивидуально.
	for _, c := range configs {
		c := c // захват переменной цикла
		item := configItems[c.name]
		go func() {
			for range item.ClickedCh {
				if err := switchConfig(c); err != nil {
					fmt.Printf("switchConfig(%s): ошибка: %v\n", c.name, err)
					continue
				}
				fmt.Printf("конфигурация переключена на %q\n", c.name)
				refresh()
			}
		}()
	}

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

			case <-mOpenZed.ClickedCh:
				if err := openConfigsInZed(); err != nil {
					fmt.Printf("openConfigsInZed: ошибка: %v\n", err)
				}

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

// runBrew выполняет `brew services <action> sing-box`. Ошибку и возвращает,
// и показывает уведомлением — иначе в режиме .app сбой останется незамеченным.
func runBrew(action string) error {
	if brewPath == "" {
		err := fmt.Errorf("brew не найден (проверь brewCandidates в коде под свою установку)")
		fmt.Println(err)
		notify(err.Error())
		return err
	}
	cmd := exec.Command(brewPath, "services", action, serviceName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("brew services %s %s: ошибка: %v\n%s\n", action, serviceName, err, out)
		notify(fmt.Sprintf("brew services %s не удался: %v", action, err))
		return fmt.Errorf("brew services %s %s: %w", action, serviceName, err)
	}
	fmt.Printf("brew services %s %s: ok\n%s\n", action, serviceName, out)
	return nil
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
