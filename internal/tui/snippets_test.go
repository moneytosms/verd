package tui

import (
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

func TestSnippetPickerFiltersAndCopies(t *testing.T) {
	var asked []string
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>hi</p></div>`, TimeLimitMS: 1000}, nil
		},
		Langs: []string{"cpp", "python"}, DefaultLang: "cpp",
		Snippets: func(lang string) []Snippet {
			asked = append(asked, lang)
			return []Snippet{
				{Name: "bfs", Desc: "breadth first search", Code: "// bfs " + lang + "\nvoid bfs() {}\n"},
				{Name: "dijkstra", Desc: "shortest paths", Code: "// dijkstra\n"},
				{Name: "modpow", Desc: "modular exponentiation", Code: "// modpow\n"},
			}
		},
	}
	m, cmd := send(New([]cf.Problem{{ContestID: 1, Index: "A", Name: "N"}}, "", deps), "enter")
	m, _ = step(t, m, cmd)

	m, _ = send(m, "i")
	if m.sp == nil {
		t.Fatal("picker should open with the library")
	}
	mustHave(t, plain(m), "Snippets", "dijkstra")
	// typing filters by name, then by description; the preview follows the selection
	m = sendKeys(m, "m", "o")
	mustHave(t, plain(m), "modpow")
	mustLack(t, plain(m), "dijkstra")
	m = sendKeys(m, "backspace", "backspace", "s", "h", "o", "r")
	mustHave(t, plain(m), "dijkstra")
	mustLack(t, plain(m), "modpow")
	// tab switches language and reloads the list
	m, _ = send(m, "tab")
	if len(asked) != 2 || asked[1] != "python" {
		t.Fatalf("tab should reload for python, asked %v", asked)
	}
	// enter copies and closes; esc closes without copying
	m, cmd = send(m, "enter")
	if m.sp != nil || !strings.Contains(m.Notice, "copied snippet") || cmd == nil {
		t.Fatalf("enter should copy and close: %q", m.Notice)
	}
	m, _ = send(m, "i")
	m, _ = send(m, "esc")
	if m.sp != nil || m.open == nil {
		t.Fatal("esc closes only the picker")
	}
}

// sendKeys presses each key in turn.
func sendKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		m, _ = send(m, k)
	}
	return m
}
