package diff

import (
	"sort"
	"strings"

	"github.com/git-pkgs/capcheck/internal/config"
	cpb "github.com/google/capslock/proto"
)

type Frame struct {
	Name     string `json:"name"`
	Package  string `json:"package,omitempty"`
	Filename string `json:"filename,omitempty"`
	Line     int64  `json:"line,omitempty"`
	Column   int64  `json:"column,omitempty"`
}

type Entry struct {
	Package    string  `json:"package"`
	Capability string  `json:"capability"`
	Path       []Frame `json:"path,omitempty"`
}

type Result struct {
	Added     []Entry `json:"added"`
	Removed   []Entry `json:"removed"`
	Unchanged []Entry `json:"unchanged"`
	Ignored   []Entry `json:"ignored,omitempty"`
}

func (r Result) Failed(strict bool) bool {
	if len(r.Added) > 0 {
		return true
	}
	if strict && len(r.Removed) > 0 {
		return true
	}
	return false
}

type key struct {
	pkg string
	cap string
	fn  string
}

func Compare(baseline, current *cpb.CapabilityInfoList, cfg config.Config) Result {
	functionGranularity := cfg.Granularity == "function"
	base := index(baseline, functionGranularity)
	curr := index(current, functionGranularity)

	var r Result
	for k, e := range curr {
		if cfg.IsIgnored(e.Capability) {
			r.Ignored = append(r.Ignored, e)
			continue
		}
		if _, ok := base[k]; ok {
			r.Unchanged = append(r.Unchanged, e)
		} else {
			r.Added = append(r.Added, e)
		}
	}
	for k, e := range base {
		if cfg.IsIgnored(e.Capability) {
			continue
		}
		if _, ok := curr[k]; !ok {
			r.Removed = append(r.Removed, e)
		}
	}
	SortEntries(r.Added)
	SortEntries(r.Removed)
	SortEntries(r.Unchanged)
	SortEntries(r.Ignored)
	return r
}

func index(cil *cpb.CapabilityInfoList, functionGranularity bool) map[key]Entry {
	m := make(map[key]Entry)
	if cil == nil {
		return m
	}
	for _, ci := range cil.GetCapabilityInfo() {
		e := toEntry(ci)
		if e.Package == "" || e.Capability == "" {
			continue
		}
		k := key{pkg: e.Package, cap: e.Capability}
		if functionGranularity && len(e.Path) > 0 {
			k.fn = e.Path[0].Name
		}
		if _, exists := m[k]; !exists {
			m[k] = e
		}
	}
	return m
}

func toEntry(ci *cpb.CapabilityInfo) Entry {
	cap := ci.GetCapabilityName()
	if cap == "" {
		cap = strings.TrimPrefix(ci.GetCapability().String(), "CAPABILITY_")
	}
	e := Entry{
		Package:    ci.GetPackageDir(),
		Capability: config.Normalise(cap),
	}
	for _, f := range ci.GetPath() {
		fr := Frame{
			Name:    f.GetName(),
			Package: f.GetPackage(),
		}
		if s := f.GetSite(); s != nil {
			fr.Filename = s.GetFilename()
			fr.Line = s.GetLine()
			fr.Column = s.GetColumn()
		}
		e.Path = append(e.Path, fr)
	}
	return e
}

func SortEntries(es []Entry) {
	sort.Slice(es, func(i, j int) bool {
		if es[i].Package != es[j].Package {
			return es[i].Package < es[j].Package
		}
		if es[i].Capability != es[j].Capability {
			return es[i].Capability < es[j].Capability
		}
		if len(es[i].Path) > 0 && len(es[j].Path) > 0 {
			return es[i].Path[0].Name < es[j].Path[0].Name
		}
		return false
	})
}
