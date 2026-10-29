// Package tui is the root Bubble Tea model.
package tui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/editorial"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/session"
	"github.com/moneytosms/verd/internal/stats"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/stress"
	"github.com/moneytosms/verd/internal/theme"
)

var tabs = []string{"Problems", "Contests", "Stats", "Picker", "Settings"}

// Deps are the side effects the model needs; nil funcs are no-ops.
type Deps struct {
	// Load returns a Problem's detail, cache-first unless force.
	Load    func(p cf.Problem, force bool) (*scrape.Detail, error)
	OpenURL func(url string) error
	Now     func() time.Time // defaults to time.Now
	// NewerVersion returns the latest release tag when it is newer than this build, else "".
	NewerVersion func() string
	// Edit creates the Problem's Solution if needed and opens it in the editor.
	// A nil cmd with a nil error means the editor opened in a split pane (see EditorAlive).
	Edit func(p cf.Problem, lang string) (*exec.Cmd, error)
	// EditAt opens the Solution at a 1-based line (like Edit; the Solution already exists).
	EditAt func(p cf.Problem, lang string, line int) (*exec.Cmd, error)
	// Note returns the Problem's note text ("" if none); EditNote opens it in the editor (like Edit).
	Note     func(p cf.Problem) string
	EditNote func(p cf.Problem) (*exec.Cmd, error)
	// AddCustom creates the next Custom Test files and opens them in the editor (like Edit).
	AddCustom func(p cf.Problem) (*exec.Cmd, error)
	// Ensure creates the Problem's Solution in lang from its Template if absent.
	Ensure func(p cf.Problem, lang string) error
	// Langs are the configured language keys (sorted); DefaultLang is used until the user picks one.
	Langs       []string
	DefaultLang string
	// EditorAlive reports whether the split-pane editor is still open; polled every 500 ms.
	EditorAlive func() bool
	// Tests starts a Test Run of the Problem's Solution and streams its events.
	Tests func(ctx context.Context, p cf.Problem, d *scrape.Detail, o RunOpts) (<-chan runner.Event, error)
	// Stress starts a stress run of the Problem's Solution against its brute force.
	Stress func(ctx context.Context, p cf.Problem, d *scrape.Detail, o RunOpts) (<-chan stress.Event, error)
	// SaveCounterexample stores a stress counterexample as the next Custom Test and returns its name.
	SaveCounterexample func(p cf.Problem, input, want string) (string, error)
	// Customs lists the Problem's Custom Tests with their contents.
	Customs func(p cf.Problem) ([]Case, error)
	// SaveCase writes a Custom Test (a new one when name is empty) and returns its name.
	SaveCase func(p cf.Problem, name, input, want string) (string, error)
	// DeleteCase removes a Custom Test.
	DeleteCase func(p cf.Problem, name string) error
	// Settings are the current config values by key; SaveSetting persists one change (and validates it).
	Settings    map[string]string
	SaveSetting func(key, value string) error
	// SaveCreds stores the browser session direct submit uses and says where it went; HasCreds reports whether one is saved.
	// Sync pulls a provider's solved tasks into the marks using its saved session (no prompts).
	Sync func(source string) (string, error)
	// SetMark records a hand-made solved mark for a Problem of a source with no Submission feed.
	SetMark func(p cf.Problem, on bool) error
	// ClearCache drops the network cache (marks and per-Problem state stay).
	ClearCache func() error
	SaveCreds  func(cookie, ua string) (string, error)
	HasCreds   func() bool
	// EditConfig opens config.toml in the editor; LangSummary describes each configured language.
	EditConfig  func() (*exec.Cmd, error)
	LangSummary []string
	// FetchContest downloads one contest's Problems into the cache (the problemset omits running contests).
	FetchContest func(contest int) error
	// EditTemplate opens the language's Solution Template in the editor, creating it if absent (like Edit).
	EditTemplate func(lang string) (*exec.Cmd, error)
	// StartSession begins a contest session (virtual or practice) and stores it; EndSession forgets it.
	StartSession func(c cf.Contest, mode string) (session.Session, error)
	EndSession   func() error
	// SaveSessionStats records the running session's stats; SaveReport writes the ended session's
	// Markdown report and says where.
	SaveSessionStats func(session.Stats) error
	SaveReport       func(s session.Session, markdown string) (string, error)
	// Standings downloads the live standings of a contest with the user's row (see cf.Client.Standings).
	Standings func(contest int) (*cf.Standing, error)
	// Editorial loads a problem's editorial (see editorial.Load).
	Editorial func(p cf.Problem) (*editorial.Result, error)
	// Snippets lists the snippet library for a language.
	Snippets func(lang string) []Snippet
	// Reload re-reads the cache (no network), e.g. after a Submission lands.
	Reload func() (Data, error)
	// Submit copies the Solution, opens the submit page and tracks the Submission (see SubmitStart).
	Submit func(ctx context.Context, p cf.Problem, lang string) (SubmitStart, error)
	// EmbedRatio is verd's share of the width when an editor is embedded (default 0.4);
	// FocusKey toggles keyboard focus between verd and the editor (default ctrl+\\).
	EmbedRatio float64
	FocusKey   string
	// Keys are the user's shortcut overrides ("context.action" -> keys); SaveKeys persists one (nil keys = reset).
	Keys     map[string][]string
	SaveKeys func(ctx, action string, keys []string) error
	// EmbedSide is the editor's column: right (default) or left.
	EmbedSide string
	// EmbedZoom starts the editor zoomed if true.
	EmbedZoom bool
	// Watch reports saved files in the Problem's directory (debounced) until ctx is done.
	Watch func(ctx context.Context, p cf.Problem) (<-chan string, error)
	// SolutionPath is where the Problem's Solution in lang lives.
	SolutionPath func(p cf.Problem, lang string) string
	// Autotest runs a Test Run whenever the active Solution is saved.
	Autotest bool
	// Mouse controls mouse reporting and wheel behavior.
	Mouse       bool
	FetchImage  func(url string) (image.Image, error) // nil: download into the cache
	WheelLines  int
	MouseSelect bool
	// LoadState and SaveState persist per-Problem choices (Comparison Mode).
	LoadState func(p cf.Problem) ProblemState
	SaveState func(p cf.Problem, s ProblemState) error
	// Refresh stages run in order in the background; each syncs stale data from the network and
	// returns the data now in the cache (valid even when err != nil, e.g. offline). A network
	// failure (cf.ErrNetwork) skips the remaining stages.
	Refresh []func() (Data, error)
	// LoadConfig re-reads the config file and returns current Settings and Keys for hot reload.
	LoadConfig func() (settings map[string]string, keys map[string][]string, err error)
}

// ProblemState is the user's per-Problem choices; empty fields mean "use the default".
type ProblemState struct{ Lang, Mode string }

// RunOpts are the per-run choices handed to Deps.Tests.
type RunOpts struct {
	Lang string
	Mode runner.Mode
	Only []string // Tests: run just the tests with these names; empty = all
}

// Data is everything the UI renders from the cache.
type Data struct {
	Session      *session.Session // the running contest session, nil if none
	SessionStats session.Stats    // what verd recorded during it
	Subs         []cf.Submission  // every stored Submission, for scoring the session
	Handle       string
	Rating       int // latest rating, 0 = unrated
	Problems     []cf.Problem
	Contests     []cf.Contest
	Statuses     map[string]store.Status
	Stats        stats.Stats
	SyncedAt     time.Time // zero = never fully synced
}

