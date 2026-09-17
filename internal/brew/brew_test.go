package brew

import (
	"errors"
	"testing"
)

type fakeRunner struct {
	output      []byte
	outputErr   error
	combined    []byte
	combinedErr error
}

func (f *fakeRunner) Output(args ...string) ([]byte, error) {
	return f.output, f.outputErr
}

func (f *fakeRunner) CombinedOutput(args ...string) ([]byte, error) {
	return f.combined, f.combinedErr
}

func TestStatusRunning(t *testing.T) {
	s := &Service{path: "/fake/brew", cmd: &fakeRunner{
		combined: []byte("sing-box started denis /opt/homebrew/var/log\nother   stopped denis\n"),
	}}

	running, detail := s.Status("sing-box")
	if !running || detail != "started" {
		t.Fatalf("got running=%v detail=%q, want running=true detail=started", running, detail)
	}
}

func TestStatusStopped(t *testing.T) {
	s := &Service{path: "/fake/brew", cmd: &fakeRunner{
		combined: []byte("sing-box stopped\n"),
	}}

	running, detail := s.Status("sing-box")
	if running || detail != "stopped" {
		t.Fatalf("got running=%v detail=%q, want running=false detail=stopped", running, detail)
	}
}

func TestStatusNotInList(t *testing.T) {
	s := &Service{path: "/fake/brew", cmd: &fakeRunner{
		combined: []byte("other started\n"),
	}}

	running, detail := s.Status("sing-box")
	if running || detail != "не найден в brew services" {
		t.Fatalf("got running=%v detail=%q", running, detail)
	}
}

func TestStatusBrewNotFound(t *testing.T) {
	s := &Service{}

	running, detail := s.Status("sing-box")
	if running || detail != "brew не найден" {
		t.Fatalf("got running=%v detail=%q, want brew не найден", running, detail)
	}
	if s.Available() {
		t.Fatal("expected Available() = false")
	}
}

func TestPrefixIsCached(t *testing.T) {
	fr := &fakeRunner{output: []byte("/opt/homebrew\n")}
	s := &Service{path: "/fake/brew", cmd: fr}

	p1, err := s.Prefix()
	if err != nil || p1 != "/opt/homebrew" {
		t.Fatalf("got prefix=%q err=%v", p1, err)
	}

	fr.output = []byte("/should/not/be/read/again\n")
	p2, err := s.Prefix()
	if err != nil || p2 != "/opt/homebrew" {
		t.Fatalf("expected cached prefix, got %q (err=%v)", p2, err)
	}
}

func TestPrefixBrewNotFound(t *testing.T) {
	s := &Service{}
	if _, err := s.Prefix(); err == nil {
		t.Fatal("expected error when brew is not found")
	}
}

func TestRestartPropagatesError(t *testing.T) {
	s := &Service{path: "/fake/brew", cmd: &fakeRunner{
		combinedErr: errors.New("boom"),
		combined:    []byte("some brew output"),
	}}

	if err := s.Restart("sing-box"); err == nil {
		t.Fatal("expected error to propagate from CombinedOutput")
	}
}
