package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/editor"
	"github.com/moneytosms/verd/internal/embed"
	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/mux"
	"github.com/moneytosms/verd/internal/refresh"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/stats"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/stress"
	"github.com/moneytosms/verd/internal/tui"
	"github.com/moneytosms/verd/internal/watch"
	"github.com/moneytosms/verd/internal/workspace"
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

// Set by goreleaser via -ldflags; "dev" for go install / go build.
var (
	version = "dev"
	commit  = "none"
)

func run(args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintf(out, "verd %s (%s)\n", version, commit)
		return nil
	}
	path := filepath.Join(config.Dir(), "config.toml")
	if i := slices.Index(args, "--here"); i >= 0 { // keep Solutions in the current directory
		args, hereWorkspace = slices.Delete(slices.Clone(args), i, i+1), true
	}
	if len(args) == 0 {
		return runTUI(path)
	}
	switch args[0] {
	case "init":
		if err := config.Init(path, slices.Contains(args[1:], "--force")); err != nil {
			return err
		}
		fmt.Fprintln(out, "wrote", path)
		tmpls, err := workspace.InitTemplates(filepath.Join(config.Dir(), "templates"), config.Default().Lang, slices.Contains(args[1:], "--force"))
		for _, p := range tmpls {
			fmt.Fprintln(out, "wrote", p)
		}
		return err
	case "test":
		if len(args) != 2 {
			return &exitError{2, "usage: verd test <file>"}
		}
		cfg, err := loadConfig(path)
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
		mode := func(contest int, index string) string {
			st, _ := s.ProblemState(contest, index)
			return st.Mode
		}
		// A running TUI shows the run in its pane; otherwise run headless.
		if handled, err := delegateTest(context.Background(), out, socketPath(), args[1]); handled {
			return err
		}
		return testCmd(context.Background(), out, cfg, config.CacheDir(), detail, mode, args[1])
	case "submit":
		if len(args) != 2 {
			return &exitError{2, "usage: verd submit <file>"}
		}
		cfg, err := loadConfig(path)
		if err != nil {
			return err
		}
		if handled, err := delegateSubmit(context.Background(), out, socketPath(), args[1]); handled {
			return err
		}
		s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
		if err != nil {
			return err
		}
		defer s.Close()
		sub := newSubmitter(cfg, cf.New(cf.BaseURL), s)
		p, lang, err := absSolutionProblem(cfg, args[1])
		if err != nil {
			return &exitError{2, err.Error()}
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		st, err := sub.begin(ctx, p, lang)
		if err != nil {
			return &exitError{2, err.Error()}
		}
		if fi, err := os.Stderr.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			fmt.Fprint(os.Stderr, osc52(st.Text)) // clipboard over ssh/tmux where the terminal allows it
		}
		return printSubmit(out, st)
	case "stress":
		fs := flag.NewFlagSet("stress", flag.ContinueOnError)
		iter := fs.Int("iter", 1000, "max iterations")
		secs := fs.Int("time", 30, "max seconds")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 1 {
			return &exitError{2, "usage: verd stress [--iter N] [--time S] <file>"}
		}
		cfg, err := loadConfig(path)
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
		mode := func(contest int, index string) string { st, _ := s.ProblemState(contest, index); return st.Mode }
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		limit := time.Duration(*secs) * time.Second
		if handled, err := delegateStress(ctx, out, socketPath(), fs.Arg(0), *iter, limit); handled {
			return err
		}
		return stressCmd(ctx, out, cfg, config.CacheDir(), detail, mode, fs.Arg(0), *iter, limit)
	case "login":
		return loginCmd(out, os.Stdin, credStore())
	case "logout":
		return logoutCmd(out, credStore())
	case "config":
		cfg, err := loadConfig(path)
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
	return fmt.Errorf("unknown command %q (commands: init [--force], config, test <file>, submit <file>, stress <file>, login, logout)", args[0])
}

// hereWorkspace is set by --here: the Workspace is the current directory, not the configured one.
var hereWorkspace bool

func loadConfig(path string) (config.Config, error) {
	cfg, err := config.Load(path)
	if hereWorkspace {
		cfg.Workspace = "."
	}
	return cfg, err
}