type Model struct {
	rng         *rand.Rand
	pickInit    bool
	pickFilter  Filter
	pickExpr    string
	pickPreset  string
	pickNote    string
	picked      *cf.Problem
	pickMatches int
	strs        *stressRun
	stressID    int
	stats       stats.Stats
	statsScroll int
	statsSel    int
	theme       theme.Theme
	dark        bool // terminal background; dark until detected otherwise
	help        bool
	helpPage    int
	helpScroll  int
	handle      string
	rating      int
	syncing     bool
	offline     bool
	syncedAt    time.Time

	Problems []cf.Problem
	Notice   string // transient note shown in the footer; fades after a few seconds
	noticeID int
	imgs     map[string]image.Image // statement images by address (nil = could not be loaded)
	imgErr   map[string]string      // why an image could not be loaded
	imgSent  map[string]bool        // Kitty uploads and placements already written
	imgWant  bool                   // the statement just loaded: fetch its images
	rawOut   string                 // terminal escape sequences to write after this update
	zen      bool                   // session-only: hide the header, footer and Problem info; keep the statement and tests
	deps     Deps
	status   map[string]store.Status

	tab           int
	contests      []cf.Contest
	upcoming      int // contests[:upcoming] are not finished
	contestCursor int
	conQ          string // fuzzy filter over the contests list
	conFind       bool   // keys go into conQ
	langAsk       bool   // the language picker for default_lang "ask" is open
	langSel       int
	conChip       string // "" or "all", "upcoming", "past", "running", "started"
	conSort       string // "" (date order) or "least" (least solved first)
	contestOpen   *cf.Contest
	contestBusy   bool   // a contest's Problems are being fetched
	contestNote   string // result of the last fetch, shown in the contest box
	snipLang      string // language the Snippets setting edits ("" = default language)
	cpCursor      int

	// list filtering
	filter     Filter
	filterExpr string
	visible    []cf.Problem // Problems that pass filter
	input      *input       // active prompt, if any
	inputErr   string
	cursor     int
	width      int
	height     int

	// Problem view
	pstate      ProblemState
	sub         *subState
	toast       *toast
	toastSeq    int
	confirm     bool         // asking whether to submit despite failing local tests
	pending     *ExternalRun // run waiting for its Problem's detail to load
	watchCancel context.CancelFunc
	embed       embedPane
	run         *testRun
	editorOpen  bool // a split-pane editor is open; a tick is polling it
	open        *cf.Problem
	marks       map[string]bool // tests marked for a partial run
	cases       []Case          // Sample Tests then Custom Tests of the open Problem
	tsel        int             // selected row of the Tests pane
	pane        int             // focused pane of the split Problem view
	detScroll   int
	full        bool // full-screen test view open
	fullOff     int
	fullX       int     // horizontal pan of the full-screen view
	fullWrap    bool    // wrap long lines instead of panning
	fullRaw     bool    // show stderr as printed, without folding sanitizer noise
	cm          cmState // contest mode (see contestmode.go)
	inner       int     // set while drawing next to the contest rail: the main area's width
	wAdj, hAdj  int     // user resize of the split view: right column width, Tests pane height
	dragging    bool    // dragging the column divider
	sel         selection
	showTags    bool   // tags revealed in the Problem pane
	bodyW       int    // width the statement was rendered at
	bodyPal     string // reading palette the statement was rendered with
	tm          *testMgr
	fm          *filterMgr
	sp          *snippetPicker
	edv         *editorialView
	raw         Data // last loaded data, before the session hides anything
	sess        *session.Session
	sessResult  session.Result
	sessSum     bool         // the session summary modal is open
	sessConfirm string       // mode awaiting a second keypress
	standing    *cf.Standing // latest live standings of the session contest
	standAt     time.Time
	standBusy   bool
	standErr    string
	sessFetchAt time.Time
	sortBy      string            // "", "rating", "-rating", "solved", "id"
	cfgVals     map[string]string // current config values shown in Settings
	setSel      int
	pal         *palette // command palette
	setQ        string   // fuzzy filter over the settings list
	setFind     bool     // keys go into setQ
	helpQ       string   // fuzzy search over every shortcut
	helpFind    bool
	km          *keymap     // shortcut bindings
	ke          *keyEditor  // the shortcut editor modal, when open
	openTab     int         // the tab the open Problem lives on; other tabs hide it until you come back
	resume      *cf.Problem // Problem to reopen at start (resume)
	setEdit     *settingEdit
	setNote     string
	bgMode      string // background setting: auto, dark or light
	subModal    bool   // the Submission modal is open
	detail      *scrape.Detail
	body        []string // rendered statement lines
	scroll      int
	loading     bool
	errMsg      string
}

func New(ps []cf.Problem, note string, deps Deps) Model {
	vals := map[string]string{}
	for k, v := range deps.Settings {
		vals[k] = v
	}
	km, err := newKeymap(deps.Keys)
	if err != nil {
		km, _ = newKeymap(nil)
		note = err.Error()
	}
	return Model{km: km, cfgVals: vals, Problems: ps, visible: ps, Notice: note, deps: deps, width: 80, height: 24, syncing: len(deps.Refresh) > 0, dark: true, theme: mustTheme("terminal")}
}

func mustTheme(name string) theme.Theme { t, _ := theme.Get(name); return t }

// WithTheme selects a named theme; an unknown name keeps the default and sets a notice.
func (m Model) WithTheme(name string) Model {
	t, ok := theme.Get(name)
	m.theme = t
	if !ok {
		m.Notice = fmt.Sprintf("unknown theme %q (have: %s)", clean(name), strings.Join(theme.Names(), ", "))
	}
	return m
}

// WithBackground applies the background setting: auto keeps detection, dark and light force it.
func (m Model) WithBackground(mode string) Model {
	m.bgMode = mode
	if mode == "dark" || mode == "light" {
		m.dark = mode == "dark"
	}
	return m
}

func (m Model) styles() theme.Styles { return m.theme.Styles(m.dark) }

func (m Model) borderStyle() string {
	if b, ok := m.cfgVals["border"]; ok && b != "" {
		return b
	}
	return "rounded"
}

// WithData replaces everything rendered from the cache, keeping view state.
func (m Model) WithData(d Data) Model {
	m.handle, m.rating, m.syncedAt = d.Handle, d.Rating, d.SyncedAt
	m.Problems, m.status, m.stats = d.Problems, d.Statuses, d.Stats
	m.raw = d
	m = m.applySession(d)
	m.statsSel = min(m.statsSel, max(0, len(d.Stats.Unsolved)-1))
	m = m.WithContests(d.Contests)
	return m.refilter()
}

// ConfigChangedMsg triggers a reload of the config file (hot reload).
type ConfigChangedMsg struct{}

type syncDoneMsg struct {
	text string
	err  error
}

type refreshedMsg struct {
	data  Data
	err   error
	stage int // index of the stage that just finished
}

func (m Model) refresh(stage int) tea.Cmd {
	r := m.deps.Refresh[stage]
	return func() tea.Msg {
		d, err := r()
		return refreshedMsg{d, err, stage}
	}
}

// WithStatuses sets the user's solved/attempted marks (keyed "<contest><index>").
func (m Model) WithStatuses(st map[string]store.Status) Model {
	m.status = st
	return m.refilter()
}

// enabledProblems are the cached Problems of the sources switched on in Settings.
func (m Model) enabledProblems() []cf.Problem {
	out := make([]cf.Problem, 0, len(m.Problems))
	for _, p := range m.Problems {
		if m.sourceOn(p.Source()) {
			out = append(out, p)
		}
	}
	return out
}

