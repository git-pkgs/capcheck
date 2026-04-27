package analyse

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/git-pkgs/capcheck/internal/config"
	"github.com/google/capslock/analyzer"
	"github.com/google/capslock/interesting"
	cpb "github.com/google/capslock/proto"
	"golang.org/x/tools/go/packages"
)

type result struct {
	cil *cpb.CapabilityInfoList
	err error
}

func Run(ctx context.Context, patterns []string, dir string, cfg config.Config) (*cpb.CapabilityInfoList, error) {
	if d := time.Duration(cfg.Timeout); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	ch := make(chan result, 1)
	go func() {
		cil, err := analyse(patterns, dir, cfg)
		ch <- result{cil, err}
	}()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("analysis timed out after %s: %w", time.Duration(cfg.Timeout), ctx.Err())
	case r := <-ch:
		return r.cil, r.err
	}
}

func analyse(patterns []string, dir string, cfg config.Config) (*cpb.CapabilityInfoList, error) {
	pcfg := &packages.Config{
		Mode: analyzer.PackagesLoadModeNeeded,
		Dir:  dir,
	}
	if cfg.BuildTags != "" {
		pcfg.BuildFlags = []string{"-tags=" + cfg.BuildTags}
	}
	if cfg.GOOS != "" || cfg.GOARCH != "" {
		env := os.Environ()
		if cfg.GOOS != "" {
			env = append(env, "GOOS="+cfg.GOOS)
		}
		if cfg.GOARCH != "" {
			env = append(env, "GOARCH="+cfg.GOARCH)
		}
		pcfg.Env = env
	}
	pkgs, err := packages.Load(pcfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched %v", patterns)
	}
	var loadErrs []error
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e)
		}
	})
	if len(loadErrs) > 0 {
		return nil, fmt.Errorf("package load errors: %w", errors.Join(loadErrs...))
	}

	classifier, err := buildClassifier(cfg)
	if err != nil {
		return nil, err
	}
	gran, err := analyzer.GranularityFromString(cfg.Granularity)
	if err != nil {
		return nil, err
	}
	queried := analyzer.GetQueriedPackages(pkgs)
	cil := analyzer.GetCapabilityInfo(pkgs, queried, &analyzer.Config{
		Classifier:  classifier,
		Granularity: gran,
		OmitPaths:   cfg.OmitPaths,
	})
	return cil, nil
}

func buildClassifier(cfg config.Config) (*interesting.Classifier, error) {
	if cfg.CapabilityMap == "" {
		return analyzer.GetClassifier(true), nil
	}
	f, err := os.Open(cfg.CapabilityMap)
	if err != nil {
		return nil, fmt.Errorf("opening capability map: %w", err)
	}
	defer func() { _ = f.Close() }()
	c, err := interesting.LoadClassifier(cfg.CapabilityMap, f, false)
	if err != nil {
		return nil, fmt.Errorf("loading capability map: %w", err)
	}
	return c, nil
}
