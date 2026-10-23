package tui

import (
	"errors"
	"image"
	"image/color"
	"math/rand"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
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
	if len(m.visible) != 2 {
		t.Fatalf("one mixed list:\n%s", plain(m))
	}
	mustHave(t, plain(m), "CSES1068", "4A")
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
	checkMarks(t, m, &marks)
}

// checkMarks toggles the CSES row with m, then checks a Codeforces row refuses the mark.
func checkMarks(t *testing.T, m Model, marks *[]string) {
	t.Helper()
	m.cursor = 1
	m, _ = send(m, "m")
	if len(*marks) != 1 || (*marks)[0] != "CSES1068" || m.statusOf(m.visible[1]) != store.StatusSolved {
		t.Fatalf("m marks a CSES task solved: %v", *marks)
	}
	m, _ = send(m, "m")
	if m.statusOf(m.visible[1]) == store.StatusSolved {
		t.Fatal("m again clears it")
	}
	m.cursor = 0
	m, _ = send(m, "m")
	if len(*marks) != 2 || !strings.Contains(m.Notice, "Submissions") {
		t.Fatalf("Codeforces rows cannot be marked: %v %q", *marks, m.Notice)
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

func TestPickerWorksWithOnlyCSES(t *testing.T) {
	ps := []cf.Problem{{ContestID: 1068, Index: cf.SourceCSES, Name: "Weird Algorithm"}}
	m := New(nil, "", Deps{}).WithData(Data{Problems: ps}).WithRand(rand.New(rand.NewSource(1)))
	m.cfgVals["source_cf"], m.cfgVals["source_cses"] = "false", "true"
	m, _ = send(m, "4")
	if m.picked == nil || m.picked.ContestID != 1068 {
		t.Fatalf("CSES-only picker found nothing:\n%s", plain(m))
	}
}

// drain runs cmd and everything it batches (skipping timers), feeding each message back to m.
func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			m = drain(t, m, c)
		}
	case imagesMsg, detailMsg:
		nm, next := m.Update(msg)
		m = drain(t, nm.(Model), next)
	}
	return m
}

func TestStatementImagesAreDrawn(t *testing.T) {
	pic := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			pic.Set(x, y, color.RGBA{200, 30, 30, 255})
		}
	}
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p>See</p><p><img src="/file/abc"></p><p>after</p></div>`}, nil
		},
		FetchImage: func(u string) (image.Image, error) {
			if u != "https://cses.fi/file/abc" {
				t.Errorf("image address: %s", u)
			}
			return pic, nil
		},
	}
	m := New([]cf.Problem{{ContestID: 1068, Index: cf.SourceCSES, Name: "N"}}, "", deps)
	m.cfgVals["images"] = "blocks"
	m, cmd := send(m, "enter")
	if out := plain(drain(t, m, cmd)); !strings.Contains(out, "▀") || strings.Contains(out, "VERDIMG") {
		t.Fatalf("image should be drawn as blocks, token hidden:\n%s", out)
	}
	m = New([]cf.Problem{{ContestID: 1068, Index: cf.SourceCSES, Name: "N"}}, "", deps)
	m.cfgVals["images"] = "off"
	m, cmd = send(m, "enter")
	if out := plain(drain(t, m, cmd)); strings.Contains(out, "▀") || !strings.Contains(out, "[image]") {
		t.Fatalf("images off should leave a link:\n%s", out)
	}
}

func TestFailedImageSaysWhy(t *testing.T) {
	deps := Deps{
		Load: func(cf.Problem, bool) (*scrape.Detail, error) {
			return &scrape.Detail{Statement: `<div class="problem-statement"><p><img src="https://i.imgur.com/x.png"></p></div>`}, nil
		},
		FetchImage: func(string) (image.Image, error) { return nil, errors.New("image: HTTP 403") },
	}
	m := New([]cf.Problem{{ContestID: 1, Index: "A", Name: "N"}}, "", deps)
	m.cfgVals["images"] = "blocks"
	m, cmd := send(m, "enter")
	if out := plain(drain(t, m, cmd)); !strings.Contains(out, "could not load: image: HTTP 403") {
		t.Fatalf("the reason should show:\n%s", out)
	}
}