// toggleMark flips a hand-made solved mark. Codeforces Problems are marked by their Submissions.
func (m Model) toggleMark(p cf.Problem) Model {
	if p.Source() == cf.SourceCF {
		m.Notice = "Codeforces marks come from your Submissions"
		return m
	}
	on := m.statusOf(p) != store.StatusSolved
	if m.deps.SetMark != nil {
		if err := m.deps.SetMark(p, on); err != nil {
			m.errMsg = "mark: " + err.Error()
			return m
		}
	}
	st := make(map[string]store.Status, len(m.status)+1)
	for k, v := range m.status {
		st[k] = v
	}
	if on {
		st[fmt.Sprintf("%d%s", p.ContestID, p.Index)] = store.StatusSolved
	} else {
		delete(st, fmt.Sprintf("%d%s", p.ContestID, p.Index))
	}
	return m.WithStatuses(st)
}

// sourceOn reports whether Problems from a source are shown (Settings: source_cf, source_cses).
func (m Model) sourceOn(source string) bool {
	v, ok := m.cfgVals["source_"+source]
	if !ok {
		return source == cf.SourceCF
	}
	return v == "true"
}

// sourceOpts are the choices of the filter's Source field: any, then each enabled source.
func (m Model) sourceOpts() []string {
	out := []string{"any"}
	for _, s := range cf.Sources {
		if m.sourceOn(s) {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) statusOf(p cf.Problem) store.Status {
	if st, ok := m.liveStatus(p); ok {
		return st
	}
	return m.status[fmt.Sprintf("%d%s", p.ContestID, p.Index)]
}

func (m Model) refilter() Model {
	m.visible = make([]cf.Problem, 0, len(m.Problems))
	scores := map[string]int{}
	for _, p := range m.Problems {
		if m.sourceOn(p.Source()) && m.filter.Match(p, m.statusOf(p)) {
			m.visible = append(m.visible, p)
			if m.filter.Search != "" {
				scores[fmt.Sprintf("%d%s", p.ContestID, p.Index)], _ = m.filter.Score(p)
			}
		}
	}
	m.sortVisible(scores)
	m.cursor = min(m.cursor, max(0, len(m.visible)-1))
	return m
}

// sortVisible orders the visible Problems by the sort setting, else by search score.
func (m Model) sortVisible(scores map[string]int) {
	switch m.sortBy {
	case "rating", "-rating":
		m.sortByRating()
	case "solved":
		sort.SliceStable(m.visible, func(i, j int) bool { return m.visible[i].SolvedCount > m.visible[j].SolvedCount })
	case "id":
		sort.SliceStable(m.visible, func(i, j int) bool {
			a, b := m.visible[i], m.visible[j]
			if a.ContestID != b.ContestID {
				return a.ContestID < b.ContestID
			}
			return a.Index < b.Index
		})
	}
	if m.filter.Search != "" && m.sortBy == "" { // best match first; stable keeps the list order among equals
		sort.SliceStable(m.visible, func(i, j int) bool {
			a, b := m.visible[i], m.visible[j]
			return scores[fmt.Sprintf("%d%s", a.ContestID, a.Index)] > scores[fmt.Sprintf("%d%s", b.ContestID, b.Index)]
		})
	}
}

// sortByRating sorts ascending for "rating" and descending for "-rating", unrated last.
func (m Model) sortByRating() {
	sort.SliceStable(m.visible, func(i, j int) bool {
		a, b := m.visible[i].Rating, m.visible[j].Rating
		if a == 0 || b == 0 { // unrated last either way
			return b == 0 && a != 0
		}
		if m.sortBy == "rating" {
			return a < b
		}
		return a > b
	})
}

// input is a one-line prompt for the filter (`f`) or search (`/`).
type input struct {
	kind byte // 'f', '/' or 'p'
	text string
	prev string // '/' only: the search to restore on esc
}

// onTab runs when a tab becomes active.
func (m Model) onTab() Model {
	if m.tab == 3 {
		return m.enterPicker()
	}
	return m
}

func (m Model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.input.kind == '/' { // live search: esc undoes it
			m.filter.Search, m.cursor = m.input.prev, 0
			m.input, m.inputErr = nil, ""
			return m.refilter(), nil
		}
		m.input, m.inputErr = nil, ""
	case "tab":
		if m.input.kind == '/' {
			m.filter.SearchMode = nextOf(searchModes, m.searchMode())
			return m.liveSearch(), nil
		}
	case "enter":
		if m.input.kind == 'p' {
			f, err := ParseFilter(m.input.text)
			if err != nil {
				m.inputErr = err.Error()
				return m, nil
			}
			f.Unsolved = true // the Picker only suggests unsolved Problems
			if f.MinRating == 0 && f.MaxRating == 0 {
				d := m.pickDefault()
				f.MinRating, f.MaxRating = d.MinRating, d.MaxRating
			}
			m.pickFilter, m.pickExpr, m.pickPreset, m.pickNote = f, m.input.text, "", ""
			m.input, m.inputErr = nil, ""
			return m.roll(), nil
		}
		if m.input.kind == '/' {
			m.input, m.inputErr = nil, ""
			return m, nil // already applied live
		} else {
			f, err := ParseFilter(m.input.text)
			if err != nil {
				m.inputErr = err.Error()
				return m, nil
			}
			f.Search, f.SearchMode, f.Status = m.filter.Search, m.filter.SearchMode, m.filter.Status
			m.filter, m.filterExpr = f, m.input.text
		}
		m.input, m.inputErr = nil, ""
		m.cursor = 0
		return m.refilter(), nil
	case "backspace":
		if r := []rune(m.input.text); len(r) > 0 {
			m.input.text = string(r[:len(r)-1])
		}
	case "ctrl+u":
		m.input.text = ""
	case "ctrl+c":
		return m, tea.Quit
	default:
		if msg.Text != "" {
			m.input.text += clean(msg.Text)
		}
	}
	if m.input != nil && m.input.kind == '/' {
		return m.liveSearch(), nil
	}
	return m, nil
}

func (m Model) searchMode() string {
	if m.filter.SearchMode == "" {
		return "all"
	}
	return m.filter.SearchMode
}

// liveSearch applies the search prompt's text to the list as the user types.
func (m Model) liveSearch() Model {
	m.filter.Search, m.cursor = m.input.text, 0
	return m.refilter()
}

// contentWidth is the width available to verd's own screen.
// contentWidth is verd's drawing width: the window less an embedded editor and the contest rail.
func (m Model) contentWidth() int {
	if m.inner > 0 {
		return m.inner
	}
	w := m.verdWidth()
	if rw := m.railCols(w); rw > 0 {
		return w - rw - 1
	}
	return w
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.sess != nil {
		cmds = append(cmds, sessTick())
	}
	if len(m.deps.Refresh) > 0 {
		cmds = append(cmds, m.refresh(0))
	}
	if p := m.resume; p != nil {
		cmds = append(cmds, func() tea.Msg { return resumeMsg{*p} })
	}
	if check := m.deps.NewerVersion; check != nil {
		cmds = append(cmds, func() tea.Msg {
			if tag := check(); tag != "" {
				return updateAvailMsg{tag}
			}
			return nil
		})
	}
	return tea.Batch(cmds...)
}

// ExternalRun attaches a Test Run started elsewhere (e.g. `verd test` from Neovim, over the
// socket) to the TUI: the Problem opens and the panel streams the run.
type ExternalRun struct {
	Problem cf.Problem
	Events  <-chan runner.Event
	Stress  <-chan stress.Event // set instead of Events for a stress run
}

// attachExternal shows an external run in whichever panel it belongs to.
func (m Model) attachExternal(r ExternalRun) (Model, tea.Cmd) {
	if r.Stress != nil {
		return m.attachStress(r.Stress)
	}
	return m.attachRun(r.Events)
}

type watchMsg struct {
	path   string
	ch     <-chan string
	closed bool
}

func listenWatch(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		return watchMsg{path: p, ch: ch, closed: !ok}
	}
}

