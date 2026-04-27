package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/git-pkgs/capcheck/internal/analyse"
	"github.com/git-pkgs/capcheck/internal/baseline"
	"github.com/git-pkgs/capcheck/internal/config"
	"github.com/git-pkgs/capcheck/internal/diff"
	"github.com/git-pkgs/capcheck/internal/report"
	"github.com/spf13/cobra"
)

var Version = "dev"

type ExitError struct {
	Code int
}

func (e ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }

type options struct {
	configPath   string
	baselinePath string
	dir          string
	format       string
	granularity  string
	timeout      time.Duration
	strict       bool
	omitPaths    bool
	ignore       []string
}

func (o *options) load() (config.Config, error) {
	cfg, err := config.Load(o.resolve(o.configPath))
	if err != nil {
		return cfg, err
	}
	if o.baselinePath != "" {
		cfg.BaselinePath = o.baselinePath
	}
	if o.granularity != "" {
		cfg.Granularity = o.granularity
	}
	if o.timeout > 0 {
		cfg.Timeout = config.Duration(o.timeout)
	}
	if o.omitPaths {
		cfg.OmitPaths = true
	}
	cfg.Ignore = append(cfg.Ignore, o.ignore...)
	return cfg, nil
}

func (o *options) resolve(p string) string {
	if filepath.IsAbs(p) || o.dir == "" {
		return p
	}
	return filepath.Join(o.dir, p)
}

func (o *options) packages(args []string) []string {
	if len(args) == 0 {
		return []string{"./..."}
	}
	return args
}

func New() *cobra.Command {
	o := &options{}
	root := &cobra.Command{
		Use:           "capcheck [packages]",
		Short:         "Gate Go builds on capability changes using capslock",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), o, args)
		},
	}
	root.Version = Version

	pf := root.PersistentFlags()
	pf.StringVarP(&o.configPath, "config", "c", config.DefaultPath, "path to capcheck.json")
	pf.StringVar(&o.baselinePath, "baseline", "", "path to capcheck.lock.json (overrides config)")
	pf.StringVarP(&o.dir, "dir", "C", "", "run as if in this directory")
	pf.StringVarP(&o.format, "format", "f", "text", "output format: text, json, github")
	pf.StringVar(&o.granularity, "granularity", "", "comparison granularity: package or function (overrides config)")
	pf.DurationVar(&o.timeout, "timeout", 0, "analysis timeout (overrides config)")
	pf.BoolVar(&o.strict, "strict", false, "fail on removed capabilities as well as added")
	pf.BoolVar(&o.omitPaths, "omit-paths", false, "do not record call paths in the lock file (smaller, less helpful diffs)")
	pf.StringSliceVar(&o.ignore, "ignore", nil, "capabilities to ignore (repeatable, stacks with config)")

	root.AddCommand(
		newCheckCmd(o),
		newInitCmd(o),
		newUpdateCmd(o),
		newListCmd(o),
	)
	return root
}

func newCheckCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:   "check [packages]",
		Short: "Compare current capabilities against the lock file",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), o, args)
		},
	}
}

func newInitCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:   "init [packages]",
		Short: "Analyse packages and write capcheck.json and capcheck.lock.json",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), cmd.OutOrStdout(), o, args)
		},
	}
}

func newUpdateCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:   "update [packages]",
		Short: "Re-analyse and overwrite capcheck.lock.json",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), cmd.OutOrStdout(), o, args)
		},
	}
}

func newListCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list [packages]",
		Short: "Print current capabilities without comparing to a baseline",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), cmd.OutOrStdout(), o, args)
		},
	}
}

func runCheck(ctx context.Context, stdout, stderr io.Writer, o *options, args []string) error {
	cfg, err := o.load()
	if err != nil {
		return err
	}
	base, err := baseline.Read(o.resolve(cfg.BaselinePath))
	if baseline.IsNotExist(err) {
		return fmt.Errorf("no baseline at %s: run `capcheck init` first", cfg.BaselinePath)
	}
	if err != nil {
		return err
	}
	curr, err := analyse.Run(ctx, o.packages(args), o.dir, cfg)
	if err != nil {
		return err
	}
	r := diff.Compare(base, curr, cfg)
	if err := writeResult(stdout, o.format, r, o.strict); err != nil {
		return err
	}
	if r.Failed(o.strict) {
		return ExitError{Code: 1}
	}
	if len(r.Removed) > 0 {
		_, _ = fmt.Fprintln(stderr, "note: baseline contains capabilities no longer present; run `capcheck update` to refresh")
	}
	return nil
}

func runInit(ctx context.Context, stdout io.Writer, o *options, args []string) error {
	cfgPath := o.resolve(o.configPath)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("%s already exists", cfgPath)
	}
	cfg := config.Default()
	cfg.Ignore = append(cfg.Ignore, o.ignore...)
	if o.granularity != "" {
		cfg.Granularity = o.granularity
	}
	cil, err := analyse.Run(ctx, o.packages(args), o.dir, cfg)
	if err != nil {
		return err
	}
	if err := config.Write(cfgPath, cfg); err != nil {
		return err
	}
	lockPath := o.resolve(cfg.BaselinePath)
	if err := baseline.Write(lockPath, cil); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "wrote %s\nwrote %s (%d capabilities)\n", cfgPath, lockPath, len(cil.GetCapabilityInfo()))
	return err
}

func runUpdate(ctx context.Context, stdout io.Writer, o *options, args []string) error {
	cfg, err := o.load()
	if err != nil {
		return err
	}
	cil, err := analyse.Run(ctx, o.packages(args), o.dir, cfg)
	if err != nil {
		return err
	}
	lockPath := o.resolve(cfg.BaselinePath)
	if err := baseline.Write(lockPath, cil); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "wrote %s (%d capabilities)\n", lockPath, len(cil.GetCapabilityInfo()))
	return err
}

func runList(ctx context.Context, stdout io.Writer, o *options, args []string) error {
	cfg, err := o.load()
	if err != nil {
		return err
	}
	cil, err := analyse.Run(ctx, o.packages(args), o.dir, cfg)
	if err != nil {
		return err
	}
	r := diff.Compare(nil, cil, cfg)
	entries := make([]diff.Entry, 0, len(r.Added)+len(r.Ignored))
	entries = append(entries, r.Added...)
	entries = append(entries, r.Ignored...)
	diff.SortEntries(entries)
	if o.format == "json" {
		return report.JSON(stdout, diff.Result{Unchanged: entries})
	}
	return report.List(stdout, entries)
}

func writeResult(w io.Writer, format string, r diff.Result, strict bool) error {
	switch format {
	case "json":
		return report.JSON(w, r)
	case "github":
		return report.GitHub(w, r)
	case "text", "":
		return report.Text(w, r, strict)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}
