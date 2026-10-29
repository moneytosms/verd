package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
	"github.com/moneytosms/verd/internal/editor"
	"github.com/moneytosms/verd/internal/editorial"
	"github.com/moneytosms/verd/internal/embed"
	"github.com/moneytosms/verd/internal/ipc"
	"github.com/moneytosms/verd/internal/mux"
	"github.com/moneytosms/verd/internal/refresh"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/session"
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
		msg, code := err.Error(), 1
		if errors.As(err, &ee) {
			msg, code = ee.msg, ee.code
		}
		if msg != "" {
			fmt.Fprintln(os.Stderr, "verd:", msg)
		}
		if h := usageHint(os.Args[1:], msg); h != "" {
			fmt.Fprintln(os.Stderr, h)
		}
		os.Exit(code)
	}
}

// usageHint points a usage error at the command's help page.
func usageHint(args []string, msg string) string {
	if len(args) == 0 || !strings.Contains(msg, "usage:") {
		return ""
	}
	topic := []string{args[0]}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		topic = append(topic, args[1])
	}
	if d, ok := findDoc(topic); ok {
		return "run `verd help " + d.Name + "` for details"
	}
	return ""
}

// chooseMux picks how the editor opens. Inside tmux or herdr it is a split pane; outside one, "auto"
// draws the editor inside verd (the embedded pane). "suspend" keeps the old suspend-and-resume.
func chooseMux(mode string, env func(string) string, embedded func() mux.Mux) mux.Mux {
	if mode == "embedded" {
		return embedded()
	}
	if m := mux.Detect(mode, env); m != nil {
		return m
	}
	if mode == "auto" || mode == "" {
		return embedded()
	}
	return nil
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
	if topic, ok := wantsHelp(args); ok {
		return showHelp(out, topic)
	}
	path := filepath.Join(config.Dir(), "config.toml")
	args = takeHere(args)
	if len(args) == 0 {
		return runTUI(path)
	}
	if cmd, ok := commands(out, path)[args[0]]; ok {
		return cmd(args)
	}
	return fmt.Errorf("unknown command %q (see verd --help)", args[0])
}

// takeHere strips --here: the Workspace becomes the current directory, so Solutions stay there.
func takeHere(args []string) []string {
	i := slices.Index(args, "--here")
	if i < 0 {
		return args
	}
	hereWorkspace = true
	return slices.Delete(slices.Clone(args), i, i+1)
}

// commands maps each subcommand to its handler. args[0] is the command name.
func commands(out io.Writer, path string) map[string]func(args []string) error {
	return map[string]func(args []string) error{
		"init":        func(args []string) error { return runInit(out, path, args) },
		"update":      func(args []string) error { return updateCmd(out, slices.Contains(args[1:], "--check")) },
		"uninstall":   func(args []string) error { return runUninstall(out, path, args) },
		"setup":       func(args []string) error { return setupCmd(out, os.Stdin, path, credStore()) },
		"test":        func(args []string) error { return runTest(out, path, args) },
		"submit":      func(args []string) error { return runSubmit(out, path, args) },
		"stress":      func(args []string) error { return runStress(out, path, args) },
		"login":       func(args []string) error { return loginCmd(out, os.Stdin, credStore()) },
		"logout":      func(args []string) error { return runLogout(out, args) },
		"sync":        func(args []string) error { return runSync(out, args) },
		"daily":       func(args []string) error { return runDaily(out, path) },
		"keys":        func(args []string) error { return keysCmd(out, path, args[1:]) },
		"themes":      func(args []string) error { return themesCmd(out, args[1:]) },
		"config":      func(args []string) error { return runConfig(out, path, args) },
		"doctor":      func(args []string) error { return doctorCmd(out, path, DefaultChecks()) },
		"__complete":  func(args []string) error { return runComplete(out, args) },
		"editorial":   func(args []string) error { return runEditorial(out, path, args) },
		"snippet":     func(args []string) error { return runSnippet(out, path, args) },
		"completions": func(args []string) error { return completionsCmd(out, args[1:]) },
		"schema":      func(args []string) error { return schemaCmd(out) },
	}
}

// openStore opens the verd database.
func openStore() (*store.Store, error) {
	return store.Open(filepath.Join(config.DataDir(), "verd.db"))
}

// detailFor returns the cache-first Problem detail used by the test, stress and TUI paths.
func detailFor(s *store.Store, client *cf.Client) detailFunc {
	return func(ctx context.Context, contest int, index string) (*scrape.Detail, error) {
		return refresh.Detail(ctx, s, client, cf.Problem{ContestID: contest, Index: index}, false, time.Now())
	}
}