type editorDoneMsg struct{ err error }

type editorTickMsg struct{}

func editorTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return editorTickMsg{} })
}

type detailMsg struct {
	p     cf.Problem
	d     *scrape.Detail
	w     int // width body was rendered at
	body  string
	state ProblemState
	err   error
}

// readingOptions constructs scrape.Options from config values.
func (m Model) readingOptions() scrape.Options {
	o := scrape.DefaultOptions()
	m.readingSize(&o)
	m.readingText(&o)
	if p, ok := m.readingPalette(); ok {
		o.Palette = &p
	}
	return o
}

// readingSize applies the width and margin settings.
func (m Model) readingSize(o *scrape.Options) {
	if v := m.setting("reading_width"); v != "" && v != "0" {
		fmt.Sscanf(v, "%d", &o.Width)
	}
	if _, _, ok := m.split(); ok && m.setting("reading_split") == "cap" && o.Width == 0 {
		o.Width = 76
	}
	if v := m.setting("reading_margin"); v != "" && v != "1" { // 1 is the default
		fmt.Sscanf(v, "%d", &o.Margin)
	}
}

// readingText applies the spacing, heading, math and emphasis settings.
func (m Model) readingText(o *scrape.Options) {
	if v := m.setting("reading_spacing"); v != "" && v != "normal" {
		o.Spacing = v
	}
	if v := m.setting("reading_headings"); v != "" && v != "bar" {
		o.Headings = v
	}
	if v := m.setting("reading_math"); v != "" && v != "unicode" {
		o.Math = v
	}
	if v := m.setting("reading_emphasis"); v == "false" {
		o.Emphasis = false
	}
}

func (m Model) load(p cf.Problem, force bool) tea.Cmd {
	load, width, style, loadState := m.deps.Load, m.stmtWidth()+4, m.styles().Glamour, m.deps.LoadState
	return func() tea.Msg {
		if load == nil {
			return detailMsg{p: p, err: errors.New("loading unavailable")}
		}
		d, err := load(p, force)
		if err != nil {
			return detailMsg{p: p, err: err}
		}
		opts := m.readingOptions()
		body, err := scrape.Render(d.Statement, width-4, style, opts)
		var state ProblemState
		if loadState != nil {
			state = loadState(p)
		}
		return detailMsg{p: p, d: d, w: width - 4, body: body, state: state, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if c, ok := msg.(noticeClearMsg); ok {
		if c.id == m.noticeID {
			m.Notice = ""
		}
		return m, nil
	}
	old := m.Notice
	if m.imgSent == nil {
		m.imgSent = map[string]bool{}
	}
	nm, cmd := m.update(msg)
	if mm, ok := nm.(Model); ok {
		nm = mm.rewrap()
		if n := nm.(Model); n.imgWant || n.rawOut != "" {
			cmds := []tea.Cmd{cmd}
			if n.imgWant {
				cmds = append(cmds, n.fetchImages()...)
			}
			if n.rawOut != "" {
				cmds = append(cmds, tea.Raw(n.rawOut))
			}
			n.imgWant, n.rawOut = false, ""
			nm, cmd = n, tea.Batch(cmds...)
		}
		if n := nm.(Model); n.Notice != "" && n.Notice != old { // a new notice fades after a few seconds
			n.noticeID++
			id := n.noticeID
			nm, cmd = n, tea.Batch(cmd, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return noticeClearMsg{id} }))
		}
	}
	return nm, cmd
}

// noticeClearMsg removes the footer notice numbered id, unless a newer one has replaced it.
type noticeClearMsg struct{ id int }

// rewrap re-renders the statement when the pane it lives in changed width (resize, layout switch).
func (m Model) rewrap() Model {
	if m.detail == nil || m.open == nil || (m.bodyW != 0 && m.stmtWidth() == m.bodyW && m.bodyPal == m.resolvedPalette()) {
		return m
	}
	w := m.stmtWidth()
	opts := m.readingOptions()
	if body, err := scrape.Render(m.detail.Statement, w, m.styles().Glamour, opts); err == nil {
		m = m.setBody(body, w)
		m.scroll = min(m.scroll, m.maxScroll())
	}
	return m
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd, ok := m.updateEmbed(msg)
	if ok {
		return nm, cmd
	}
	m = nm // a click on verd's side moves focus even though the click is also handled below
	msg = m.shiftMouse(msg)
	if nm, rest, cmd, ok := m.railMouse(msg); ok {
		return nm, cmd
	} else {
		msg = rest
	}
	if nm, cmd, ok := m.updateGroups(msg); ok {
		return nm, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeEmbed()
	case tea.BackgroundColorMsg:
		if m.bgMode == "" || m.bgMode == "auto" {
			m.dark = msg.IsDark()
		}
	case tea.PasteMsg:
		m = m.onPaste(msg)
	case tea.KeyPressMsg:
		return m.updateKeyPress(msg)
	}
	return m, nil
}

// updateEmbed gives the embedded editor pane and contest mode first claim on a message.
func (m Model) updateEmbed(msg tea.Msg) (Model, tea.Cmd, bool) {
	if nm, cmd, ok := m.onEmbedMsg(msg); ok {
		return nm, cmd, true
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && m.cmOn() && m.ke == nil && m.modalBox() == nil {
		if nm, cmd, ok := m.cmKey(k); ok {
			return nm, cmd, true
		}
	}
	return m.routeEmbedInput(msg)
}

// updateGroups routes run results, editor polls, data, sync and pointer messages.
func (m Model) updateGroups(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if nm, cmd, ok := m.updateRun(msg); ok {
		return nm, cmd, true
	}
	if nm, cmd, ok := m.updateEditor(msg); ok {
		return nm, cmd, true
	}
	if nm, cmd, ok := m.updateData(msg); ok {
		return nm, cmd, true
	}
	if nm, cmd, ok := m.updateSync(msg); ok {
		return nm, cmd, true
	}
	return m.updatePointer(msg)
}

func (m Model) updateRun(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case submitStartedMsg, ExternalSubmit, subUpdateMsg, toastClearMsg, reloadMsg:
		nm, cmd, _ := m.onSubmitMsg(msg)
		return nm, cmd, true
	case testEventMsg:
		nm, cmd := m.onTestEvent(msg)
		return nm, cmd, true
	case updateAvailMsg:
		if m.toast == nil { // never replace a Submission toast
			m.toast = &toast{text: fmt.Sprintf("verd %s available: run `verd update`  x dismiss", msg.tag)}
		}
		return m, nil, true
	case resumeMsg:
		m.resume = nil
		nm, cmd := m.reopen(msg.p)
		return nm, cmd, true
	case ExternalRun:
		nm, cmd := m.onExternal(msg)
		return nm, cmd, true
	case stressEventMsg:
		nm, cmd := m.onStressEvent(msg)
		return nm, cmd, true
	case watchMsg:
		nm, cmd := m.onWatch(msg)
		return nm, cmd, true
	case sessTickMsg:
		nm, cmd := m.onSessTick()
		return nm, cmd, true
	}
	return m, nil, false
}

// updateEditor polls the split-pane editor and reloads the tests when it closes.
func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case editorTickMsg:
		if m.deps.EditorAlive != nil && m.deps.EditorAlive() {
			return m, editorTick(), true
		}
		m.editorOpen = false // the pane closed
		return m.loadCases(), nil, true
	case editorDoneMsg:
		if msg.err != nil {
			m.errMsg = "editor: " + msg.err.Error()
		}
		return m.loadCases(), nil, true
	}
	return m, nil, false
}

