// Package tray строит меню трея и оркестрирует brew, sbconfig и notify.
// Сам он не содержит бизнес-логики — только склеивает остальные пакеты и
// реагирует на клики.
package tray

import (
	"fmt"
	"time"

	"github.com/getlantern/systray"

	"singbox-tray/internal/brew"
	"singbox-tray/internal/notify"
	"singbox-tray/internal/sbconfig"
)

// configEditorApp — приложение, в котором открывается папка с конфигами
// (пункт меню "Открыть папку конфигов в ..."). Это чисто UI-решение трея,
// поэтому константа здесь, а не в sbconfig/notify.
const configEditorApp = "Zed"

// OnReady строит меню трея. version — версия сборки для одноимённого пункта
// меню, подставляется вызывающей стороной (main), см. -ldflags в
// package-app.sh.
func OnReady(version string) {
	systray.SetTitle("SB ○")
	systray.SetTooltip("sing-box: проверка статуса…")

	br := brew.New()

	mStatus := systray.AddMenuItem("Статус: …", "Текущий статус sing-box")
	mStatus.Disable()
	systray.AddSeparator()

	mStart := systray.AddMenuItem("Запустить", "brew services start sing-box")
	mStop := systray.AddMenuItem("Остановить", "brew services stop sing-box")
	mRestart := systray.AddMenuItem("Перезапустить", "brew services restart sing-box")
	systray.AddSeparator()

	configs := sbconfig.Load()
	configItems := make(map[string]*systray.MenuItem, len(configs))

	if len(configs) > 0 {
		mConfigRoot := systray.AddMenuItem("Конфигурация", "Выбор активного конфига sing-box")
		for _, c := range configs {
			configItems[c.Name] = mConfigRoot.AddSubMenuItem(c.Name, c.Path)
		}
	} else {
		note := systray.AddMenuItem("Конфигурация: нет файлов", sbconfig.Dir())
		note.Disable()
	}
	mOpenEditor := systray.AddMenuItem("Открыть папку конфигов в "+configEditorApp, sbconfig.Dir())
	systray.AddSeparator()

	mVersion := systray.AddMenuItem("Версия: "+version, "Версия сборки singbox-tray")
	mVersion.Disable()
	mQuit := systray.AddMenuItem("Выход", "Закрыть трей-иконку")

	// activeConfigName — на какой конфиг сейчас указывает symlink brew-сервиса.
	activeConfigName := func() string {
		prefix, err := br.Prefix()
		if err != nil {
			return ""
		}
		return sbconfig.ActiveName(configs, prefix)
	}

	updateConfigChecks := func() {
		active := activeConfigName()
		for name, item := range configItems {
			if name == active {
				item.Check()
			} else {
				item.Uncheck()
			}
		}
	}

	refresh := func() {
		running, detail := br.Status(sbconfig.ServiceName)
		active := activeConfigName()

		// В строке меню показываем и статус, и активный конфиг:
		//   "SB ● home"  — запущен на конфиге home
		//   "SB ○"       — остановлен, конфиг не из нашего списка
		dot, word := "○", "остановлен"
		if running {
			dot, word = "●", "запущен"
		}
		title := "SB " + dot
		if active != "" {
			title += " " + active
		}
		systray.SetTitle(title)

		tooltip := "sing-box: " + word
		if detail != "" {
			tooltip = "sing-box: " + detail
		}
		if active != "" {
			tooltip += " (" + active + ")"
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

	// runBrewAction выполняет brew-операцию, логирует и уведомляет об ошибке
	// (в режиме .app консоли нет — иначе сбой прошёл бы незаметно), затем
	// обновляет меню в любом случае.
	runBrewAction := func(label string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("brew %s: ошибка: %v\n", label, err)
			_ = notify.Show(fmt.Sprintf("brew %s не удался: %v", label, err))
		}
		refresh()
	}

	switchTo := func(c sbconfig.Entry) {
		prefix, err := br.Prefix()
		if err != nil {
			fmt.Printf("switchConfig(%s): ошибка: %v\n", c.Name, err)
			_ = notify.Show(fmt.Sprintf("не удалось переключить конфиг: %v", err))
			return
		}
		linkPath, err := sbconfig.ActivePath(prefix)
		if err != nil {
			fmt.Printf("switchConfig(%s): ошибка: %v\n", c.Name, err)
			_ = notify.Show(fmt.Sprintf("не удалось переключить конфиг: %v", err))
			return
		}
		if err := sbconfig.Switch(c, linkPath); err != nil {
			fmt.Printf("switchConfig(%s): ошибка: %v\n", c.Name, err)
			_ = notify.Show(fmt.Sprintf("не удалось переключить конфиг: %v", err))
			return
		}
		fmt.Printf("конфигурация переключена на %q\n", c.Name)
		runBrewAction("restart", func() error { return br.Restart(sbconfig.ServiceName) })
	}

	// Отдельная горутина на каждый пункт конфигурации — ClickedCh у
	// каждого MenuItem свой, и его нужно слушать индивидуально.
	for _, c := range configs {
		c := c // захват переменной цикла
		item := configItems[c.Name]
		go func() {
			for range item.ClickedCh {
				switchTo(c)
			}
		}()
	}

	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				runBrewAction("start", func() error { return br.Start(sbconfig.ServiceName) })

			case <-mStop.ClickedCh:
				runBrewAction("stop", func() error { return br.Stop(sbconfig.ServiceName) })

			case <-mRestart.ClickedCh:
				runBrewAction("restart", func() error { return br.Restart(sbconfig.ServiceName) })

			case <-mOpenEditor.ClickedCh:
				if err := notify.OpenInApp(sbconfig.Dir(), configEditorApp); err != nil {
					fmt.Printf("OpenInApp: ошибка: %v\n", err)
				}

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// OnExit вызывается при выходе из трея.
func OnExit() {
	// Здесь можно, например, гарантированно остановить sing-box при выходе:
	// brew.New().Stop(sbconfig.ServiceName)
}