// modeFor returns the saved Comparison Mode of a Problem.
func modeFor(s *store.Store) modeFunc {
	return func(contest int, index string) string {
		st, _ := s.ProblemState(contest, index)
		return st.Mode
	}
}

func runInit(out io.Writer, path string, args []string) error {
	force := slices.Contains(args[1:], "--force")
	if err := config.Init(path, force); err != nil {
		return err
	}
	fmt.Fprintln(out, "wrote", path)
	cfg, err := config.Load(path)
	if err != nil {
		cfg = config.Default()
	}
	if err := writeInitFiles(out, cfg, force); err != nil {
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
}

// writeInitFiles writes the starter templates and snippets.
func writeInitFiles(out io.Writer, cfg config.Config, force bool) error {
	tmpls, err := workspace.InitTemplates(cfg.TemplatesPath(), config.Default().Lang, force)
	for _, p := range tmpls {
		fmt.Fprintln(out, "wrote", p)
	}
	if err != nil {
		return err
	}
	nsnip, err := workspace.InitSnippets(cfg.SnippetsPath(), config.Default().Lang, force)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %d snippets to %s\n", nsnip, cfg.SnippetsPath())
	return nil
}

func runUninstall(out io.Writer, path string, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cfg, _ := loadConfig(path)
	return uninstallCmd(out, os.Stdin, args[1:], uninstallPaths{exe: exe, config: config.Dir(), data: config.DataDir(), cache: config.CacheDir(), workspace: workspace.New(cfg.Workspace).Root})
}

func runTest(out io.Writer, path string, args []string) error {
	file, opts, ok := parseTestArgs(args[1:])
	if !ok {
		return &exitError{2, "usage: verd test <file> [-v] [-t a,b]"}
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	// A running TUI shows the run in its pane; otherwise run headless.
	if !opts.verbose && len(opts.only) == 0 {
		if handled, err := delegateTest(context.Background(), out, socketPath(), file); handled {
			return err
		}
	}
	client := cf.New(cf.BaseURL)
	return testCmd(context.Background(), out, cfg, config.CacheDir(), detailFor(s, client), modeFor(s), file, opts)
}

func runSubmit(out io.Writer, path string, args []string) error {
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
	s, err := openStore()
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
}

func runStress(out io.Writer, path string, args []string) error {
	fset := flag.NewFlagSet("stress", flag.ContinueOnError)
	iter := fset.Int("iter", 1000, "max iterations")
	secs := fset.Int("time", 30, "max seconds")
	jobs := fset.Int("jobs", 0, "parallel iterations (0 = CPU count, max 16)")
	multi := fset.Int("multi", 0, "cases per Solution/brute run; both read t then t cases, gen prints one case without t")
	batch := fset.Int("batch", 0, "cases per gen run; gen gets \"<seed> <N>\" and prints N cases split by --- lines")
	if err := fset.Parse(args[1:]); err != nil || fset.NArg() != 1 {
		return &exitError{2, "usage: verd stress [--iter N] [--time S] [--jobs N] [--batch N] [--multi N] <file>"}
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	client := cf.New(cf.BaseURL)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	limit := time.Duration(*secs) * time.Second
	if handled, err := delegateStress(ctx, out, socketPath(), fset.Arg(0), *iter, limit, *jobs, *batch, *multi); handled {
		return err
	}
	return stressCmd(ctx, out, cfg, config.CacheDir(), detailFor(s, client), modeFor(s), fset.Arg(0), *iter, limit, *jobs, *batch, *multi)
}

func runLogout(out io.Writer, args []string) error {
	if len(args) == 2 { // verd logout cses: forget a provider's session
		for _, p := range providers() {
			if strings.EqualFold(args[1], p.source) {
				return logoutCmd(out, p.creds)
			}
		}
		return &exitError{2, "unknown provider " + args[1]}
	}
	return logoutCmd(out, credStore())
}

func runSync(out io.Writer, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return syncCmd(ctx, out, os.Stdin, args[1:], s)
}

func runDaily(out io.Writer, path string) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := loadData(s, cfg.Handle)
	if err != nil {
		return err
	}
	return dailyCmd(out, d, time.Now())
}

func runConfig(out io.Writer, path string, args []string) error {
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
}

func runComplete(out io.Writer, args []string) error {
	if len(args) != 2 {
		return &exitError{2, "usage: verd __complete config-keys|key-actions|themes"}
	}
	return completeCmd(out, args[1])
}

func runEditorial(out io.Writer, path string, args []string) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	client := cf.New(cf.BaseURL)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return editorialCmd(out, cfg, func(p cf.Problem) (*editorial.Result, error) {
		return editorial.Load(ctx, s, client, p, time.Now())
	}, args[1:])
}

func runSnippet(out io.Writer, path string, args []string) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	return snippetCmd(out, cfg, args[1:])
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

// buildSettings constructs the Settings map for a config, reused by startup and hot reload.
func buildSettings(cfg config.Config) map[string]string {
	return map[string]string{
		"theme": cfg.Theme, "background": cfg.Background, "handle": cfg.Handle, "workspace": cfg.Workspace,
		"default_lang": cfg.DefaultLang, "autotest": fmt.Sprint(cfg.Autotest), "resume": fmt.Sprint(cfg.Resume),
		"time_multiplier": fmt.Sprint(cfg.TimeMultiplier), "float_eps": fmt.Sprint(cfg.FloatEps),
		"editor": cfg.Editor, "source_cf": fmt.Sprint(cfg.SourceCF), "source_cses": fmt.Sprint(cfg.SourceCSES), "split": cfg.Split, "embed_ratio": fmt.Sprint(cfg.EmbedRatio), "embed_focus_key": cfg.EmbedFocusKey,
		"embed_side": cfg.EmbedSide, "embed_zoom": fmt.Sprint(cfg.EmbedZoom), "border": cfg.Border, "submit_mode": cfg.SubmitMode, "browser": cfg.Browser,
		"images": cfg.Images, "reading_width": fmt.Sprint(cfg.ReadingWidth), "reading_margin": fmt.Sprint(cfg.ReadingMargin),
		"reading_spacing": cfg.ReadingSpacing, "reading_headings": cfg.ReadingHeadings,
		"reading_math": cfg.ReadingMath, "reading_emphasis": fmt.Sprint(cfg.ReadingEmphasis),
		"reading_split": cfg.ReadingSplit, "reading_palette": cfg.ReadingPalette, "reading_night_from": fmt.Sprint(cfg.ReadingNightFrom),
		"header": fmt.Sprint(cfg.Header), "footer": fmt.Sprint(cfg.Footer), "tabs": strings.Join(cfg.Tabs, ","),
		"mouse": fmt.Sprint(cfg.Mouse), "wheel_lines": fmt.Sprint(cfg.WheelLines), "mouse_select": fmt.Sprint(cfg.MouseSelect),
	}
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

	s, err := openStore()
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
		// Controller calls Open from the update loop, so messages must not block on prog.Send.
		return chooseMux(mode, os.Getenv, func() mux.Mux { return &embed.Adapter{Send: func(m any) { go prog.Send(m) }} })
	}
	ctrl.Mux = pickMux(cfg.Split)
	ctrl.SetEditor(cfg.Editor)
	sub := newSubmitter(cfg, client, s)
	keys, err := checkKeys(cfg, path)
	if err != nil {
		return err
	}
	deps := tui.Deps{
		Keys:        keys,
		Autotest:    cfg.Autotest,
		Mouse:       cfg.Mouse,
		WheelLines:  cfg.WheelLines,
		MouseSelect: cfg.MouseSelect,
		EmbedRatio:  cfg.EmbedRatio,
		FocusKey:    cfg.EmbedFocusKey,
		EmbedSide:   cfg.EmbedSide,
		EmbedZoom:   cfg.EmbedZoom,
		Langs:       langKeys(cfg),
		DefaultLang: cfg.DefaultLang,
		EditorAlive: ctrl.Alive,
		Settings:    buildSettings(cfg),
		LangSummary: langSummary(cfg),
	}
	deps.NewerVersion = newerRelease
	deps.Update = runUpdate
	deps.WhatsNew = whatsNew
	submitDeps(&deps, sub)
	editDeps(&deps, cfg, path, ctrl, pickMux)
	problemDeps(&deps, cfg, s, client, ctx)
	runDeps(&deps, cfg, ctrl)
	sessionDeps(&deps, cfg, s)
	dataDeps(&deps, cfg, path, s, client, ctx)

	note := workspace.Warning(workspace.New(cfg.Workspace).Root, os.Getenv("WSL_DISTRO_NAME") != "")
	model := tui.New(data.Problems, note, deps).WithTheme(cfg.Theme).WithBackground(cfg.Background).WithData(data)
	statePath := filepath.Join(config.DataDir(), "state.json")
	model = restoreState(model, statePath, cfg.Resume)
	prog = tea.NewProgram(model)
	go srv.Serve(ipcHandler(cfg, s, client, sub, prog))
	// Watch config file for changes (hot reload).
	go watchConfig(context.Background(), filepath.Dir(path), filepath.Base(path), func() {
		prog.Send(tui.ConfigChangedMsg{})
	})
	final, err := prog.Run()
	if fm, ok := final.(tui.Model); ok && err == nil {
		saveState(fm, path, statePath)
	}
	return err
}