func (m Model) updateData(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case refreshedMsg:
		nm, cmd := m.onRefreshed(msg)
		return nm, cmd, true
	case editorialMsg:
		return m.onEditorial(msg), nil, true
	case standingMsg:
		return m.onStanding(msg), nil, true
	case detailMsg:
		nm, cmd := m.onDetail(msg)
		return nm, cmd, true
	case imagesMsg:
		return m.onImages(msg), nil, true
	case ConfigChangedMsg:
		nm, cmd := m.onConfigChanged()
		return nm, cmd, true
	}
	return m, nil, false
}

// updateSync applies a manual sync's result and a contest fetch's result.
func (m Model) updateSync(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case syncDoneMsg:
		m.Notice = msg.text
		if msg.err != nil {
			m.Notice = clean(msg.err.Error())
			return m, nil, true
		}
		return m, m.reload(), true
	case contestFetchedMsg:
		m.contestBusy = false
		if msg.err != nil {
			m.contestNote = clean(msg.err.Error())
			return m, nil, true
		}
		m.contestNote = ""
		return m, m.reload(), true
	}
	return m, nil, false
}

// onRefreshed takes one sync stage's data and starts the next stage while online.
func (m Model) onRefreshed(msg refreshedMsg) (tea.Model, tea.Cmd) {
	m = m.WithData(msg.data)
	m.offline = errors.Is(msg.err, cf.ErrNetwork)
	if msg.stage == 0 || msg.err != nil { // later stages must not erase an earlier stage's notice
		m.Notice = ""
		if msg.err != nil && !m.offline {
			m.Notice = "sync failed: " + msg.err.Error()
		}
	}
	if next := msg.stage + 1; !m.offline && next < len(m.deps.Refresh) {
		return m, m.refresh(next)
	}
	m.syncing = false
	return m, nil
}

// onExternal attaches a Test Run started outside verd, opening its Problem if needed.
func (m Model) onExternal(msg ExternalRun) (tea.Model, tea.Cmd) {
	for _, p := range m.Problems { // the server only knows the id; borrow the name
		if p.ContestID == msg.Problem.ContestID && p.Index == msg.Problem.Index {
			msg.Problem = p
			break
		}
	}
	if m.open == nil || m.open.ContestID != msg.Problem.ContestID || m.open.Index != msg.Problem.Index {
		nm, cmd := m.openProblem(msg.Problem)
		m = nm.(Model)
		m.pending = &msg
		return m, cmd
	}
	if m.detail == nil {
		m.pending = &msg
		return m, nil
	}
	return m.attachExternal(msg)
}

// onWatch keeps listening for saved files and runs the tests on the Solution when autotest is on.
func (m Model) onWatch(msg watchMsg) (tea.Model, tea.Cmd) {
	if msg.closed || m.open == nil {
		return m, nil
	}
	cmd := listenWatch(msg.ch)
	if m.deps.Autotest && m.detail != nil && m.deps.SolutionPath != nil && msg.path == m.deps.SolutionPath(*m.open, m.lang()) {
		nm, run := m.startTests()
		return nm, tea.Batch(cmd, run)
	}
	return m, cmd
}

func (m Model) onDetail(msg detailMsg) (tea.Model, tea.Cmd) {
	if m.open == nil || m.open.ContestID != msg.p.ContestID || m.open.Index != msg.p.Index {
		return m, nil // user already left or switched Problems
	}
	m.loading = false
	if msg.err != nil {
		return m.onDetailErr(msg.err), nil
	}
	m.errMsg, m.detail, m.scroll, m.pstate = "", msg.d, 0, msg.state
	if m.defaultLang() == "ask" && m.pstate.Lang == "" && len(m.deps.Langs) > 0 {
		m.langAsk, m.langSel = true, 0
	}
	m = m.setBody(msg.body, msg.w).cmRestoreScroll()
	m.imgWant = true
	m = m.loadCases()
	if m.pending != nil {
		r := *m.pending
		m.pending = nil
		return m.attachExternal(r)
	}
	return m, nil
}

// onDetailErr shows why a statement did not load.
func (m Model) onDetailErr(err error) Model {
	switch {
	case errors.Is(err, cf.ErrNetwork):
		m.offline = true
		if m.detail == nil {
			m.errMsg = "offline: statement not cached. Press {problem.open_browser} to open in browser."
		} else {
			m.errMsg = "offline: could not refetch"
		}
	case errors.Is(err, cf.ErrChallenge):
		m.errMsg = "Codeforces blocked the request (Cloudflare). Press {problem.open_browser} to open in browser."
	default:
		m.errMsg = "load failed: " + err.Error()
	}
	return m
}

// updatePointer handles mouse clicks, drags, releases and the wheel.
func (m Model) updatePointer(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			nm, cmd := m.onClick(msg.X, msg.Y)
			return nm, cmd, true
		}
	case tea.MouseMotionMsg:
		return m.onDrag(msg), nil, true
	case tea.MouseReleaseMsg:
		m.dragging = false
		nm, cmd := m.endSelect()
		return nm, cmd, true
	case tea.MouseWheelMsg:
		nm, cmd := m.onWheel(msg.X, msg.Y, msg.Button == tea.MouseWheelUp)
		return nm, cmd, true
	}
	return m, nil, false
}

// onDrag extends a selection under a left drag and moves the column divider.
func (m Model) onDrag(msg tea.MouseMotionMsg) Model {
	if msg.Button == tea.MouseLeft {
		m = m.dragSelect(msg.X, msg.Y)
	}
	if m.dragging && msg.Button == tea.MouseLeft {
		if g, ok := m.splitGeom(); ok {
			m.wAdj += g.lw - msg.X // divider follows the pointer
		}
	}
	return m
}

// onPaste puts pasted text into the focused field.
func (m Model) onPaste(msg tea.PasteMsg) Model {
	if m.tab == 4 && m.setEdit != nil {
		m.setEdit.text += clean(msg.Content)
	} else if m.tm != nil && m.tm.ed != nil {
		m.tm.ed.insert(clean(msg.Content))
	} else if m.input != nil {
		m.input.text += clean(msg.Content)
		if m.input.kind == '/' {
			m = m.liveSearch()
		}
	}
	return m
}

func (m Model) updateKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.ke != nil {
		return m.updateKeyEditor(msg)
	}
	var keep bool
	if msg, keep = m.remapKey(msg); !keep {
		return m, nil
	}
	if nm, cmd, ok := m.updatePanel(msg); ok {
		return nm, cmd
	}
	if nm, cmd, ok := m.updateDialog(msg); ok {
		return nm, cmd
	}
	if nm, cmd, ok := m.globalKey(msg); ok {
		return nm, cmd
	}
	if m.viewing() {
		return m.updateProblem(msg)
	}
	if m.input != nil {
		return m.updateInput(msg)
	}
	if nm, cmd, ok := m.tabKey(msg.String()); ok {
		return nm, cmd
	}
	return m.updateTab(msg)
}

// updatePanel sends a key to the first open panel that takes input.
func (m Model) updatePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if m.edv != nil {
		nm, cmd := m.updateEditorial(msg)
		return nm, cmd, true
	}
	if m.sessSum {
		nm, cmd := m.updateSessSummary(msg)
		return nm, cmd, true
	}
	if m.pal != nil {
		nm, cmd := m.updatePalette(msg)
		return nm, cmd, true
	}
	if m.sp != nil {
		nm, cmd := m.updateSnippets(msg)
		return nm, cmd, true
	}
	if m.tm != nil {
		nm, cmd := m.updateTM(msg)
		return nm, cmd, true
	}
	if m.fm != nil {
		nm, cmd := m.updateFilters(msg)
		return nm, cmd, true
	}
	return m, nil, false
}

