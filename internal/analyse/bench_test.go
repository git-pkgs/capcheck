package analyse

import (
	"context"
	"testing"

	"github.com/git-pkgs/capcheck/internal/config"
)

func BenchmarkRunFixture(b *testing.B) {
	cfg := config.Default()
	dir := fixtureDir(b)
	b.ReportAllocs()
	for b.Loop() {
		_, err := Run(context.Background(), []string{"./..."}, dir, cfg)
		if err != nil {
			b.Fatal(err)
		}
	}
}