// checkKeys loads the key bindings and rejects a config with bad ones.
func checkKeys(cfg config.Config, path string) (map[string][]string, error) {
	keys, err := cfg.KeyBindings()
	if err == nil {
		err = tui.CheckKeys(keys)
	}
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return keys, nil
}

// ipcHandler serves `verd test|submit|stress` from another shell through the running TUI.
func ipcHandler(cfg config.Config, s *store.Store, client *cf.Client, sub *submitter, prog *tea.Program) ipc.Handler {
	return dispatch(map[string]ipc.Handler{
		"test":   testHandler(cfg, config.CacheDir(), detailFor(s, client), modeFor(s), func(r tui.ExternalRun) { prog.Send(r) }),
		"submit": submitHandler(sub, func(r tui.ExternalSubmit) { prog.Send(r) }),
		"stress": stressHandler(cfg, config.CacheDir(), detailFor(s, client), modeFor(s), func(r tui.ExternalRun) { prog.Send(r) }),
	})
}

// submitDeps wires the submit flow and opening a URL in the browser.
func submitDeps(d *tui.Deps, sub *submitter) {
	d.Submit = func(ctx context.Context, p cf.Problem, lang string) (tui.SubmitStart, error) {
		st, err := sub.begin(ctx, p, lang)
		if err != nil {
			return tui.SubmitStart{}, err
		}
		return tui.SubmitStart{Text: st.Text, Notes: st.Notes, Direct: st.Direct, Status: st.Status, Updates: st.Updates}, nil
	}
	d.OpenURL = func(url string) error {
		open := sub.env.OpenCommand(url)
		if open == nil {
			return errors.New("no browser opener found")
		}
		return exec.Command(open[0], open[1:]...).Start()
	}
}

