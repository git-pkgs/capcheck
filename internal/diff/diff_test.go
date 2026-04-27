package diff

import (
	"testing"

	"github.com/git-pkgs/capcheck/internal/config"
	cpb "github.com/google/capslock/proto"
	"google.golang.org/protobuf/proto"
)

func ci(pkg, cap string, path ...string) *cpb.CapabilityInfo {
	c := &cpb.CapabilityInfo{
		PackageDir:     proto.String(pkg),
		PackageName:    proto.String(pkg),
		CapabilityName: proto.String(cap),
	}
	for _, p := range path {
		c.Path = append(c.Path, &cpb.Function{Name: proto.String(p)})
	}
	return c
}

func cil(infos ...*cpb.CapabilityInfo) *cpb.CapabilityInfoList {
	return &cpb.CapabilityInfoList{CapabilityInfo: infos}
}

func TestCompareNoChange(t *testing.T) {
	base := cil(ci("app", "FILES"), ci("app", "NETWORK"))
	curr := cil(ci("app", "FILES"), ci("app", "NETWORK"))
	r := Compare(base, curr, config.Config{Granularity: "package"})
	if len(r.Added) != 0 || len(r.Removed) != 0 {
		t.Errorf("Added=%d Removed=%d, want 0/0", len(r.Added), len(r.Removed))
	}
	if len(r.Unchanged) != 2 {
		t.Errorf("Unchanged=%d, want 2", len(r.Unchanged))
	}
	if r.Failed(false) || r.Failed(true) {
		t.Error("Failed = true, want false")
	}
}

func TestCompareAdded(t *testing.T) {
	base := cil(ci("app", "FILES"))
	curr := cil(ci("app", "FILES"), ci("app", "EXEC", "app.Run", "os/exec.Command"))
	r := Compare(base, curr, config.Config{Granularity: "package"})
	if len(r.Added) != 1 {
		t.Fatalf("Added=%d, want 1", len(r.Added))
	}
	if r.Added[0].Package != "app" || r.Added[0].Capability != "EXEC" {
		t.Errorf("Added[0] = %+v", r.Added[0])
	}
	if len(r.Added[0].Path) != 2 {
		t.Errorf("Added[0].Path len = %d, want 2", len(r.Added[0].Path))
	}
	if !r.Failed(false) {
		t.Error("Failed(strict=false) = false, want true")
	}
}

func TestCompareRemoved(t *testing.T) {
	base := cil(ci("app", "FILES"), ci("app", "NETWORK"))
	curr := cil(ci("app", "FILES"))
	r := Compare(base, curr, config.Config{Granularity: "package"})
	if len(r.Added) != 0 || len(r.Removed) != 1 {
		t.Fatalf("Added=%d Removed=%d, want 0/1", len(r.Added), len(r.Removed))
	}
	if r.Failed(false) {
		t.Error("Failed(strict=false) = true, want false (removed only)")
	}
	if !r.Failed(true) {
		t.Error("Failed(strict=true) = false, want true")
	}
}

func TestCompareIgnored(t *testing.T) {
	base := cil(ci("app", "FILES"))
	curr := cil(ci("app", "FILES"), ci("app", "REFLECT"), ci("app", "EXEC"))
	cfg := config.Config{Granularity: "package", Ignore: []string{"REFLECT"}}
	r := Compare(base, curr, cfg)
	if len(r.Added) != 1 {
		t.Fatalf("Added=%d, want 1 (REFLECT should be filtered)", len(r.Added))
	}
	if r.Added[0].Capability != "EXEC" {
		t.Errorf("Added[0].Capability = %q, want EXEC", r.Added[0].Capability)
	}
	if len(r.Ignored) != 1 || r.Ignored[0].Capability != "REFLECT" {
		t.Errorf("Ignored = %v, want [REFLECT]", r.Ignored)
	}
}

func TestCompareFunctionGranularity(t *testing.T) {
	base := cil(ci("app", "FILES", "app.A", "os.Open"))
	curr := cil(
		ci("app", "FILES", "app.A", "os.Open"),
		ci("app", "FILES", "app.B", "os.Open"),
	)
	rPkg := Compare(base, curr, config.Config{Granularity: "package"})
	if len(rPkg.Added) != 0 {
		t.Errorf("package granularity: Added=%d, want 0", len(rPkg.Added))
	}
	rFn := Compare(base, curr, config.Config{Granularity: "function"})
	if len(rFn.Added) != 1 {
		t.Errorf("function granularity: Added=%d, want 1", len(rFn.Added))
	}
}

func TestCompareLegacyCapabilityEnum(t *testing.T) {
	enum := cpb.Capability_CAPABILITY_FILES
	base := &cpb.CapabilityInfoList{CapabilityInfo: []*cpb.CapabilityInfo{
		{PackageDir: proto.String("app"), Capability: &enum},
	}}
	curr := cil(ci("app", "FILES"))
	r := Compare(base, curr, config.Config{Granularity: "package"})
	if len(r.Added) != 0 || len(r.Removed) != 0 {
		t.Errorf("legacy enum not normalised: Added=%d Removed=%d", len(r.Added), len(r.Removed))
	}
}

func TestCompareSorted(t *testing.T) {
	base := cil()
	curr := cil(ci("zzz", "NETWORK"), ci("aaa", "EXEC"), ci("aaa", "CGO"))
	r := Compare(base, curr, config.Config{Granularity: "package"})
	if len(r.Added) != 3 {
		t.Fatalf("Added=%d, want 3", len(r.Added))
	}
	keys := []string{
		r.Added[0].Package + "/" + r.Added[0].Capability,
		r.Added[1].Package + "/" + r.Added[1].Capability,
		r.Added[2].Package + "/" + r.Added[2].Capability,
	}
	want := []string{"aaa/CGO", "aaa/EXEC", "zzz/NETWORK"}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("Added[%d] = %q, want %q", i, keys[i], want[i])
		}
	}
}
