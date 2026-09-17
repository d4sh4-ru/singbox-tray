// Package sbconfig управляет именованными конфигами sing-box: их список и
// переключение активного через symlink. О brew (перезапуск сервиса) пакет
// ничего не знает — это забота вызывающей стороны.
package sbconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ServiceName — имя сервиса в `brew services` и подпапка конфига:
// $(brew --prefix)/etc/<ServiceName>/config.json — путь, который читает
// установленный через brew sing-box.
const ServiceName = "sing-box"

// Entry — один именованный конфиг из папки Dir().
type Entry struct {
	Name string // без .json, показывается в меню
	Path string // абсолютный путь к файлу
}

// Dir возвращает папку с именованными конфигами.
//
// Намеренно не используем os.UserConfigDir(): на macOS он возвращает
// ~/Library/Application Support, а не ~/.config, что неочевидно и
// расходится с тем, куда обычно кладут конфиги руками.
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "singbox-tray", "configs")
}

// Load сканирует Dir() на *.json и сортирует по имени.
func Load() []Entry {
	return loadEntries(Dir())
}

func loadEntries(dir string) []Entry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		result = append(result, Entry{
			Name: strings.TrimSuffix(e.Name(), ".json"),
			Path: filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ActivePath — путь symlink'а, который читает sing-box, установленный через
// brew с данным prefix (например /opt/homebrew).
func ActivePath(brewPrefix string) (string, error) {
	if brewPrefix == "" {
		return "", fmt.Errorf("brew prefix пуст")
	}
	return filepath.Join(brewPrefix, "etc", ServiceName, "config.json"), nil
}

// ActiveName определяет, на какой из configs сейчас указывает symlink
// ActivePath(brewPrefix). Пустая строка — конфиг не из этого списка
// (например, дефолтный config.json от формулы) или symlink ещё не создан.
func ActiveName(configs []Entry, brewPrefix string) string {
	target, err := ActivePath(brewPrefix)
	if err != nil {
		return ""
	}
	linkDest, err := os.Readlink(target)
	if err != nil {
		return "" // не symlink — ничего не подсвечиваем
	}
	for _, c := range configs {
		if linkDest == c.Path {
			return c.Name
		}
	}
	return ""
}

// Switch переключает symlink activeLinkPath на target.Path. Только
// файловая операция — перезапуск sing-box остаётся на вызывающей стороне
// (пакет brew), чтобы "поменять конфиг" и "перезапустить сервис" можно было
// протестировать и вызвать по отдельности.
func Switch(target Entry, activeLinkPath string) error {
	if err := os.MkdirAll(filepath.Dir(activeLinkPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(activeLinkPath), err)
	}
	// Удаляем текущий файл/симлинк (если есть) и создаём новый симлинк.
	if _, err := os.Lstat(activeLinkPath); err == nil {
		if err := os.Remove(activeLinkPath); err != nil {
			return fmt.Errorf("remove %s: %w", activeLinkPath, err)
		}
	}
	if err := os.Symlink(target.Path, activeLinkPath); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", activeLinkPath, target.Path, err)
	}
	return nil
}