// editDeps wires the editor, notes, custom cases, templates and the config file.
func editDeps(d *tui.Deps, cfg config.Config, path string, ctrl *editor.Controller, pickMux func(string) mux.Mux) {
	d.Edit = func(p cf.Problem, lang string) (*exec.Cmd, error) { return editOpen(cfg, ctrl, p, lang) }
	d.EditAt = func(p cf.Problem, lang string, line int) (*exec.Cmd, error) {
		if err := requireEditor(ctrl); err != nil {
			return nil, err
		}
		ref, err := solutionRef(cfg, p, lang)
		if err != nil {
			return nil, err
		}
		return ctrl.Open(ref.Path, line)
	}
	d.EditNote = func(p cf.Problem) (*exec.Cmd, error) {
		if err := requireEditor(ctrl); err != nil {
			return nil, err
		}
		note, err := workspace.New(cfg.Workspace).EnsureNote(p.ContestID, p.Index)
		if err != nil {
			return nil, err
		}
		return ctrl.Open(note, 1)
	}
	d.AddCustom = func(p cf.Problem) (*exec.Cmd, error) { return addCustom(cfg, ctrl, p) }
	d.EditTemplate = func(lang string) (*exec.Cmd, error) { return editTemplate(cfg, ctrl, lang) }
	d.EditSnippet = func(lang, name string, add bool) (*exec.Cmd, error) {
		return editLibrarySnippet(cfg, ctrl, lang, name, add)
	}
	d.OpenSnippets = func() (*exec.Cmd, error) { return openLibraryFolder(cfg, ctrl) }
	d.EditConfig = func() (*exec.Cmd, error) {
		if _, err := os.Stat(path); err != nil {
			if err := config.Init(path, false); err != nil {
				return nil, err
			}
		}
		return ctrl.Open(path, 0)
	}
	d.SaveSetting = func(key, value string) error {
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
	}
}

