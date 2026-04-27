package diff

import (
	"fmt"
	"testing"

	"github.com/git-pkgs/capcheck/internal/config"
	cpb "github.com/google/capslock/proto"
)

func synth(nPkgs, nCaps int) *cpb.CapabilityInfoList {
	cil := &cpb.CapabilityInfoList{}
	caps := []string{"FILES", "NETWORK", "EXEC", "RUNTIME", "REFLECT", "CGO", "SYSTEM_CALLS", "UNSAFE_POINTER"}
	for p := range nPkgs {
		pkg := fmt.Sprintf("github.com/example/pkg%d", p)
		for c := 0; c < nCaps && c < len(caps); c++ {
			cil.CapabilityInfo = append(cil.CapabilityInfo, ci(pkg, caps[c],
				pkg+".Func", "github.com/dep.Helper", "os.Thing"))
		}
	}
	return cil
}

func BenchmarkCompare(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("pkgs=%d", n), func(b *testing.B) {
			base := synth(n, 5)
			curr := synth(n, 6)
			cfg := config.Config{Granularity: "package"}
			b.ReportAllocs()
			for b.Loop() {
				_ = Compare(base, curr, cfg)
			}
		})
	}
}
