package analyse

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/git-pkgs/capcheck/internal/config"
)

func fixtureDir(tb testing.TB) string {
	tb.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fixture")
}

func TestRunFixture(t *testing.T) {
	cfg := config.Default()
	cil, err := Run(context.Background(), []string{"./..."}, fixtureDir(t), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	caps := map[string]bool{}
	for _, ci := range cil.GetCapabilityInfo() {
		caps[ci.GetCapabilityName()] = true
	}
	for _, want := range []string{"FILES", "NETWORK", "EXEC"} {
		if !caps[want] {
			t.Errorf("expected capability %s, got %v", want, caps)
		}
	}
}

func TestRunTimeout(t *testing.T) {
	cfg := config.Default()
	cfg.Timeout = config.Duration(1 * time.Nanosecond)
	_, err := Run(context.Background(), []string{"./..."}, fixtureDir(t), cfg)
	if err == nil {
		t.Error("Run with 1ns timeout: want error, got nil")
	}
}

func TestRunOmitPaths(t *testing.T) {
	cfg := config.Default()
	cfg.OmitPaths = true
	cil, err := Run(context.Background(), []string{"./..."}, fixtureDir(t), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(cil.GetCapabilityInfo()) == 0 {
		t.Fatal("no capabilities returned")
	}
	for _, ci := range cil.GetCapabilityInfo() {
		if len(ci.GetPath()) > 0 {
			t.Errorf("OmitPaths: capability %s still has %d path frames", ci.GetCapabilityName(), len(ci.GetPath()))
		}
	}
}

func TestRunBadPackage(t *testing.T) {
	cfg := config.Default()
	_, err := Run(context.Background(), []string{"./does/not/exist"}, fixtureDir(t), cfg)
	if err == nil {
		t.Error("Run on missing package: want error, got nil")
	}
}
