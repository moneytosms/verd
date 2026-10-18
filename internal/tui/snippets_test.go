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
	if m.sp == nil || !strings.Contains(plain(m), "Snippets") || !strings.Contains(plain(m), "dijkstra") {
		t.Fatalf("picker should open with the library:\n%s", plain(m))
	}
	// typing filters by name, then by description; the preview follows the selection
	for _, k := range []string{"m", "o"} {
		m, _ = send(m, k)
	}
	if out := plain(m); !strings.Contains(out, "modpow") || strings.Contains(out, "dijkstra") {
		t.Fatalf("filter 'mo':\n%s", out)
	}
	for _, k := range []string{"backspace", "backspace", "s", "h", "o", "r"} {
		m, _ = send(m, k)
	}
	if out := plain(m); !strings.Contains(out, "dijkstra") || strings.Contains(out, "modpow") {
		t.Fatalf("filter by description 'shor':\n%s", out)
	}
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
