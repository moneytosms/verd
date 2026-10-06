// Package tui is the root Bubble Tea model.
package tui

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/runner"
	"github.com/moneytosms/verd/internal/scrape"
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
	// Edit creates the Problem's Solution if needed and opens it in the editor.
	// A nil cmd with a nil error means the editor opened in a split pane (see EditorAlive).
	Edit func(p cf.Problem, lang string) (*exec.Cmd, error)
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
	SetMark   func(p cf.Problem, on bool) error
	SaveCreds func(cookie, ua string) (string, error)
	HasCreds  func() bool
	// EditConfig opens config.toml in the editor; LangSummary describes each configured language.
	EditConfig  func() (*exec.Cmd, error)
	LangSummary []string
	// Reload re-reads the cache (no network), e.g. after a Submission lands.
	Reload func() (Data, error)
	// Submit copies the Solution, opens the submit page and tracks the Submission (see SubmitStart).
	Submit func(ctx context.Context, p cf.Problem, lang string) (SubmitStart, error)
	// EmbedRatio is verd's share of the width when an editor is embedded (default 0.4);
	// FocusKey toggles keyboard focus between verd and the editor (default ctrl+\\).
	EmbedRatio float64
	FocusKey   string
	// Watch reports saved files in the Problem's directory (debounced) until ctx is done.
	Watch func(ctx context.Context, p cf.Problem) (<-chan string, error)
	// SolutionPath is where the Problem's Solution in lang lives.
	SolutionPath func(p cf.Problem, lang string) string
	// Autotest runs a Test Run whenever the active Solution is saved.
	Autotest bool
	// LoadState and SaveState persist per-Problem choices (Comparison Mode).
	LoadState func(p cf.Problem) ProblemState
	SaveState func(p cf.Problem, s ProblemState) error
	// Refresh stages run in order in the background; each syncs stale data from the network and
	// returns the data now in the cache (valid even when err != nil, e.g. offline). A network
	// failure (cf.ErrNetwork) skips the remaining stages.
	Refresh []func() (Data, error)
}

// ProblemState is the user's per-Problem choices; empty fields mean "use the default".
type ProblemState struct{ Lang, Mode string }

// RunOpts are the per-run choices handed to Deps.Tests.
type RunOpts struct {
	Lang string
	Mode runner.Mode
}

