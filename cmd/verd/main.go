package main

import (
	"bufio"
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
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
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
	"golang.org/x/term"
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
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprint(out, helpText)
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
		if err != nil {
			return err
		}
		if slices.Contains(args[1:], "--no-setup") || !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(out, "next: verd setup (guided), or set handle in", path)
			return nil
		}
		fmt.Fprint(out, "Run the guided setup now? [Y/n] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a == "n" || a == "no" {
			fmt.Fprintln(out, "next: verd setup (guided), or set handle in", path)
			return nil
		}
		return setupCmd(out, os.Stdin, path, credStore())
	case "update":
		return updateCmd(out, slices.Contains(args[1:], "--check"))
	case "setup":
		return setupCmd(out, os.Stdin, path, credStore())
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
		if len(args) == 2 { // verd logout cses: forget a provider's session
			for _, p := range providers() {
				if strings.EqualFold(args[1], p.source) {
					return logoutCmd(out, p.creds)
				}
			}
			return &exitError{2, "unknown provider " + args[1]}
		}
		return logoutCmd(out, credStore())
	case "sync":
		s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
		if err != nil {
			return err
		}
		defer s.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return syncCmd(ctx, out, os.Stdin, args[1:], s)
	case "daily":
		cfg, err := loadConfig(path)
		if err != nil {
			return err
		}
		s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
		if err != nil {
			return err
		}
		defer s.Close()
		d, err := loadData(s, cfg.Handle)
		if err != nil {
			return err
		}
		return dailyCmd(out, d, time.Now())
	case "keys":
		return keysCmd(out, path, args[1:])
	case "themes":
		return themesCmd(out, args[1:])
	case "config":
		if handled, err := configSub(out, path, args[1:]); handled {
			return err
		}
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
	case "doctor":
		return doctorCmd(out, path, DefaultChecks())
	case "__complete":
		if len(args) != 2 {
			return &exitError{2, "usage: verd __complete config-keys|key-actions|themes"}
		}
		return completeCmd(out, args[1])
	case "completions":
		return completionsCmd(out, args[1:])
	case "schema":
		return schemaCmd(out)
	}
	return fmt.Errorf("unknown command %q (see verd --help)", args[0])
}