// updateDialog sends a key to the Submission modal, help or the language picker.
func (m Model) updateDialog(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if m.subModal {
		switch msg.String() {
		case "q", "esc", "enter":
			m.subModal = false
		case "ctrl+c":
			return m, tea.Quit, true
		}
		return m, nil, true
	}
	if m.help {
		nm, cmd := m.updateHelp(msg)
		return nm, cmd, true
	}
	if m.langAsk {
		nm, cmd := m.updateLangAsk(msg)
		return nm, cmd, true
	}
	return m, nil, false
}

// globalKey handles the keys that work on every screen: toast, palette, zen and help.
func (m Model) globalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case msg.String() == "x" && m.toast != nil && m.input == nil && !m.typing():
		m.toast = nil
		return m, nil, true
	case msg.String() == "ctrl+k" && m.input == nil && !m.typing():
		return m.openPalette(), nil, true
	case msg.String() == "Z" && m.input == nil && !m.typing() && m.ke == nil:
		return m.toggleZen(), nil, true
	case msg.String() == "?" && m.input == nil && !m.typing():
		m.help, m.helpPage, m.helpScroll = true, 0, 0
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) tabKey(k string) (tea.Model, tea.Cmd, bool) {
	switch k {
	case "ctrl+r":
		return m.refreshKey()
	case "1", "2", "3", "4", "5":
		return m.tabNumber(k)
	case "tab":
		return m.cycleTab()
	}
	return m, nil, false
}

// refreshKey is ctrl+r: fetch the contest on its page, else sync the data.
func (m Model) refreshKey() (tea.Model, tea.Cmd, bool) {
	if m.tab == 1 && m.contestOpen != nil {
		nm, cmd := m.fetchContest()
		return nm, cmd, true
	}
	if len(m.deps.Refresh) == 0 || m.syncing {
		return m, nil, true
	}
	m.syncing = true
	return m, m.refresh(0), true
}

// tabNumber jumps to the tab at a number key; a number the tab row lacks is not handled.
func (m Model) tabNumber(k string) (tea.Model, tea.Cmd, bool) {
	pos := int(k[0] - '1')
	visible := m.visibleTabs()
	if pos < len(visible) && !m.cmOn() && !m.typing() {
		m.tab = m.tabIDFromPos(pos)
		return m.onTab(), nil, true
	}
	return m, nil, false
}

// cycleTab moves to the next visible tab.
func (m Model) cycleTab() (tea.Model, tea.Cmd, bool) {
	if m.cmOn() || m.typing() {
		return m, nil, true
	}
	visible := m.visibleTabs()
	currentPos := m.tabPosFromID(m.tab)
	if currentPos >= 0 {
		nextPos := (currentPos + 1) % len(visible)
		m.tab = m.tabIDFromPos(nextPos)
	} else {
		m.tab = m.tabIDFromPos(0)
	}
	return m.onTab(), nil, true
}

// updateTab sends the key to the current tab. q and ctrl+c quit only if no tab takes them.
func (m Model) updateTab(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.tab {
	case 0:
		return m.updateList(msg)
	case 1:
		return m.updateContests(msg)
	case 2:
		return m.updateStats(msg)
	case 3:
		return m.updatePicker(msg)
	case 4:
		return m.updateSettings(msg)
	}
	if k := msg.String(); k == "q" || k == "ctrl+c" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	case "pgup":
		m.cursor = max(0, m.cursor-m.page())
	case "pgdown":
		m.cursor = min(len(m.visible)-1, m.cursor+m.page())
	case "f":
		m = m.openFilters()
	case "m":
		if len(m.visible) > 0 {
			m = m.toggleMark(m.visible[m.cursor])
		}
	case "Y": // sync solved marks from the providers that support it
		if m.deps.Sync == nil || !m.sourceOn(cf.SourceCSES) {
			m.Notice = "turn CSES on in Settings to sync it"
			break
		}
		sync := m.deps.Sync
		m.Notice = "syncing CSES..."
		return m, func() tea.Msg {
			text, err := sync(cf.SourceCSES)
			return syncDoneMsg{text, err}
		}
	case ":":
		m.input, m.inputErr = &input{kind: 'f', text: m.filter.Expr()}, ""
	case "X":
		m.filter, m.filterExpr, m.cursor = Filter{}, "", 0
		return m.refilter(), nil
	case "/":
		m.input, m.inputErr = &input{kind: '/', text: m.filter.Search, prev: m.filter.Search}, ""
	case "enter":
		if len(m.visible) == 0 {
			break
		}
		return m.openProblem(m.visible[m.cursor])
	}
	return m, nil
}

func (m Model) openProblem(p cf.Problem) (tea.Model, tea.Cmd) {
	m = m.closeProblem()
	m.open, m.openTab, m.detail, m.body, m.errMsg, m.scroll, m.loading = &p, m.tab, nil, nil, "", 0, true
	cmds := []tea.Cmd{m.load(p, false)}
	if m.deps.Watch != nil {
		ctx, cancel := context.WithCancel(context.Background())
		if ch, err := m.deps.Watch(ctx, p); err == nil {
			m.watchCancel = cancel
			cmds = append(cmds, listenWatch(ch))
		} else {
			cancel()
		}
	}
	return m, tea.Batch(cmds...)
}

// closeProblem leaves the Problem view: stops its run and watcher.
// viewing reports whether the Problem view is on screen: a Problem is open and its tab is current.
func (m Model) viewing() bool { return m.open != nil && m.tab == m.openTab }

func (m Model) closeProblem() Model {
	m = m.stopTests().stopStress()
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.pending = nil
	m.open, m.detail, m.body, m.errMsg, m.loading = nil, nil, nil, "", false
	m.marks = nil
	return m
}

