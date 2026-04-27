package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/git-pkgs/capcheck/internal/diff"
)

func resultWithAdded() diff.Result {
	return diff.Result{
		Added: []diff.Entry{
			{
				Package:    "github.com/example/app",
				Capability: "EXEC",
				Path: []diff.Frame{
					{Name: "app.Run", Package: "github.com/example/app", Filename: "/src/app/main.go", Line: 42, Column: 3},
					{Name: "helper.Do", Package: "github.com/example/app/helper", Filename: "/src/app/helper/h.go", Line: 10, Column: 5},
					{Name: "thirdparty.Thing", Package: "github.com/other/lib"},
					{Name: "os/exec.Command", Package: "os/exec"},
				},
			},
		},
		Removed: []diff.Entry{
			{Package: "github.com/example/app", Capability: "NETWORK"},
		},
		Unchanged: []diff.Entry{
			{Package: "github.com/example/app", Capability: "FILES"},
		},
	}
}

func TestTextClean(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, diff.Result{Unchanged: []diff.Entry{{Package: "p", Capability: "FILES"}}}, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "no capability changes") {
		t.Errorf("clean output missing summary: %q", out)
	}
}

func TestTextAdded(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, resultWithAdded(), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"1 new capability",
		"github.com/example/app gained EXEC",
		"main.go:42",
		"app.Run",
		"os/exec.Command",
		"capcheck update",
		"1 capability removed",
		"NETWORK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n---\n%s", want, out)
		}
	}
}

func TestTextStrictRemoved(t *testing.T) {
	r := diff.Result{Removed: []diff.Entry{{Package: "p", Capability: "NETWORK"}}}
	var buf bytes.Buffer
	if err := Text(&buf, r, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Errorf("strict removed output: %q", buf.String())
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, resultWithAdded()); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Added     []diff.Entry `json:"added"`
		Removed   []diff.Entry `json:"removed"`
		Unchanged []diff.Entry `json:"unchanged"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json output not parseable: %v\n%s", err, buf.String())
	}
	if len(decoded.Added) != 1 || decoded.Added[0].Capability != "EXEC" {
		t.Errorf("Added = %+v", decoded.Added)
	}
	if len(decoded.Removed) != 1 || len(decoded.Unchanged) != 1 {
		t.Errorf("Removed/Unchanged lengths wrong: %d/%d", len(decoded.Removed), len(decoded.Unchanged))
	}
}

func TestGitHub(t *testing.T) {
	var buf bytes.Buffer
	if err := GitHub(&buf, resultWithAdded()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "::error file=/src/app/main.go,line=42") {
		t.Errorf("github output missing annotation: %q", out)
	}
	if !strings.Contains(out, "EXEC") {
		t.Errorf("github output missing capability: %q", out)
	}
}

func TestGitHubNoPath(t *testing.T) {
	r := diff.Result{Added: []diff.Entry{{Package: "p", Capability: "EXEC"}}}
	var buf bytes.Buffer
	if err := GitHub(&buf, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "::error ::") {
		t.Errorf("github output without path should still annotate: %q", buf.String())
	}
}

func TestList(t *testing.T) {
	entries := []diff.Entry{
		{Package: "github.com/a", Capability: "FILES"},
		{Package: "github.com/a", Capability: "NETWORK"},
		{Package: "github.com/b", Capability: "EXEC"},
	}
	var buf bytes.Buffer
	if err := List(&buf, entries); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"github.com/a", "FILES", "NETWORK", "github.com/b", "EXEC"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q\n%s", want, out)
		}
	}
}

func TestTrimPath(t *testing.T) {
	full := []diff.Frame{
		{Name: "a", Package: "user/app"},
		{Name: "b", Package: "user/app"},
		{Name: "c", Package: "user/app"},
		{Name: "d", Package: "user/app"},
		{Name: "e", Package: "third/lib"},
		{Name: "f", Package: "third/lib"},
		{Name: "g", Package: "os"},
	}
	got := TrimPath(full, "user/app")
	if len(got) >= len(full) {
		t.Errorf("TrimPath did not shorten: len=%d", len(got))
	}
	if got[0].Name != "a" {
		t.Errorf("first frame = %q, want a", got[0].Name)
	}
	if got[len(got)-1].Name != "g" {
		t.Errorf("last frame = %q, want g", got[len(got)-1].Name)
	}
}

func TestTrimPathShort(t *testing.T) {
	short := []diff.Frame{{Name: "a"}, {Name: "b"}}
	got := TrimPath(short, "x")
	if len(got) != 2 {
		t.Errorf("TrimPath on short path changed length: %d", len(got))
	}
}
