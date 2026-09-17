// Package brew оборачивает `brew services` и `brew --prefix` для управления
// сервисом sing-box через Homebrew.
package brew

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GUI-процессы (запущенные через Finder / Login Items, а не из терминала)
// не наследуют PATH из .zshrc/.zprofile, поэтому обычный exec.Command("brew", ...)
// его не находит. Ищем brew явно по типичным путям установки Homebrew.
var candidatePaths = []string{
	"/opt/homebrew/bin/brew", // Apple Silicon
	"/usr/local/bin/brew",    // Intel
}

func resolvePath() string {
	for _, p := range candidatePaths {
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
	return "" // не нашли — Service вернёт понятную ошибку при первом вызове
}

// runner абстрагирует запуск brew-команд, чтобы Service можно было
// тестировать без реального brew в системе.
type runner interface {
	Output(args ...string) ([]byte, error)
	CombinedOutput(args ...string) ([]byte, error)
}

type execRunner struct{ path string }

func (r execRunner) Output(args ...string) ([]byte, error) {
	return exec.Command(r.path, args...).Output()
}

func (r execRunner) CombinedOutput(args ...string) ([]byte, error) {
	return exec.Command(r.path, args...).CombinedOutput()
}

// Service оборачивает конкретный найденный brew-бинарник. Путь и кэш
// префикса — поля структуры, а не пакетные глобальные переменные, поэтому
// Service можно свободно создавать заново (в т.ч. с фейковым runner) в тестах.
type Service struct {
	path        string
	prefixCache string
	cmd         runner
}

// New ищет brew в типичных местах установки. Если не находит, path остаётся
// пустым — методы Service тогда возвращают понятную ошибку вместо паники.
func New() *Service {
	path := resolvePath()
	return &Service{path: path, cmd: execRunner{path: path}}
}

// Available сообщает, нашёлся ли brew-бинарник вообще.
func (s *Service) Available() bool { return s.path != "" }

// Prefix возвращает результат `brew --prefix` (например /opt/homebrew),
// закэшированный в поле — команда дёргает Ruby и не бесплатна.
func (s *Service) Prefix() (string, error) {
	if s.prefixCache != "" {
		return s.prefixCache, nil
	}
	if s.path == "" {
		return "", fmt.Errorf("brew не найден")
	}
	out, err := s.cmd.Output("--prefix")
	if err != nil {
		return "", fmt.Errorf("brew --prefix: %w", err)
	}
	s.prefixCache = strings.TrimSpace(string(out))
	return s.prefixCache, nil
}

func (s *Service) Start(name string) error   { return s.run("start", name) }
func (s *Service) Stop(name string) error    { return s.run("stop", name) }
func (s *Service) Restart(name string) error { return s.run("restart", name) }

func (s *Service) run(action, name string) error {
	if s.path == "" {
		return fmt.Errorf("brew не найден (проверь candidatePaths в internal/brew)")
	}
	out, err := s.cmd.CombinedOutput("services", action, name)
	if err != nil {
		return fmt.Errorf("brew services %s %s: %w\n%s", action, name, err, out)
	}
	return nil
}

// Status парсит `brew services list` и определяет, запущен ли сервис name.
func (s *Service) Status(name string) (running bool, detail string) {
	if s.path == "" {
		return false, "brew не найден"
	}
	out, err := s.cmd.CombinedOutput("services", "list")
	if err != nil {
		return false, "brew недоступен"
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != name {
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