const helpText = `verd: terminal Codeforces + CSES

usage: verd [--here] [command]

  (none)                          open the TUI
  init [--force] [--no-setup]     write config and Templates, offer setup
  setup                           guided setup
  config                          print effective config
  config path | keys              config file path | settable keys
  config get <key> | set <key> <value>
                                  read or change one key (comments kept)
  keys [--json]                   list every shortcut, default and current
  keys contexts                   list contexts
  keys set <ctx.action> <key>...  rebind (several keys allowed); keys reset <ctx.action>
  keys presets [list | preset <name> | export | import <file|->]
                                  apply presets, export or import shortcuts
  themes [show <name>]            list themes | print an editable [themes.*] block
  test <file>                     run Sample and Custom Tests
  stress [--iter N] [--time S] <file>
                                  search for a counterexample
  submit <file>                   submit and track the Verdict
  update [--check]                update verd
  daily                           today's unsolved pick, weak topics first
  sync [cses]                     pull solved CSES tasks into marks
  login | logout [cses]           manage direct-submit sessions
  doctor                          diagnostics: config, compilers, editor, terminal
  completions bash|zsh|fish       print shell completion script
  schema                          print JSON Schema for config.toml
  version, --version              print version

flags:
  --here                          use the current directory as Workspace
  -h, --help                      show this help
`

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
	ctrl := &editor.Controller{Bin: "nvim", SockDir: config.RuntimeDir()}
	pickMux := func(mode string) mux.Mux {
		if mode == "embedded" {
			// Controller calls Open from the update loop, so messages must not block on prog.Send.
			return &embed.Adapter{Send: func(m any) { go prog.Send(m) }}
		}
		return mux.Detect(mode, os.Getenv)
	}
	ctrl.Mux = pickMux(cfg.Split)
	ctrl.SetEditor(cfg.Editor)
	sub := newSubmitter(cfg, client, s)
	keys, err := cfg.KeyBindings()
	if err == nil {
		err = tui.CheckKeys(keys)
	}
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	deps := tui.Deps{
		Keys:     keys,
		SaveKeys: func(ctx, action string, ks []string) error { return config.SetKeys(path, ctx, action, ks) },
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
		OpenURL: openURL,
		Edit:    func(p cf.Problem, lang string) (*exec.Cmd, error) { return editOpen(cfg, ctrl, p, lang) },
		Note:    func(p cf.Problem) string { return workspace.New(cfg.Workspace).Note(p.ContestID, p.Index) },
		EditNote: func(p cf.Problem) (*exec.Cmd, error) {
			if err := requireEditor(ctrl); err != nil {
				return nil, err
			}
			path, err := workspace.New(cfg.Workspace).EnsureNote(p.ContestID, p.Index)
			if err != nil {
				return nil, err
			}
			return ctrl.Open(path, 1)
		},
		AddCustom: func(p cf.Problem) (*exec.Cmd, error) { return addCustom(cfg, ctrl, p) },
		Ensure: func(p cf.Problem, lang string) error {
			_, _, err := ensureSolution(cfg, p, lang)
			return err
		},
		Autotest:    cfg.Autotest,
		Mouse:       cfg.Mouse,
		WheelLines:  cfg.WheelLines,
		MouseSelect: cfg.MouseSelect,
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
		EmbedSide:   cfg.EmbedSide,
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
				return nil, fmt.Errorf("%w (press {problem.edit} to create one)", err)
			}
			return runner.Run(ctx, spec), nil
		},
		Stress: func(ctx context.Context, p cf.Problem, d *scrape.Detail, o tui.RunOpts) (<-chan stress.Event, error) {
			ref, err := solutionRef(cfg, p, o.Lang)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(ref.Path); err != nil {
				return nil, fmt.Errorf("no Solution yet (press {problem.edit} to create one)")
			}
			gen, brute, missing := stressHelpers(cfg, ref)
			spec, err := buildStress(cfg, config.CacheDir(), ref, d, o.Mode, 0, 0)
			if err != nil {
				return nil, err
			}
			if missing { // first time: open the fresh helpers so the user can fill them in
				if cmd, err := ctrl.OpenPair(gen, brute); err != nil || cmd != nil {
					return nil, fmt.Errorf("created %s and %s: fill them in, then press {problem.stress} again", gen, brute)
				}
				return nil, errors.New("created gen and brute: fill them in, then press {problem.stress} again")
			}
			return stress.Run(ctx, spec), nil
		},
		Settings: map[string]string{
			"theme": cfg.Theme, "background": cfg.Background, "handle": cfg.Handle, "workspace": cfg.Workspace,
			"default_lang": cfg.DefaultLang, "autotest": fmt.Sprint(cfg.Autotest),
			"time_multiplier": fmt.Sprint(cfg.TimeMultiplier), "float_eps": fmt.Sprint(cfg.FloatEps),
			"editor": cfg.Editor, "source_cf": fmt.Sprint(cfg.SourceCF), "source_cses": fmt.Sprint(cfg.SourceCSES), "split": cfg.Split, "embed_ratio": fmt.Sprint(cfg.EmbedRatio), "embed_focus_key": cfg.EmbedFocusKey,
			"embed_side": cfg.EmbedSide, "submit_mode": cfg.SubmitMode,
			"reading_width": fmt.Sprint(cfg.ReadingWidth), "reading_margin": fmt.Sprint(cfg.ReadingMargin),
			"reading_spacing": cfg.ReadingSpacing, "reading_headings": cfg.ReadingHeadings,
			"reading_math": cfg.ReadingMath, "reading_emphasis": fmt.Sprint(cfg.ReadingEmphasis),
			"mouse": fmt.Sprint(cfg.Mouse), "wheel_lines": fmt.Sprint(cfg.WheelLines), "mouse_select": fmt.Sprint(cfg.MouseSelect),
		},
		Sync: func(source string) (string, error) {
			for _, p := range providers() {
				if p.source == source {
					solved, added, err := syncProvider(context.Background(), p, s, nil)
					if errors.Is(err, errNeedLogin) {
						return "", fmt.Errorf("run `verd sync %s` once to sign in", p.source)
					}
					return fmt.Sprintf("%s: %d solved, %d new marks", p.name, solved, added), err
				}
			}
			return "", fmt.Errorf("no sync for %s", source)
		},
		SetMark: func(p cf.Problem, on bool) error { return s.SetMark(p.ContestID, p.Index, on) },
		SaveCreds: func(cookie, ua string) (string, error) {
			where, _, err := credStore().Save(creds.Creds{Cookie: cookie, UserAgent: ua})
			return where, err
		},
		HasCreds: func() bool { _, err := credStore().Load(); return err == nil },
		SaveSetting: func(key, value string) error {
			if err := config.Set(path, key, value); err != nil {
				return err
			}
			switch key {
			case "split":
				ctrl.SetMux(pickMux(value))
			case "editor":
				ctrl.SetEditor(value)
			}
			return nil
		},
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
			stage(s, cfg.Handle, func() error {
				// read live: Settings can switch CSES on while verd runs
				if c, err := config.Load(path); err == nil && c.SourceCSES {
					_, err := refresh.Source(ctx, s, cf.SourceCSES, time.Now())
					return err
				}
				return nil
			}),
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
	var cfOnly []cf.Problem // stats are about Codeforces: other sources have no ratings or Submissions
	for _, p := range d.Problems {
		if p.Source() == cf.SourceCF {
			cfOnly = append(cfOnly, p)
		}
	}
	d.Stats = stats.Compute(stats.Input{Problems: cfOnly, Submissions: subs, Rating: rs, Now: time.Now()})
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