// Data is everything the UI renders from the cache.
type Data struct {
	Handle   string
	Rating   int // latest rating, 0 = unrated
	Problems []cf.Problem
	Contests []cf.Contest
	Statuses map[string]store.Status
	Stats    stats.Stats
	SyncedAt time.Time // zero = never fully synced
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
	Notice   string // transient note shown in the footer
	deps     Deps
	status   map[string]store.Status

	tab           int
	contests      []cf.Contest
	upcoming      int // contests[:upcoming] are not finished
	contestCursor int
	contestOpen   *cf.Contest
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
	cases       []Case // Sample Tests then Custom Tests of the open Problem
	tsel        int    // selected row of the Tests pane
	pane        int    // focused pane of the split Problem view
	detScroll   int
	wAdj, hAdj  int  // user resize of the split view: right column width, Tests pane height
	dragging    bool // dragging the column divider
	sel         selection
	showTags    bool // tags revealed in the Problem pane
	bodyW       int  // width the statement was rendered at
	tm          *testMgr
	fm          *filterMgr
	sortBy      string            // "", "rating", "-rating", "solved", "id"
	cfgVals     map[string]string // current config values shown in Settings
	setSel      int
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
	return Model{cfgVals: vals, Problems: ps, visible: ps, Notice: note, deps: deps, width: 80, height: 24, syncing: len(deps.Refresh) > 0, dark: true, theme: mustTheme("terminal")}
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

// WithData replaces everything rendered from the cache, keeping view state.
func (m Model) WithData(d Data) Model {
	m.handle, m.rating, m.syncedAt = d.Handle, d.Rating, d.SyncedAt
	m.Problems, m.status, m.stats = d.Problems, d.Statuses, d.Stats
	m.statsSel = min(m.statsSel, max(0, len(d.Stats.Unsolved)-1))
	m = m.WithContests(d.Contests)
	return m.refilter()
}

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
	switch m.sortBy {
	case "rating", "-rating":
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
	m.cursor = min(m.cursor, max(0, len(m.visible)-1))
	return m
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
				d := PickFilter(m.stats.Rating)
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
func (m Model) contentWidth() int {
	if m.embed.term != nil {
		if left, right := m.layout(); right > 0 {
			return left
		}
	}
	return m.width
}

func (m Model) Init() tea.Cmd {
	if len(m.deps.Refresh) == 0 {
		return tea.RequestBackgroundColor
	}
	return tea.Batch(tea.RequestBackgroundColor, m.refresh(0))
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
		body, err := scrape.Render(d.Statement, width-4, style)
		var state ProblemState
		if loadState != nil {
			state = loadState(p)
		}
		return detailMsg{p: p, d: d, w: width - 4, body: body, state: state, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd := m.update(msg)
	if mm, ok := nm.(Model); ok {
		nm = mm.rewrap()
	}
	return nm, cmd
}

// rewrap re-renders the statement when the pane it lives in changed width (resize, layout switch).
func (m Model) rewrap() Model {
	if m.detail == nil || m.open == nil || m.bodyW == 0 || m.stmtWidth() == m.bodyW {
		return m
	}
	w := m.stmtWidth()
	if body, err := scrape.Render(m.detail.Statement, w, m.styles().Glamour); err == nil {
		m.body, m.bodyW = strings.Split(body, "\n"), w
		m.scroll = min(m.scroll, m.maxScroll())
	}
	return m
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if nm, cmd, ok := m.onEmbedMsg(msg); ok {
		return nm, cmd
	}
	nm, cmd, ok := m.routeEmbedInput(msg)
	if ok {
		return nm, cmd
	}
	m = nm // a click on verd's side moves focus even though the click is also handled below
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeEmbed()
	case tea.BackgroundColorMsg:
		if m.bgMode == "" || m.bgMode == "auto" {
			m.dark = msg.IsDark()
		}
	case refreshedMsg:
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
	case submitStartedMsg, ExternalSubmit, subUpdateMsg, toastClearMsg, reloadMsg:
		nm, cmd, _ := m.onSubmitMsg(msg)
		return nm, cmd
	case testEventMsg:
		return m.onTestEvent(msg)
	case ExternalRun:
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
	case stressEventMsg:
		return m.onStressEvent(msg)
	case watchMsg:
		if msg.closed || m.open == nil {
			return m, nil
		}
		cmd := listenWatch(msg.ch)
		if m.deps.Autotest && m.detail != nil && m.deps.SolutionPath != nil && msg.path == m.deps.SolutionPath(*m.open, m.lang()) {
			nm, run := m.startTests()
			return nm, tea.Batch(cmd, run)
		}
		return m, cmd
	case editorTickMsg:
		if m.deps.EditorAlive != nil && m.deps.EditorAlive() {
			return m, editorTick()
		}
		m.editorOpen = false // the pane closed
		m = m.loadCases()    // the user may have added or edited tests there
	case editorDoneMsg:
		if msg.err != nil {
			m.errMsg = "editor: " + msg.err.Error()
		}
		m = m.loadCases()
	case detailMsg:
		if m.open == nil || m.open.ContestID != msg.p.ContestID || m.open.Index != msg.p.Index {
			break // user already left or switched Problems
		}
		m.loading = false
		switch {
		case errors.Is(msg.err, cf.ErrNetwork):
			m.offline = true
			if m.detail == nil {
				m.errMsg = "offline: statement not cached. Press o to open in browser."
			} else {
				m.errMsg = "offline: could not refetch"
			}
		case errors.Is(msg.err, cf.ErrChallenge):
			m.errMsg = "Codeforces blocked the request (Cloudflare). Press o to open in browser."
		case msg.err != nil:
			m.errMsg = "load failed: " + msg.err.Error()
		default:
			m.errMsg, m.detail, m.scroll, m.pstate = "", msg.d, 0, msg.state
			m.body, m.bodyW = strings.Split(msg.body, "\n"), msg.w
			m = m.loadCases()
			if m.pending != nil {
				r := *m.pending
				m.pending = nil
				return m.attachExternal(r)
			}
		}
	case syncDoneMsg:
		m.Notice = msg.text
		if msg.err != nil {
			m.Notice = clean(msg.err.Error())
			return m, nil
		}
		return m, m.reload()
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m.onClick(msg.X, msg.Y)
		}
	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft {
			m = m.dragSelect(msg.X, msg.Y)
		}
		if m.dragging && msg.Button == tea.MouseLeft {
			if g, ok := m.splitGeom(); ok {
				m.wAdj += g.lw - msg.X // divider follows the pointer
			}
		}
	case tea.MouseReleaseMsg:
		m.dragging = false
		nm, cmd := m.endSelect()
		return nm, cmd
	case tea.MouseWheelMsg:
		return m.onWheel(msg.X, msg.Y, msg.Button == tea.MouseWheelUp)
	case tea.PasteMsg:
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
	case tea.KeyPressMsg:
		if m.tm != nil {
			return m.updateTM(msg)
		}
		if m.fm != nil {
			return m.updateFilters(msg)
		}
		if m.subModal {
			switch msg.String() {
			case "q", "esc", "enter":
				m.subModal = false
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.help {
			return m.updateHelp(msg)
		}
		if msg.String() == "x" && m.toast != nil && m.input == nil {
			m.toast = nil
			return m, nil
		}
		if msg.String() == "?" && m.input == nil {
			m.help, m.helpPage, m.helpScroll = true, 0, 0
			return m, nil
		}
		if m.open != nil {
			return m.updateProblem(msg)
		}
		if m.input != nil {
			return m.updateInput(msg)
		}
		switch k := msg.String(); k {
		case "ctrl+r":
			if len(m.deps.Refresh) == 0 || m.syncing {
				return m, nil
			}
			m.syncing = true
			return m, m.refresh(0)
		case "1", "2", "3", "4", "5":
			m.tab = int(k[0] - '1')
			return m.onTab(), nil
		case "tab":
			m.tab = (m.tab + 1) % len(tabs)
			return m.onTab(), nil
		}
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
	m.open, m.detail, m.body, m.errMsg, m.scroll, m.loading = &p, nil, nil, "", 0, true
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
func (m Model) closeProblem() Model {
	m = m.stopTests().stopStress()
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.pending = nil
	m.open, m.detail, m.body, m.errMsg, m.loading = nil, nil, nil, "", false
	return m
}

func (m Model) updateProblem(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.sel = selection{}
	if m.confirm {
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
	if r := m.run; r != nil && r.diff {
		switch msg.String() {
		case "esc", "d", "q":
			r.diff = false
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
	if m.strs != nil && m.strs.diff {
		return m.updateStressOverlay(msg)
	}
	if nm, cmd, handled := m.updateStress(msg); handled {
		return nm, cmd
	}
	if nm, cmd, handled := m.updateTests(msg); handled {
		return nm, cmd
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m = m.closeProblem()
	case "1", "2", "3", "4", "5":
		m = m.closeProblem()
		m.tab = int(msg.String()[0] - '1')
		return m.onTab(), nil
	case "v":
		m.showTags = !m.showTags
	case "y":
		return m.copyPane()
	case ">", ".":
		m.wAdj += 4
	case "<", ",":
		m.wAdj -= 4
	case "+", "=":
		m.hAdj++
	case "-":
		m.hAdj--
	case "r":
		if m.offline {
			m.errMsg = "offline: refetch needs network (ctrl+r on a list retries the connection)"
			break
		}
		m.loading, m.errMsg = true, ""
		return m, m.load(*m.open, true)
	case "c":
		if m.detail == nil {
			break
		}
		m.pstate.Mode = nextMode(m.mode())
		if m.deps.SaveState != nil {
			if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
				m.errMsg = "saving mode: " + err.Error()
			}
		}
	case "s":
		return m.trySubmit()
	case "l":
		if len(m.deps.Langs) == 0 || m.detail == nil {
			break
		}
		lang := nextOf(m.deps.Langs, m.lang())
		if m.deps.Ensure != nil {
			if err := m.deps.Ensure(*m.open, lang); err != nil {
				m.errMsg = "language " + lang + ": " + err.Error()
				break
			}
		}
		m.pstate.Lang = lang
		m.errMsg = ""
		if m.deps.SaveState != nil {
			if err := m.deps.SaveState(*m.open, m.pstate); err != nil {
				m.errMsg = "saving language: " + err.Error()
			}
		}
	case "e":
		if m.deps.Edit == nil {
			break
		}
		cmd, err := m.deps.Edit(*m.open, m.lang())
		return m.afterOpen(cmd, err, "edit")
	case "N":
		if m.deps.EditNote == nil {
			break
		}
		cmd, err := m.deps.EditNote(*m.open)
		return m.afterOpen(cmd, err, "note")
	case "T":
		m = m.openTM(false)
	case "a":
		m = m.openTM(true)
	case "tab":
		m.pane = (m.pane + 1) % panes
	case "shift+tab":
		m.pane = (m.pane + panes - 1) % panes
	case "m":
		m = m.toggleMark(*m.open)
	case "o":
		if open, p := m.deps.OpenURL, *m.open; open != nil {
			return m, func() tea.Msg {
				open(p.URL())
				return nil
			}
		}
	case "up", "k":
		m = m.moveInPane(-1)
	case "down", "j":
		m = m.moveInPane(1)
	case "pgup":
		m = m.moveInPane(-m.page())
	case "pgdown":
		m = m.moveInPane(m.page())
	case "home", "g":
		m = m.moveInPane(-1 << 30)
	case "end", "G":
		m = m.moveInPane(1 << 30)
	}
	return m, nil
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
		lines = append(lines, m.body...)
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

func (m Model) page() int { return max(1, m.height-4) }

// screen is verd's own screen (everything except an embedded editor pane), truncated to m.width.
func (m Model) screen() string {
	st := m.styles()
	var b strings.Builder
	b.WriteString(m.header(m.width) + "\n" + m.rule(m.width) + "\n")
	footer := ""
	switch {
	case m.open != nil:
		footer = m.viewProblem(&b)
	case m.tab == 0:
		footer = m.viewList(&b)
	case m.tab == 1:
		footer = m.viewContests(&b)
	case m.tab == 2:
		footer = m.viewStats(&b)
	case m.tab == 3:
		footer = m.viewPicker(&b)
	default:
		footer = m.viewSettings(&b)
	}
	if m.confirm {
		footer = paintRow(" "+st.Warn.Render(m.confirmText()), m.width, st.Bar)
	} else {
		footer = m.hintBar(footer, m.width)
	}
	// Status badges get their own line so the key hints can never push them off the pane.
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
	if len(badges) > 0 {
		footer += "\n" + strings.Join(badges, " ")
	}
	if t := m.viewToast(); t != "" {
		b.WriteString("\n" + t)
	}
	b.WriteString("\n" + footer)
	// Never let a line overflow the pane (styles are preserved by the ANSI-aware truncation).
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = xansi.Truncate(l, m.width, "…")
	}
	if bx := m.modalBox(); bx != nil {
		for i, l := range lines { // scrim: mute what is behind the modal so it stands out
			lines[i] = st.Dim.Render(xansi.Strip(l))
		}
		lines = overlay(lines, bx, m.width, m.height-3) // keep the footer and toast visible
		for i, l := range lines {
			lines[i] = xansi.Truncate(l, m.width, "")
		}
	}
	return strings.Join(lines, "\n")
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
		lines = append(lines, "", m.styles().Bad.Render(clean(m.errMsg)))
	}
	end := min(len(lines), m.scroll+m.page())
	for _, l := range lines[min(m.scroll, end):end] {
		b.WriteString(" " + l + "\n")
	}
	return "esc back  e edit  N note  s submit  a add test  l lang  t test  S stress  c mode  j/k scroll  r refetch  o browser  ? help  q quit"
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
