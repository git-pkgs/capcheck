package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/git-pkgs/capcheck/internal/diff"
)

func Text(w io.Writer, r diff.Result, strict bool) error {
	ew := &errWriter{w: w}
	if len(r.Added) == 0 && len(r.Removed) == 0 {
		ew.printf("capcheck: no capability changes (%d tracked)\n", len(r.Unchanged))
		return ew.err
	}
	if len(r.Added) > 0 {
		ew.printf("capcheck: %d new %s since baseline\n\n", len(r.Added), plural(len(r.Added), "capability", "capabilities"))
		for _, e := range r.Added {
			writeEntry(ew, e, "gained")
		}
	}
	if len(r.Removed) > 0 {
		ew.printf("capcheck: %d %s removed since baseline\n\n", len(r.Removed), plural(len(r.Removed), "capability", "capabilities"))
		for _, e := range r.Removed {
			writeEntry(ew, e, "lost")
		}
	}
	if r.Failed(strict) {
		ew.printf("Run `capcheck update` to accept, or remove the offending call path.\n")
	}
	return ew.err
}

func writeEntry(ew *errWriter, e diff.Entry, verb string) {
	ew.printf("  %s %s %s\n", e.Package, verb, e.Capability)
	for _, f := range TrimPath(e.Path, e.Package) {
		loc := ""
		if f.Filename != "" {
			loc = fmt.Sprintf("%s:%d:%d", f.Filename, f.Line, f.Column)
		}
		ew.printf("    %-40s  %s\n", loc, f.Name)
	}
	ew.printf("\n")
}

func JSON(w io.Writer, r diff.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func GitHub(w io.Writer, r diff.Result) error {
	ew := &errWriter{w: w}
	for _, e := range r.Added {
		var loc string
		for _, f := range e.Path {
			if f.Filename != "" {
				loc = fmt.Sprintf("file=%s,line=%d,col=%d", f.Filename, f.Line, f.Column)
				break
			}
		}
		msg := fmt.Sprintf("%s gained capability %s", e.Package, e.Capability)
		if n := len(e.Path); n > 0 {
			msg += " via " + e.Path[n-1].Name
		}
		ew.printf("::error %s::%s\n", loc, msg)
	}
	return ew.err
}

func List(w io.Writer, entries []diff.Entry) error {
	ew := &errWriter{w: w}
	byPkg := map[string][]string{}
	var order []string
	for _, e := range entries {
		if _, ok := byPkg[e.Package]; !ok {
			order = append(order, e.Package)
		}
		byPkg[e.Package] = append(byPkg[e.Package], e.Capability)
	}
	for _, pkg := range order {
		ew.printf("%s\n", pkg)
		for _, c := range byPkg[pkg] {
			ew.printf("  %s\n", c)
		}
	}
	return ew.err
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, a ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, a...)
}

// TrimPath shortens a call path for display: keep the first user-code frame,
// the last user-code frame, the first third-party frame, and the final sink.
func TrimPath(path []diff.Frame, userPkg string) []diff.Frame {
	const max = 4
	if len(path) <= max {
		return path
	}
	inUser := func(f diff.Frame) bool {
		return f.Package != "" && strings.HasPrefix(f.Package, userPkg)
	}
	lastUser := 0
	firstExt := len(path) - 1
	for i, f := range path {
		if inUser(f) {
			lastUser = i
		} else if firstExt == len(path)-1 {
			firstExt = i
		}
	}
	picks := []int{0, lastUser, firstExt, len(path) - 1}
	seen := map[int]bool{}
	var out []diff.Frame
	for _, i := range picks {
		if i >= 0 && i < len(path) && !seen[i] {
			seen[i] = true
			out = append(out, path[i])
		}
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