func (m Model) updateProblem(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.sel = selection{}
	if m.confirm {
		return m.updateConfirm(msg)
	}
	if m.full {
		return m.updateFull(msg)
	}
	if r := m.run; r != nil && r.diff {
		return m.updateDiff(r, msg)
	}
	if m.strs != nil && m.strs.diff {
		return m.updateStressOverlay(msg)
	}
	if nm, cmd, handled := m.updateStress(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.updateTests(msg); handled {
		return nm, cmd
	}
	k := msg.String()
	if nm, cmd, ok := m.problemTop(k); ok {
		return nm, cmd
	}
	if nm, cmd, ok := m.problemEdit(k); ok {
		return nm, cmd
	}
	if nm, cmd, ok := m.problemLayout(k); ok {
		return nm, cmd
	}
	return m, nil
}

// updateConfirm answers the prompt asking whether to submit despite failing local tests.
func (m Model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m.startSubmit()
	case "n", "esc":
		m.confirm = false
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// updateDiff handles keys while a Test Run's diff is open.
func (m Model) updateDiff(r *testRun, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "d", "q":
		r.diff = false
	case "e":
		if r.overall == runner.CE {
			return m.editAtError()
		}
	case "j", "down":
		r.diffOff++
	case "k", "up":
		r.diffOff = max(0, r.diffOff-1)
	case "pgdown":
		r.diffOff += m.page()
	case "pgup":
		r.diffOff = max(0, r.diffOff-m.page())
	case "ctrl+c":
		return m.stopTests(), tea.Quit
	}
	return m, nil
}

// problemTop handles quit, closing the Problem, panels, tabs and pane focus.
func (m Model) problemTop(k string) (tea.Model, tea.Cmd, bool) {
	switch k {
	case "ctrl+c", "q":
		return m, tea.Quit, true
	case "esc":
		if !m.cmOn() { // contest mode always shows a Problem
			m = m.closeProblem()
		}
		return m, nil, true
	case "1", "2", "3", "4", "5": // the Problem stays open; esc closes it
		pos := int(k[0] - '1')
		visible := m.visibleTabs()
		if pos < len(visible) && !m.cmOn() {
			m.tab = m.tabIDFromPos(pos)
			return m.onTab(), nil, true
		}
		return m, nil, true
	case "f":
		m.full, m.fullOff = true, 0
	case "v":
		m.showTags = !m.showTags
	case "P":
		return m.cyclePalette(), nil, true
	case "tab":
		m.pane = (m.pane + 1) % panes
	case "shift+tab":
		m.pane = (m.pane + panes - 1) % panes
	default:
		return m, nil, false
	}
	return m, nil, true
}

// problemEdit handles the keys that change the Problem or open something for it.
func (m Model) problemEdit(k string) (tea.Model, tea.Cmd, bool) {
	switch k {
	case "y":
		nm, cmd := m.copyPane()
		return nm, cmd, true
	case "I":
		nm, cmd := m.copyCase("input")
		return nm, cmd, true
	case "E":
		nm, cmd := m.copyCase("expected")
		return nm, cmd, true
	case "O":
		nm, cmd := m.copyCase("output")
		return nm, cmd, true
	case "r":
		return m.refetchProblem()
	case "c":
		return m.cycleMode(), nil, true
	case "s":
		nm, cmd := m.trySubmit()
		return nm, cmd, true
	case "l":
		return m.pickLang(), nil, true
	case "e":
		if m.deps.Edit == nil {
			return m, nil, true
		}
		cmd, err := m.deps.Edit(*m.open, m.lang())
		nm, c := m.afterOpen(cmd, err, "edit")
		return nm, c, true
	case "N":
		if m.deps.EditNote == nil {
			return m, nil, true
		}
		cmd, err := m.deps.EditNote(*m.open)
		nm, c := m.afterOpen(cmd, err, "note")
		return nm, c, true
	case "i":
		return m.openSnippets(m.lang()), nil, true
	case "H":
		nm, cmd := m.openEditorial(*m.open)
		return nm, cmd, true
	case "T":
		return m.openTM(false), nil, true
	case "a":
		return m.openTM(true), nil, true
	case "m":
		return m.toggleMark(*m.open), nil, true
	case "o":
		return m.openBrowser()
	}
	return m, nil, false
}

// problemLayout handles the split-view resize keys and scrolling.
func (m Model) problemLayout(k string) (tea.Model, tea.Cmd, bool) {
	switch k {
	case ">", ".":
		m.wAdj += 4
	case "<", ",":
		m.wAdj -= 4
	case "+", "=":
		m.hAdj++
	case "-":
		m.hAdj--
	case "up", "k":
		return m.moveInPane(-1), nil, true
	case "down", "j":
		return m.moveInPane(1), nil, true
	case "pgup":
		return m.moveInPane(-m.page()), nil, true
	case "pgdown":
		return m.moveInPane(m.page()), nil, true
	case "home", "g":
		return m.moveInPane(-1 << 30), nil, true
	case "end", "G":
		return m.moveInPane(1 << 30), nil, true
	default:
		return m, nil, false
	}
	return m, nil, true
}

// refetchProblem reloads the statement from the network; offline it says why not.
func (m Model) refetchProblem() (tea.Model, tea.Cmd, bool) {
	if m.offline {
		m.errMsg = "offline: refetch needs network (ctrl+r on a list retries the connection)"
		return m, nil, true
	}
	m.loading, m.errMsg = true, ""
	return m, m.load(*m.open, true), true
}

// cycleMode moves the Comparison Mode to the next one and saves it.
func (m Model) cycleMode() Model {
	if m.detail == nil {
		return m
	}
	m.pstate.Mode = nextMode(m.mode())
	if m.deps.SaveState != nil {
		if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
			m.errMsg = "saving mode: " + err.Error()
		}
	}
	return m
}

// pickLang moves to the next configured language.
func (m Model) pickLang() Model {
	if len(m.deps.Langs) == 0 || m.detail == nil {
		return m
	}
	return m.chooseLang(nextOf(m.deps.Langs, m.lang()))
}

// openBrowser opens the Problem page in the browser.
func (m Model) openBrowser() (tea.Model, tea.Cmd, bool) {
	if open, p := m.deps.OpenURL, *m.open; open != nil {
		return m, func() tea.Msg {
			open(p.URL())
			return nil
		}, true
	}
	return m, nil, true
}

// moveInPane scrolls the focused pane (or moves the test selection) by d lines.
func (m Model) moveInPane(d int) Model {
	if _, _, ok := m.split(); ok {
		switch m.pane {
		case paneTests:
			m.tsel = max(0, min(m.tsel+d, len(m.rows())-1))
			m.detScroll = 0
			return m
		case paneDetail:
			m.detScroll = max(0, m.detScroll+d)
			return m
		}
	}
	m.scroll = max(0, min(m.scroll+d, m.maxScroll()))
	return m
}

// content is the whole Problem view as lines: header, statement, Sample Tests.
func (m Model) content() []string {
	p := *m.open
	lines := []string{fmt.Sprintf("%s  %s", p.Code(), clean(p.Name))}
	if m.detail != nil {
		d := m.detail
		lim := fmt.Sprintf("time %.4g s   memory %d MB", float64(d.TimeLimitMS)/1000, d.MemoryLimitMB)
		if d.Interactive {
			lim += "   [interactive: local run unsupported]"
		}
		lines = append(lines, lim)
		if m.deps.Note != nil {
			if first := firstLine(m.deps.Note(p)); first != "" {
				lines = append(lines, "note: "+clean(first))
			}
		}
		lines = append(lines, "")
		lines = append(lines, m.submissionLines()...)
		lines = append(lines, m.testsPanel()...)
		lines = append(lines, m.stressPanel()...)
		lines = append(lines, "")
		lines = append(lines, m.paintStmt(m.body)...)
		for i, s := range d.Samples {
			lines = append(lines, "", fmt.Sprintf("Sample %d input:", i+1))
			lines = append(lines, cleanLines(s.Input)...)
			lines = append(lines, fmt.Sprintf("Sample %d output:", i+1))
			lines = append(lines, cleanLines(s.Output)...)
		}
	}
	return lines
}

var modes = []string{"tokens", "exact", "float", "none"}

func nextMode(cur string) string { return nextOf(modes, cur) }

// nextOf returns the entry after cur, wrapping; the first if cur is not in the list.
func nextOf(list []string, cur string) string {
	for i, x := range list {
		if x == cur {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// lang is the language in effect: the user's choice if still configured, else the default.
func (m Model) lang() string {
	for _, l := range m.deps.Langs {
		if l == m.pstate.Lang {
			return l
		}
	}
	if d := m.defaultLang(); d != "ask" {
		return d
	}
	if len(m.deps.Langs) > 0 {
		return m.deps.Langs[0] // "ask" not answered yet: the first language
	}
	return "cpp"
}

// defaultLang is default_lang as set now ("ask" asks each time a Problem is opened).
func (m Model) defaultLang() string {
	if d := m.setting("default_lang"); d != "" {
		return d
	}
	return m.deps.DefaultLang
}

// afterOpen handles the result of opening files in the editor: an error message, a split pane
// (nil cmd: poll until it closes), or a foreground editor to suspend for.
func (m Model) afterOpen(cmd *exec.Cmd, err error, what string) (tea.Model, tea.Cmd) {
	if err != nil {
		m.errMsg = what + ": " + err.Error()
		return m, nil
	}
	m.errMsg = ""
	if cmd == nil {
		if m.editorOpen || m.deps.EditorAlive == nil {
			return m, nil
		}
		m.editorOpen = true
		return m, editorTick()
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{err} })
}

// mode is the Comparison Mode in effect: the user's choice, else the Problem's parsed hint.
func (m Model) mode() string {
	if m.pstate.Mode != "" {
		return m.pstate.Mode
	}
	if m.detail != nil {
		return m.detail.DefaultMode()
	}
	return "tokens"
}

func (m Model) maxScroll() int {
	if _, _, ok := m.split(); ok {
		return max(0, len(m.stmtLines())-(m.paneHeight()-2))
	}
	return max(0, len(m.content())-m.page())
}

// clean drops control characters (ESC etc.) so server-supplied text can't inject terminal escapes.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func cleanLines(s string) []string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range ls {
		ls[i] = clean(ls[i])
	}
	return ls
}