// problemDeps wires the Problem, Solution, test, stress and custom-case actions.
func problemDeps(d *tui.Deps, cfg config.Config, s *store.Store, client *cf.Client, ctx context.Context) {
	d.Load = func(p cf.Problem, force bool) (*scrape.Detail, error) {
		return refresh.Detail(ctx, s, client, p, force, time.Now())
	}
	d.Note = func(p cf.Problem) string { return workspace.New(cfg.Workspace).Note(p.ContestID, p.Index) }
	d.Ensure = func(p cf.Problem, lang string) error {
		_, _, err := ensureSolution(cfg, p, lang)
		return err
	}
	d.Watch = func(ctx context.Context, p cf.Problem) (<-chan string, error) {
		dir := workspace.New(cfg.Workspace).Dir(p.ContestID, p.Index)
		if err := os.MkdirAll(dir, 0o755); err != nil { // so a Solution created later is still watched
			return nil, err
		}
		return watch.Dir(ctx, dir, 150*time.Millisecond)
	}
	d.SolutionPath = func(p cf.Problem, lang string) string {
		ref, err := solutionRef(cfg, p, lang)
		if err != nil {
			return ""
		}
		abs, _ := filepath.Abs(ref.Path) // watcher events are absolute
		return abs
	}
	d.Editorial = func(p cf.Problem) (*editorial.Result, error) {
		return editorial.Load(context.Background(), s, client, p, time.Now())
	}
	d.LoadState = func(p cf.Problem) tui.ProblemState {
		st, _ := s.ProblemState(p.ContestID, p.Index)
		return tui.ProblemState{Lang: st.Lang, Mode: st.Mode}
	}
	d.SaveState = func(p cf.Problem, st tui.ProblemState) error {
		cur, _ := s.ProblemState(p.ContestID, p.Index)
		cur.Lang, cur.Mode = st.Lang, st.Mode
		return s.SaveProblemState(p.ContestID, p.Index, cur, time.Now())
	}
	d.SetMark = func(p cf.Problem, on bool) error { return s.SetMark(p.ContestID, p.Index, on) }
	d.Snippets = func(lang string) []tui.Snippet {
		var out []tui.Snippet
		for _, s := range workspace.ListSnippets(cfg.SnippetsPath(), lang, cfg.Lang[lang]) {
			out = append(out, tui.Snippet{Name: s.Name, Desc: s.Desc, Code: s.Code})
		}
		return out
	}
	d.Customs = func(p cf.Problem) ([]tui.Case, error) {
		cs, err := workspace.New(cfg.Workspace).Customs(p.ContestID, p.Index)
		out := make([]tui.Case, len(cs))
		for i, c := range cs {
			out[i] = tui.Case{Name: c.Name, Input: c.Input, Want: c.Want, Custom: true}
		}
		return out, err
	}
	d.SaveCase = func(p cf.Problem, name, input, want string) (string, error) {
		return workspace.New(cfg.Workspace).SaveCase(p.ContestID, p.Index, name, input, want)
	}
	d.DeleteCase = func(p cf.Problem, name string) error {
		return workspace.New(cfg.Workspace).DeleteCustom(p.ContestID, p.Index, name)
	}
	d.SaveCounterexample = func(p cf.Problem, input, want string) (string, error) {
		return workspace.New(cfg.Workspace).SaveCustom(p.ContestID, p.Index, input, want)
	}
}

// runDeps wires the test and stress runs of the open Solution.
func runDeps(d *tui.Deps, cfg config.Config, ctrl *editor.Controller) {
	d.Tests = func(ctx context.Context, p cf.Problem, det *scrape.Detail, o tui.RunOpts) (<-chan runner.Event, error) {
		ref, err := solutionRef(cfg, p, o.Lang)
		if err != nil {
			return nil, err
		}
		spec, err := buildSpec(cfg, config.CacheDir(), ref, det, o.Mode)
		if err != nil {
			return nil, fmt.Errorf("%w (press {problem.edit} to create one)", err)
		}
		spec.Only, spec.Live = o.Only, true
		return runner.Run(ctx, spec), nil
	}
	d.Stress = func(ctx context.Context, p cf.Problem, det *scrape.Detail, o tui.RunOpts) (<-chan stress.Event, error) {
		ref, err := solutionRef(cfg, p, o.Lang)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(ref.Path); err != nil {
			return nil, fmt.Errorf("no Solution yet (press {problem.edit} to create one)")
		}
		gen, brute, missing := stressHelpers(cfg, ref)
		spec, err := buildStress(cfg, config.CacheDir(), ref, det, o.Mode, 0, 0, 0, 0, 0)
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
	}
}

