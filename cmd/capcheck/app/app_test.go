package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "fixture")
}

func setupModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := fixtureDir(t)
	for _, f := range []string{"go.mod", "fixture.go"} {
		b, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := New()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestCheckWithoutBaseline(t *testing.T) {
	dir := setupModule(t)
	_, _, err := run(t, "check", "-C", dir, "./...")
	if err == nil {
		t.Fatal("check without baseline: want error, got nil")
	}
	if !strings.Contains(err.Error(), "capcheck init") {
		t.Errorf("error = %q, want hint to run init", err)
	}
}

func TestInitCheckUpdateFlow(t *testing.T) {
	dir := setupModule(t)

	// init
	out, _, err := run(t, "init", "-C", dir, "./...")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "capcheck.json")); err != nil {
		t.Errorf("capcheck.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "capcheck.lock.json")); err != nil {
		t.Errorf("capcheck.lock.json not written: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Errorf("init output = %q", out)
	}

	// check: should be clean
	out, _, err = run(t, "check", "-C", dir, "./...")
	if err != nil {
		t.Fatalf("check after init: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no capability changes") {
		t.Errorf("check output = %q, want clean", out)
	}

	// add a new capability: MODIFY_SYSTEM_STATE via os.Setenv
	extra := `package fixture

import "os"

func SetEnv(k, v string) error { return os.Setenv(k, v) }
`
	if err := os.WriteFile(filepath.Join(dir, "extra.go"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}

	// check: should fail with exit 1
	out, _, err = run(t, "check", "-C", dir, "./...")
	if err == nil {
		t.Fatalf("check after adding capability: want error, got nil\n%s", out)
	}
	var ee ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("check after adding capability: err = %v, want ExitError{1}", err)
	}
	if !strings.Contains(out, "MODIFY_SYSTEM_STATE") {
		t.Errorf("check output missing MODIFY_SYSTEM_STATE:\n%s", out)
	}
	if !strings.Contains(out, "capcheck update") {
		t.Errorf("check output missing update hint:\n%s", out)
	}

	// json format
	out, _, err = run(t, "check", "-C", dir, "--format", "json", "./...")
	if err == nil {
		t.Fatal("json check: want error")
	}
	if !strings.Contains(out, `"capability": "MODIFY_SYSTEM_STATE`) {
		t.Errorf("json output missing capability:\n%s", out)
	}

	// github format
	out, _, _ = run(t, "check", "-C", dir, "--format", "github", "./...")
	if !strings.Contains(out, "::error") {
		t.Errorf("github output missing annotation:\n%s", out)
	}

	// ignore the capability: should pass
	_, _, err = run(t, "check", "-C", dir, "--ignore", "MODIFY_SYSTEM_STATE", "./...")
	if err != nil {
		t.Errorf("check with ignore: %v", err)
	}

	// update: accept the change
	_, _, err = run(t, "update", "-C", dir, "./...")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	// check: clean again
	out, _, err = run(t, "check", "-C", dir, "./...")
	if err != nil {
		t.Fatalf("check after update: %v\n%s", err, out)
	}

	// remove the capability: non-strict passes, strict fails
	if err := os.Remove(filepath.Join(dir, "extra.go")); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := run(t, "check", "-C", dir, "./...")
	if err != nil {
		t.Errorf("check after removal (non-strict): %v\n%s", err, out)
	}
	if !strings.Contains(stderr, "no longer present") {
		t.Errorf("expected stale-baseline note on stderr, got %q", stderr)
	}
	_, _, err = run(t, "check", "-C", dir, "--strict", "./...")
	if err == nil {
		t.Error("check after removal (strict): want error, got nil")
	}
}

func TestInitTwice(t *testing.T) {
	dir := setupModule(t)
	if _, _, err := run(t, "init", "-C", dir, "./..."); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if _, _, err := run(t, "init", "-C", dir, "./..."); err == nil {
		t.Error("second init: want error, got nil")
	}
}

func TestList(t *testing.T) {
	dir := setupModule(t)
	out, _, err := run(t, "list", "-C", dir, "./...")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{"FILES", "NETWORK", "EXEC"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %s:\n%s", want, out)
		}
	}
}

func TestUnknownFormat(t *testing.T) {
	dir := setupModule(t)
	if _, _, err := run(t, "init", "-C", dir, "./..."); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, "check", "-C", dir, "--format", "xml", "./...")
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("unknown format: err = %v", err)
	}
}
