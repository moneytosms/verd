package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/refresh"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/tui"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			if ee.msg != "" {
				fmt.Fprintln(os.Stderr, "verd:", ee.msg)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintln(os.Stderr, "verd:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	path := filepath.Join(config.Dir(), "config.toml")
	if len(args) == 0 {
		return runTUI(path)
	}
	switch args[0] {
	case "init":
		if err := config.Init(path, slices.Contains(args[1:], "--force")); err != nil {
			return err
		}
		fmt.Fprintln(out, "wrote", path)
		return nil
	case "test":
		if len(args) != 2 {
			return &exitError{2, "usage: verd test <file>"}
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
		if err != nil {
			return err
		}
		defer s.Close()
		client := cf.New(cf.BaseURL)
		detail := func(ctx context.Context, contest int, index string) (*scrape.Detail, error) {
			return refresh.Detail(ctx, s, client, cf.Problem{ContestID: contest, Index: index}, false, time.Now())
		}
		return testCmd(context.Background(), out, cfg, config.CacheDir(), detail, args[1])
	case "config":
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		b, err := cfg.Effective()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "# %s\n%s", path, b)
		return nil
	}
	return fmt.Errorf("unknown command %q (commands: init [--force], config, test <file>)", args[0])
}

func runTUI(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if cfg.Handle == "" {
		return fmt.Errorf("no handle set: run `verd init`, then set handle in %s", path)
	}
	s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	client := cf.New(cf.BaseURL)
	ctx := context.Background()
	data, err := loadData(s, cfg.Handle)
	if err != nil {
		return err
	}
	deps := tui.Deps{
		Load: func(p cf.Problem, force bool) (*scrape.Detail, error) {
			return refresh.Detail(ctx, s, client, p, force, time.Now())
		},
		OpenURL: openURL,
		// Runs in a Bubble Tea cmd, so the UI renders from cache immediately and stays responsive.
		Refresh: []func() (tui.Data, error){
			stage(s, cfg.Handle, func() error { return refresh.Core(ctx, s, client, cfg.Handle, time.Now()) }),
			stage(s, cfg.Handle, func() error { _, err := refresh.Submissions(ctx, s, client, cfg.Handle); return err }),
		},
	}
	_, err = tea.NewProgram(tui.New(data.Problems, "", deps).WithTheme(cfg.Theme).WithData(data)).Run()
	return err
}

// stage wraps a sync step: run it, then reload whatever is now cached.
func stage(s *store.Store, handle string, sync func() error) func() (tui.Data, error) {
	return func() (tui.Data, error) {
		err := sync()
		d, lerr := loadData(s, handle)
		return d, errors.Join(err, lerr)
	}
}

// loadData reads everything the UI renders from the cache.
func loadData(s *store.Store, handle string) (tui.Data, error) {
	d := tui.Data{Handle: handle}
	var err error
	if d.Problems, err = s.Problems(); err != nil {
		return d, err
	}
	if d.Contests, err = s.Contests(); err != nil {
		return d, err
	}
	if d.Statuses, err = s.Statuses(); err != nil {
		return d, err
	}
	if d.SyncedAt, err = s.LastSync(); err != nil {
		return d, err
	}
	rs, err := s.Rating()
	if err != nil {
		return d, err
	}
	if len(rs) > 0 {
		d.Rating = rs[len(rs)-1].NewRating
	}
	return d, nil
}

func openURL(url string) error {
	switch {
	case os.Getenv("WSL_DISTRO_NAME") != "":
		return exec.Command("wslview", url).Start()
	case runtime.GOOS == "darwin":
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}
