package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Granularity != "package" {
		t.Errorf("Granularity = %q, want %q", c.Granularity, "package")
	}
	if time.Duration(c.Timeout) != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", c.Timeout, 5*time.Minute)
	}
	if c.BaselinePath != "capcheck.lock.json" {
		t.Errorf("BaselinePath = %q, want %q", c.BaselinePath, "capcheck.lock.json")
	}
	if len(c.Ignore) != 0 {
		t.Errorf("Ignore = %v, want empty", c.Ignore)
	}
	if c.GOOS != "linux" || c.GOARCH != "amd64" {
		t.Errorf("GOOS/GOARCH = %q/%q, want linux/amd64", c.GOOS, c.GOARCH)
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Errorf("Load on missing file = %+v, want defaults", c)
	}
}

func TestLoadFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capcheck.json")
	body := `{
  "granularity": "function",
  "timeout": "30s",
  "baseline": "custom.lock.json",
  "build_tags": "integration,e2e",
  "goos": "linux",
  "goarch": "amd64",
  "capability_map": "maps/extra.yml",
  "omit_paths": true,
  "ignore": ["FILES", "NETWORK", "REFLECT"]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Granularity != "function" {
		t.Errorf("Granularity = %q", c.Granularity)
	}
	if time.Duration(c.Timeout) != 30*time.Second {
		t.Errorf("Timeout = %v", c.Timeout)
	}
	if c.BaselinePath != "custom.lock.json" {
		t.Errorf("BaselinePath = %q", c.BaselinePath)
	}
	if c.BuildTags != "integration,e2e" {
		t.Errorf("BuildTags = %q", c.BuildTags)
	}
	if c.GOOS != "linux" || c.GOARCH != "amd64" {
		t.Errorf("GOOS/GOARCH = %q/%q", c.GOOS, c.GOARCH)
	}
	if c.CapabilityMap != "maps/extra.yml" {
		t.Errorf("CapabilityMap = %q", c.CapabilityMap)
	}
	if !c.OmitPaths {
		t.Error("OmitPaths = false, want true")
	}
	want := []string{"FILES", "NETWORK", "REFLECT"}
	if !reflect.DeepEqual(c.Ignore, want) {
		t.Errorf("Ignore = %v, want %v", c.Ignore, want)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capcheck.json")
	if err := os.WriteFile(path, []byte(`{"ignore": [unclosed`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load on invalid JSON: want error, got nil")
	}
}

func TestLoadInvalidGranularity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capcheck.json")
	if err := os.WriteFile(path, []byte(`{"granularity": "nonsense"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load on invalid granularity: want error, got nil")
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capcheck.json")
	in := Config{
		Granularity:  "package",
		Timeout:      Duration(90 * time.Second),
		BaselinePath: "x.lock.json",
		GOOS:         "darwin",
		GOARCH:       "arm64",
		Ignore:       []string{"FILES"},
	}
	if err := Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
}

func TestIsIgnored(t *testing.T) {
	c := Config{Ignore: []string{"FILES", "network", "CAPABILITY_REFLECT"}}
	cases := []struct {
		cap  string
		want bool
	}{
		{"FILES", true},
		{"files", true},
		{"NETWORK", true},
		{"REFLECT", true},
		{"CAPABILITY_FILES", true},
		{"NETWORK/TCP", true},
		{"FILESYSTEM", false},
		{"EXEC", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := c.IsIgnored(tc.cap); got != tc.want {
			t.Errorf("IsIgnored(%q) = %v, want %v", tc.cap, got, tc.want)
		}
	}
}