// sessionDeps wires contest sessions and their reports.
func sessionDeps(d *tui.Deps, cfg config.Config, s *store.Store) {
	d.StartSession = func(c cf.Contest, mode string) (session.Session, error) {
		now := time.Now().Unix()
		x := session.Session{ContestID: c.ID, Name: c.Name, Mode: mode, Start: now}
		switch mode {
		case session.Virtual:
			x.End = now + c.Duration
		case session.Live: // the real clock
			x.Start, x.End = c.Start, c.Start+c.Duration
		}
		return x, s.SaveSession(x)
	}
	d.EndSession = s.EndSession
	d.SaveSessionStats = s.SaveSessionStats
	d.SaveReport = func(x session.Session, md string) (string, error) {
		return saveReport(workspace.New(cfg.Workspace).Dir(x.ContestID, ""), x, md)
	}
}

// dataDeps wires the cache, sync, settings and credentials.
func dataDeps(d *tui.Deps, cfg config.Config, path string, s *store.Store, client *cf.Client, ctx context.Context) {
	d.SaveKeys = func(ctx, action string, ks []string) error { return config.SetKeys(path, ctx, action, ks) }
	d.Reload = func() (tui.Data, error) { return loadData(s, cfg.Handle) }
	d.Sync = func(source string) (string, error) {
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
	}
	d.ClearCache = s.ClearCache
	d.SaveCreds = func(cookie, ua string) (string, error) {
		where, _, err := credStore().Save(creds.Creds{Cookie: cookie, UserAgent: ua})
		return where, err
	}
	d.HasCreds = func() bool { _, err := credStore().Load(); return err == nil }
	d.FetchContest = func(id int) error {
		_, err := refresh.ContestProblems(ctx, s, client, id)
		return err
	}
	d.Standings = func(contest int) (*cf.Standing, error) {
		return client.Standings(context.Background(), contest, cfg.Handle)
	}
	// Runs in a Bubble Tea cmd, so the UI renders from cache immediately and stays responsive.
	d.Refresh = []func() (tui.Data, error){
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
	}
	d.LoadConfig = func() (map[string]string, map[string][]string, error) {
		c, err := loadConfig(path)
		if err != nil {
			return nil, nil, err
		}
		keys, err := c.KeyBindings()
		if err != nil {
			return nil, nil, err
		}
		return buildSettings(c), keys, nil
	}
}

// restoreState resumes the last TUI state when resume is on and a saved state exists.
func restoreState(model tui.Model, statePath string, resume bool) tui.Model {
	if b, err := os.ReadFile(statePath); err == nil && resume {
		var st tui.State
		if json.Unmarshal(b, &st) == nil {
			return model.WithState(st)
		}
	}
	return model
}

// saveState keeps the TUI state for the next start. Settings can turn resume off while verd runs,
// so it reads the config live.
func saveState(fm tui.Model, path, statePath string) {
	fm.FlushStats()
	if c, cerr := config.Load(path); cerr == nil && c.Resume {
		if b, jerr := json.Marshal(fm.State()); jerr == nil {
			_ = os.MkdirAll(filepath.Dir(statePath), 0o755)
			_ = os.WriteFile(statePath, b, 0o644)
		}
	}
}

// stage wraps a sync step: run it, then reload whatever is now cached.
func stage(s *store.Store, handle string, sync func() error) func() (tui.Data, error) {
	return func() (tui.Data, error) {
		err := sync()
		d, lerr := loadData(s, handle)
		return d, errors.Join(err, lerr)
	}
}

// saveReport writes a session's report under dir as <mode>-<start>.md, never overwriting: a second
// session of the same mode starting in the same minute gets -2, -3 and so on.
func saveReport(dir string, x session.Session, md string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := fmt.Sprintf("%s-%s", x.Mode, time.Unix(x.Start, 0).Format("2006-01-02-1504"))
	for n := 1; ; n++ {
		name := base + ".md"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.md", base, n)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.WriteString(md)
		return path, errors.Join(werr, f.Close())
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
	d.Subs = subs
	if d.Session, err = s.Session(); err != nil {
		return d, err
	}
	if d.SessionStats, err = s.SessionStats(); err != nil {
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

// watchConfig watches the config file for changes and calls send when it changes.
func watchConfig(ctx context.Context, dir, name string, send func()) {
	ch, err := watch.Dir(ctx, dir, 200*time.Millisecond)
	if err != nil {
		return
	}
	configPath := filepath.Join(dir, name)
	for p := range ch {
		if p == configPath {
			send()
		}
	}
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
