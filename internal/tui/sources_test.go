package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

func mixedModel(settings map[string]string, marks *[]string) Model {
	ps := []cf.Problem{
		{ContestID: 4, Index: "A", Name: "Watermelon", Rating: 800, Tags: []string{"math"}},
		{ContestID: 1068, Index: cf.SourceCSES, Name: "Weird Algorithm", Tags: []string{"Introductory Problems"}},
	}
	deps := Deps{Settings: settings, SaveSetting: func(k, v string) error { return nil },
		SetMark: func(p cf.Problem, on bool) error { *marks = append(*marks, p.Code()); return nil }}
	m := New(ps, "", deps)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return nm.(Model).refilter()
}

func TestMixedListSourcesFilterAndMarks(t *testing.T) {
	var marks []string
	m := mixedModel(map[string]string{"source_cses": "true"}, &marks)
	if len(m.visible) != 2 || !strings.Contains(plain(m), "CSES") || !strings.Contains(plain(m), "Codeforces") || strings.Contains(plain(m), "CSES1068") {
		t.Fatalf("one mixed list:\n%s", plain(m))
	}
	f, err := ParseFilter("src:cses")
	if err != nil || f.Source != cf.SourceCSES || f.Expr() != "src:cses" {
		t.Fatalf("%+v %v", f, err)
	}
	m.filter = f
	if m = m.refilter(); len(m.visible) != 1 || m.visible[0].ContestID != 1068 {
		t.Fatalf("source filter: %+v", m.visible)
	}
	m.filter = Filter{}
	m = m.refilter()
	m.cursor = 1
	m, _ = send(m, "m")
	if len(marks) != 1 || marks[0] != "CSES1068" || m.statusOf(m.visible[1]) != store.StatusSolved {
		t.Fatalf("m marks a CSES task solved: %v", marks)
	}
	m, _ = send(m, "m")
	if m.statusOf(m.visible[1]) == store.StatusSolved {
		t.Fatal("m again clears it")
	}
	m.cursor = 0
	m, _ = send(m, "m")
	if len(marks) != 2 || !strings.Contains(m.Notice, "Submissions") {
		t.Fatalf("Codeforces rows cannot be marked: %v %q", marks, m.Notice)
	}
}

func TestSourceOffHidesItsProblems(t *testing.T) {
	var marks []string
	m := mixedModel(map[string]string{}, &marks) // CSES is off unless enabled
	if len(m.visible) != 1 || m.visible[0].Source() != cf.SourceCF {
		t.Fatalf("CSES hidden by default: %+v", m.visible)
	}
	m.cfgVals["source_cses"] = "true"
	if m = m.refilter(); len(m.visible) != 2 {
		t.Fatal("enabling shows it")
	}
}
