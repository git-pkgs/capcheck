package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultPath         = "capcheck.json"
	DefaultBaselinePath = "capcheck.lock.json"

	defaultTimeout = 5 * time.Minute
	filePerm       = 0o644
)

type Config struct {
	Granularity   string   `json:"granularity,omitempty"`
	Timeout       Duration `json:"timeout,omitempty"`
	BaselinePath  string   `json:"baseline,omitempty"`
	BuildTags     string   `json:"build_tags,omitempty"`
	GOOS          string   `json:"goos,omitempty"`
	GOARCH        string   `json:"goarch,omitempty"`
	CapabilityMap string   `json:"capability_map,omitempty"`
	OmitPaths     bool     `json:"omit_paths,omitempty"`
	Ignore        []string `json:"ignore,omitempty"`
}

// Duration wraps time.Duration so it marshals as a string like "5m" in JSON.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	td, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(td)
	return nil
}

func Default() Config {
	return Config{
		Granularity:  "package",
		Timeout:      Duration(defaultTimeout),
		BaselinePath: DefaultBaselinePath,
		GOOS:         "linux",
		GOARCH:       "amd64",
	}
}

func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	return c, nil
}

func Write(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, filePerm)
}

func (c Config) validate() error {
	switch c.Granularity {
	case "package", "function", "intermediate":
	default:
		return fmt.Errorf("invalid granularity %q (want package, function, or intermediate)", c.Granularity)
	}
	return nil
}

func Normalise(cap string) string {
	return strings.TrimPrefix(strings.ToUpper(cap), "CAPABILITY_")
}

func (c Config) IsIgnored(cap string) bool {
	cap = Normalise(cap)
	if cap == "" {
		return false
	}
	for _, ig := range c.Ignore {
		ig = Normalise(ig)
		if ig == cap || strings.HasPrefix(cap, ig+"/") {
			return true
		}
	}
	return false
}