// hdrOn and ftrOn: whether the header and footer are drawn (zen mode hides both for the session).
func (m Model) hdrOn() bool { return !m.zen && m.cfgVals["header"] != "false" }
func (m Model) ftrOn() bool { return !m.zen && m.cfgVals["footer"] != "false" }

func (m Model) contentTop() int {
	if !m.hdrOn() {
		return 0
	}
	return 2 // header + rule
}

func (m Model) footerSize() int {
	if !m.ftrOn() {
		if m.zen && m.Notice != "" {
			return 1 // zen shows a notice (e.g. how to leave) and nothing else
		}
		return 0
	}
	return 2 // badges + hint bar
}

func (m Model) page() int {
	overhead := m.contentTop() + m.footerSize()
	return max(1, m.height-overhead)
}

// visibleTabs returns the list of tab names that should be shown, respecting config order.
func (m Model) visibleTabs() []string {
	if v, ok := m.cfgVals["tabs"]; ok && v != "" {
		// tabs value is stored as comma-separated in config
		parts := strings.Split(v, ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// validate it's a real tab
			for _, tab := range tabs {
				if strings.EqualFold(p, tab) {
					result = append(result, tab)
					break
				}
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	// default: all tabs
	return tabs
}

// tabIDFromPos maps a tab position (0-based visible tab index) to its absolute ID (0-4).
func (m Model) tabIDFromPos(pos int) int {
	visible := m.visibleTabs()
	if pos < 0 || pos >= len(visible) {
		return 0
	}
	for i, tab := range tabs {
		if tab == visible[pos] {
			return i
		}
	}
	return 0
}

// tabPosFromID maps an absolute tab ID (0-4) to its visible position, or -1 if hidden.
func (m Model) tabPosFromID(id int) int {
	if id < 0 || id >= len(tabs) {
		return 0
	}
	target := tabs[id]
	visible := m.visibleTabs()
	for i, tab := range visible {
		if tab == target {
			return i
		}
	}
	return -1
}

// screen is verd's own screen (everything except an embedded editor pane), truncated to m.width.
func (m Model) screen() string {
	if rw := m.railCols(m.width); rw > 0 && m.inner == 0 {
		return m.withRail(rw, Model.screen)
	}
	var b strings.Builder
	if m.hdrOn() {
		b.WriteString(m.header(m.width) + "\n" + m.rule(m.width) + "\n")
	}
	footer := m.footerLine(m.tabView(&b))
	// Status badges get their own line so the key hints can never push them off the pane.
	footerContent := footer
	if badges := m.badges(); len(badges) > 0 {
		footerContent += "\n" + strings.Join(badges, " ")
	}
	if m.zen && m.Notice != "" {
		b.WriteString("\n" + m.styles().Dim.Render(clean(m.Notice)))
	}
	if m.ftrOn() {
		// Footer is shown
		if t := m.viewToast(); t != "" {
			b.WriteString("\n" + t)
		}
		b.WriteString("\n" + footerContent)
	}
	// If footer is false, we don't append anything (footerContent is not needed)
	// Never let a line overflow the pane (styles are preserved by the ANSI-aware truncation).
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = xansi.Truncate(l, m.width, "…")
	}
	if bx := m.modalBox(); bx != nil {
		lines = m.overlayModal(lines, bx)
	}
	return strings.Join(lines, "\n")
}

// tabView writes the current screen's body to b and returns its key hints.
func (m Model) tabView(b *strings.Builder) string {
	switch {
	case m.viewing():
		return m.viewProblem(b)
	case m.tab == 0:
		return m.viewList(b)
	case m.tab == 1:
		return m.viewContests(b)
	case m.tab == 2:
		return m.viewStats(b)
	case m.tab == 3:
		return m.viewPicker(b)
	}
	return m.viewSettings(b)
}

// footerLine is the key hint line: the confirm prompt while asking, else the hints.
func (m Model) footerLine(footer string) string {
	st := m.styles()
	if m.confirm {
		return paintRow(" "+st.Warn.Render(m.confirmText()), m.width, st.Bar)
	}
	if m.cmOn() { // contest mode always shows a Problem: there is no back
		footer = strings.Replace(strings.Replace(footer, "  {problem.back} back", "", 1), "{problem.back} back  ", "", 1)
	}
	return m.hintBar(m.kx(footer), m.width)
}

// badges are the status tags shown under the key hints.
func (m Model) badges() []string {
	st := m.styles()
	var badges []string
	if m.Notice != "" {
		badges = append(badges, badge(st.Warn, clean(m.Notice)))
	}
	if b := m.embedBadge(); b != "" {
		badges = append(badges, badge(st.Accent2, strings.Trim(b, "[]")))
	} else if m.editorOpen {
		badges = append(badges, badge(st.Accent2, "editor open"))
	}
	switch {
	case m.syncing:
		badges = append(badges, badge(st.Warn, "syncing..."))
	case m.offline && m.syncedAt.IsZero():
		badges = append(badges, badge(st.Bad, "offline, never synced"))
	case m.offline:
		badges = append(badges, badge(st.Bad, fmt.Sprintf("offline, synced %s ago", ago(m.clock().Sub(m.syncedAt)))))
	}
	return badges
}

// overlayModal mutes the screen lines and lays the modal box over them.
func (m Model) overlayModal(lines []string, bx []string) []string {
	st := m.styles()
	for i, l := range lines { // scrim: mute what is behind the modal so it stands out
		lines[i] = st.Dim.Render(scrimText(l))
	}
	modalY := m.height - 3
	if !m.ftrOn() {
		modalY = m.height // no footer means more space for the modal
	}
	lines = overlay(lines, bx, m.width, modalY)
	for i, l := range lines {
		lines[i] = xansi.Truncate(l, m.width, "")
	}
	return lines
}

func (m Model) viewProblem(b *strings.Builder) string {
	if lw, rw, ok := m.split(); ok {
		return m.viewSplit(b, lw, rw)
	}
	lines := m.content()
	if m.loading {
		lines = append(lines, "", "loading...")
	}
	if m.errMsg != "" {
		lines = append(lines, "", m.styles().Bad.Render(m.kx(clean(m.errMsg))))
	}
	end := min(len(lines), m.scroll+m.page())
	for _, l := range lines[min(m.scroll, end):end] {
		b.WriteString(" " + l + "\n")
	}
	return "{problem.back} back  {problem.edit} edit  {problem.notes} note  {problem.submit} submit  {problem.add_test} add test  {problem.language} lang  {problem.run_tests} test  {problem.stress} stress  {problem.cycle_mode} mode  {problem.down}/{problem.up} scroll  {problem.refetch} refetch  {problem.open_browser} browser  {common.help} help  {problem.quit} quit"
}

func ago(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
