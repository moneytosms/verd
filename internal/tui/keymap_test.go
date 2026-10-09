package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
)

func TestParseKeyRoundTripsEveryDefault(t *testing.T) {
	for _, a := range keyActions {
		for _, k := range a.def {
			msg, err := parseKey(k)
			if err != nil || msg.String() != k {
				t.Errorf("%s: parseKey(%q) -> %q, %v", actionID(a), k, msg.String(), err)
			}
		}
	}
}

func TestDefaultKeymapHasNoConflicts(t *testing.T) {
	if _, err := newKeymap(nil); err != nil {
		t.Fatal(err)
	}
}

func TestKeymapRejectsBadBindings(t *testing.T) {
	for name, user := range map[string]map[string][]string{
		"unknown action": {"problem.nope": {"x"}},
		"bad key":        {"problem.run_tests": {"nonsense"}},
		"ctrl+c":         {"problem.run_tests": {"ctrl+c"}},
		"conflict":       {"problem.run_tests": {"s"}}, // s is submit
	} {
		if _, err := newKeymap(user); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestRebindProblemKey(t *testing.T) {
	deps := Deps{Keys: map[string][]string{"problem.run_tests": {"ctrl+t"}, "problems.filters": {"F"}}}
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "X"}}, "", deps)
	// list: F opens filters, the old f does nothing
	m, _ = send(m, "f")
	if m.fm != nil {
		t.Fatal("f was rebound away")
	}
	m, _ = send(m, "F")
	if m.fm == nil {
		t.Fatal("F should open the filters")
	}
	m, _ = send(m, "esc")
	// Problem view: t is dead, ctrl+t runs the tests
	m, _ = send(m, "enter")
	if m.open == nil {
		t.Fatal("enter opens the Problem")
	}
	m.detail = nil
	pressed := func(m Model, msg tea.KeyPressMsg) Model {
		nm, _ := m.Update(msg)
		return nm.(Model)
	}
	m = pressed(m, tea.KeyPressMsg{Code: 't', Text: "t"})
	if m.run != nil {
		t.Fatal("t was rebound away")
	}
	if got, drop := m.km.translate("problem", "ctrl+t"); drop || got != "t" {
		t.Fatalf("ctrl+t should translate to t: %q %v", got, drop)
	}
}

func TestKeyEditorRebindsAndSaves(t *testing.T) {
	var saved []string
	m := New(nil, "", Deps{SaveKeys: func(ctx, id string, ks []string) error {
		saved = append([]string{ctx + "." + id}, ks...)
		return nil
	}})
	m, _ = send(m, "5")
	for m.setSel < len(settingDefs) && settingDefs[m.setSel].key != "keys" {
		m, _ = send(m, "j")
	}
	m, _ = send(m, "enter")
	if m.ke == nil || !strings.Contains(plain(m), "Keyboard shortcuts") {
		t.Fatalf("editor should open:\n%s", plain(m))
	}
	// first row is common.help: rebind it to F1-less "H"
	m, _ = send(m, "enter")
	m, _ = send(m, "H")
	if len(saved) != 2 || saved[0] != "common.help" || saved[1] != "H" {
		t.Fatalf("saved %v", saved)
	}
	if got := m.km.user["common.help"]; len(got) != 1 || got[0] != "H" {
		t.Fatalf("keymap not updated: %v", got)
	}
	// a conflicting key is refused and nothing is saved
	saved = nil
	m, _ = send(m, "enter")
	m, _ = send(m, "x") // x is dismiss_toast
	if saved != nil || !strings.Contains(m.ke.note, "bound to both") {
		t.Fatalf("conflict should be refused: %v %q", saved, m.ke.note)
	}
	// backspace resets
	m, _ = send(m, "backspace")
	if _, ok := m.km.user["common.help"]; ok {
		t.Fatal("reset should drop the override")
	}
	m, _ = send(m, "esc")
	if m.ke != nil {
		t.Fatal("esc closes the editor")
	}
}

func TestKeyEditorFuzzySearchAndHintsFollowBindings(t *testing.T) {
	var saved []string
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "X"}}, "", Deps{SaveKeys: func(ctx, id string, ks []string) error {
		saved = append([]string{ctx + "." + id}, ks...)
		return nil
	}})
	m, _ = send(m, "5")
	for settingDefs[m.setSel].key != "keys" {
		m, _ = send(m, "j")
	}
	m, _ = send(m, "enter")
	m, _ = send(m, "/")
	m = typeText(m, "stres")
	rows := m.keRows()
	if len(rows) == 0 || rows[0].id != "stress" {
		t.Fatalf("fuzzy 'stres' should rank problem.stress first: %v", rows)
	}
	m, _ = send(m, "enter") // done typing
	m, _ = send(m, "enter") // rebind
	m, _ = send(m, "ctrl+k")
	if len(saved) != 2 || saved[0] != "problem.stress" || saved[1] != "ctrl+k" {
		t.Fatalf("saved %v", saved)
	}
	m, _ = send(m, "esc") // clears the filter
	if m.ke == nil || m.ke.q != "" {
		t.Fatal("first esc clears the filter")
	}
	m, _ = send(m, "esc")
	m, _ = send(m, "1")
	m, _ = send(m, "enter")
	if m.open == nil || !strings.Contains(plain(m), "ctrl+k stress") {
		t.Fatalf("footer should show the new key:\n%s", plain(m))
	}
	if strings.Contains(plain(m), " S stress") {
		t.Fatal("old key still in the footer")
	}
}

func TestKeyTableMarkdownContainsAllActions(t *testing.T) {
	md := KeyTableMarkdown()
	infos := KeyActions(nil)
	for _, info := range infos {
		id := "`" + info.Action + "`"
		if !strings.Contains(md, "| "+id+" |") {
			t.Errorf("action %q not found in markdown", info.Action)
		}
	}
	// Check that each context appears exactly once as a heading
	for _, g := range keyGroupOrder {
		title := keyGroupTitle[g]
		count := strings.Count(md, "## "+title)
		if count != 1 {
			t.Errorf("## %s should appear exactly once in markdown, got %d", title, count)
		}
	}
}

func TestDocsKeysMarkdownInSync(t *testing.T) {
	// Read docs/keys.md relative to the package directory
	keysFile := filepath.Join("..", "..", "docs", "keys.md")
	b, err := os.ReadFile(keysFile)
	if err != nil {
		t.Fatalf("read %s: %v (run from the repository root or package dir)", keysFile, err)
	}
	text := string(b)
	const begin, end = "<!-- keys:begin -->", "<!-- keys:end -->"
	beginIdx := strings.Index(text, begin)
	endIdx := strings.Index(text, end)
	if beginIdx == -1 || endIdx == -1 || beginIdx >= endIdx {
		t.Fatalf("markers not found in docs/keys.md (need %q and %q)", begin, end)
	}
	// Extract the block content (what's between the markers)
	blockStart := beginIdx + len(begin)
	blockContent := text[blockStart:endIdx]
	// Trim leading/trailing whitespace
	blockContent = strings.TrimSpace(blockContent)
	// Get the generated markdown
	generated := strings.TrimSpace(KeyTableMarkdown())
	if blockContent != generated {
		t.Errorf("docs/keys.md keys block is out of sync\nrun: go run ./cmd/verd keys markdown --write docs/keys.md")
	}
}
