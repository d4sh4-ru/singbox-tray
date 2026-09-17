package sbconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "work.json"), "{}")
	writeFile(t, filepath.Join(dir, "home.json"), "{}")
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignore me")

	entries := loadEntries(dir)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].Name != "home" || entries[1].Name != "work" {
		t.Fatalf("expected sorted [home, work], got %+v", entries)
	}
}

func TestLoadEntriesMissingDir(t *testing.T) {
	entries := loadEntries(filepath.Join(t.TempDir(), "does-not-exist"))
	if entries != nil {
		t.Fatalf("expected nil for missing dir, got %+v", entries)
	}
}

func TestActivePathEmptyPrefix(t *testing.T) {
	if _, err := ActivePath(""); err == nil {
		t.Fatal("expected error for empty brew prefix")
	}
}

func TestSwitchAndActiveName(t *testing.T) {
	configsDir := t.TempDir()
	writeFile(t, filepath.Join(configsDir, "home.json"), "{}")
	writeFile(t, filepath.Join(configsDir, "work.json"), "{}")
	configs := loadEntries(configsDir)

	brewPrefix := t.TempDir()
	linkPath, err := ActivePath(brewPrefix)
	if err != nil {
		t.Fatal(err)
	}

	if name := ActiveName(configs, brewPrefix); name != "" {
		t.Fatalf("expected no active config before Switch, got %q", name)
	}

	home, work := configs[0], configs[1] // после сортировки: home, work

	if err := Switch(home, linkPath); err != nil {
		t.Fatal(err)
	}
	if name := ActiveName(configs, brewPrefix); name != "home" {
		t.Fatalf("got active=%q, want home", name)
	}

	// Повторное переключение должно заменить symlink, а не наткнуться на
	// "файл уже существует".
	if err := Switch(work, linkPath); err != nil {
		t.Fatal(err)
	}
	if name := ActiveName(configs, brewPrefix); name != "work" {
		t.Fatalf("got active=%q, want work", name)
	}
}