func runTUI(path string) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	if cfg.Handle == "" {
		return fmt.Errorf("no handle set: run `verd init`, then set handle in %s", path)
	}
	// One TUI per user: it owns the socket `verd test` delegates to.
	srv, err := ipc.Listen(socketPath())
	if err != nil {
		return err
	}
	defer srv.Close()

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
	var prog *tea.Program
	ctrl := &editor.Controller{Mux: mux.Detect(cfg.Split, os.Getenv), Bin: "nvim", SockDir: config.RuntimeDir()}
	if cfg.Split == "embedded" {
		// Controller calls Open from the update loop, so messages must not block on prog.Send.
		ctrl.Mux = &embed.Adapter{Send: func(m any) { go prog.Send(m) }}
	}
	if bin, err := exec.LookPath("nvim"); err == nil {
		ctrl.Bin = bin
	}
	sub := newSubmitter(cfg, client, s)
	deps := tui.Deps{
		Submit: func(ctx context.Context, p cf.Problem, lang string) (tui.SubmitStart, error) {
			st, err := sub.begin(ctx, p, lang)
			if err != nil {
				return tui.SubmitStart{}, err
			}
			return tui.SubmitStart{Text: st.Text, Notes: st.Notes, Direct: st.Direct, Updates: st.Updates}, nil
		},
		Reload: func() (tui.Data, error) { return loadData(s, cfg.Handle) },
		Load: func(p cf.Problem, force bool) (*scrape.Detail, error) {
			return refresh.Detail(ctx, s, client, p, force, time.Now())
		},
		OpenURL:   openURL,
		Edit:      func(p cf.Problem, lang string) (*exec.Cmd, error) { return editOpen(cfg, ctrl, p, lang) },
		AddCustom: func(p cf.Problem) (*exec.Cmd, error) { return addCustom(cfg, ctrl, p) },
		Ensure: func(p cf.Problem, lang string) error {
			_, _, err := ensureSolution(cfg, p, lang)
			return err
		},
		Autotest: cfg.Autotest,
		Watch: func(ctx context.Context, p cf.Problem) (<-chan string, error) {
			dir := workspace.New(cfg.Workspace).Dir(p.ContestID, p.Index)
			if err := os.MkdirAll(dir, 0o755); err != nil { // so a Solution created later is still watched
				return nil, err
			}
			return watch.Dir(ctx, dir, 150*time.Millisecond)
		},
		SolutionPath: func(p cf.Problem, lang string) string {
			ref, err := solutionRef(cfg, p, lang)
			if err != nil {
				return ""
			}
			abs, _ := filepath.Abs(ref.Path) // watcher events are absolute
			return abs
		},
		EmbedRatio:  cfg.EmbedRatio,
		FocusKey:    cfg.EmbedFocusKey,
		Langs:       langKeys(cfg),
		DefaultLang: cfg.DefaultLang,
		EditorAlive: ctrl.Alive,
		LoadState: func(p cf.Problem) tui.ProblemState {
			st, _ := s.ProblemState(p.ContestID, p.Index)
			return tui.ProblemState{Lang: st.Lang, Mode: st.Mode}
		},
		SaveState: func(p cf.Problem, st tui.ProblemState) error {
			cur, _ := s.ProblemState(p.ContestID, p.Index)
			cur.Lang, cur.Mode = st.Lang, st.Mode
			return s.SaveProblemState(p.ContestID, p.Index, cur, time.Now())
		},
		Tests: func(ctx context.Context, p cf.Problem, d *scrape.Detail, o tui.RunOpts) (<-chan runner.Event, error) {
			ref, err := solutionRef(cfg, p, o.Lang)
			if err != nil {
				return nil, err
			}
			spec, err := buildSpec(cfg, config.CacheDir(), ref, d, o.Mode)
			if err != nil {
				return nil, fmt.Errorf("%w (press e to create one)", err)
			}
			return runner.Run(ctx, spec), nil
		},
		Stress: func(ctx context.Context, p cf.Problem, d *scrape.Detail, o tui.RunOpts) (<-chan stress.Event, error) {
			ref, err := solutionRef(cfg, p, o.Lang)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(ref.Path); err != nil {
				return nil, fmt.Errorf("no Solution yet (press e to create one)")
			}
			gen, brute, missing := stressHelpers(cfg, ref)
			spec, err := buildStress(cfg, config.CacheDir(), ref, d, o.Mode, 0, 0)
			if err != nil {
				return nil, err
			}
			if missing { // first time: open the fresh helpers so the user can fill them in
				if cmd, err := ctrl.OpenPair(gen, brute); err != nil || cmd != nil {
					return nil, fmt.Errorf("created %s and %s: fill them in, then press S again", gen, brute)
				}
				return nil, errors.New("created gen and brute: fill them in, then press S again")
			}
			return stress.Run(ctx, spec), nil
		},
		Settings: map[string]string{
			"theme": cfg.Theme, "background": cfg.Background, "handle": cfg.Handle, "workspace": cfg.Workspace,
			"default_lang": cfg.DefaultLang, "autotest": fmt.Sprint(cfg.Autotest),
			"time_multiplier": fmt.Sprint(cfg.TimeMultiplier), "float_eps": fmt.Sprint(cfg.FloatEps),
			"split": cfg.Split, "embed_ratio": fmt.Sprint(cfg.EmbedRatio), "embed_focus_key": cfg.EmbedFocusKey,
			"submit_mode": cfg.SubmitMode,
		},
		SaveSetting: func(key, value string) error { return config.Set(path, key, value) },
		EditConfig: func() (*exec.Cmd, error) {
			if _, err := os.Stat(path); err != nil {
				if err := config.Init(path, false); err != nil {
					return nil, err
				}
			}
			return ctrl.Open(path, 0)
		},
		LangSummary: langSummary(cfg),
		Customs: func(p cf.Problem) ([]tui.Case, error) {
			cs, err := workspace.New(cfg.Workspace).Customs(p.ContestID, p.Index)
			out := make([]tui.Case, len(cs))
			for i, c := range cs {
				out[i] = tui.Case{Name: c.Name, Input: c.Input, Want: c.Want, Custom: true}
			}
			return out, err
		},
		SaveCase: func(p cf.Problem, name, input, want string) (string, error) {
			return workspace.New(cfg.Workspace).SaveCase(p.ContestID, p.Index, name, input, want)
		},
		DeleteCase: func(p cf.Problem, name string) error {
			return workspace.New(cfg.Workspace).DeleteCustom(p.ContestID, p.Index, name)
		},
		SaveCounterexample: func(p cf.Problem, input, want string) (string, error) {
			return workspace.New(cfg.Workspace).SaveCustom(p.ContestID, p.Index, input, want)
		},
		// Runs in a Bubble Tea cmd, so the UI renders from cache immediately and stays responsive.
		Refresh: []func() (tui.Data, error){
			stage(s, cfg.Handle, func() error { return refresh.Core(ctx, s, client, cfg.Handle, time.Now()) }),
			stage(s, cfg.Handle, func() error { _, err := refresh.Submissions(ctx, s, client, cfg.Handle); return err }),
		},
	}
	note := workspace.Warning(workspace.New(cfg.Workspace).Root, os.Getenv("WSL_DISTRO_NAME") != "")
	prog = tea.NewProgram(tui.New(data.Problems, note, deps).WithTheme(cfg.Theme).WithBackground(cfg.Background).WithData(data))
	detail := func(ctx context.Context, contest int, index string) (*scrape.Detail, error) {
		return refresh.Detail(ctx, s, client, cf.Problem{ContestID: contest, Index: index}, false, time.Now())
	}
	mode := func(contest int, index string) string {
		st, _ := s.ProblemState(contest, index)
		return st.Mode
	}
	go srv.Serve(dispatch(map[string]ipc.Handler{
		"test":   testHandler(cfg, config.CacheDir(), detail, mode, func(r tui.ExternalRun) { prog.Send(r) }),
		"submit": submitHandler(sub, func(r tui.ExternalSubmit) { prog.Send(r) }),
		"stress": stressHandler(cfg, config.CacheDir(), detail, mode, func(r tui.ExternalRun) { prog.Send(r) }),
	}))
	_, err = prog.Run()
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
	subs, err := s.AllSubmissions()
	if err != nil {
		return d, err
	}
	d.Stats = stats.Compute(stats.Input{Problems: d.Problems, Submissions: subs, Rating: rs, Now: time.Now()})
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

// langSummary describes each configured language for the Settings tab.
func langSummary(cfg config.Config) []string {
	keys := langKeys(cfg)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		l := cfg.Lang[k]
		out = append(out, fmt.Sprintf("%s  .%s  cf id %d", k, l.Ext, l.CFCompilerID))
	}
	return out
}
