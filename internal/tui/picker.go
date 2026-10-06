package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

// PickFilter is the Picker's default: unsolved only, rating..rating+200 (800..1200 when unrated).
func PickFilter(rating int) Filter {
	f := Filter{Unsolved: true, MinRating: 800, MaxRating: 1200}
	if rating > 0 {
		f.MinRating, f.MaxRating = rating, rating+200
	}
	return f
}

// Pick returns a uniformly random Problem matching f, plus the match count. avoid (if there is
// another choice) is never returned, so a re-roll changes the Problem. ok is false when nothing matches.
func Pick(problems []cf.Problem, status func(cf.Problem) store.Status, f Filter, rng *rand.Rand, avoid *cf.Problem) (p cf.Problem, matches int, ok bool) {
	var hit []cf.Problem
	for _, q := range problems {
		if f.Match(q, status(q)) {
			hit = append(hit, q)
		}
	}
	if len(hit) == 0 {
		return cf.Problem{}, 0, false
	}
	pool := hit
	if avoid != nil && len(hit) > 1 {
		pool = pool[:0:0]
		for _, q := range hit {
			if q.ContestID != avoid.ContestID || q.Index != avoid.Index {
				pool = append(pool, q)
			}
		}
	}
	return pool[rng.Intn(len(pool))], len(hit), true
}

// WithRand sets the random source (tests seed it).
func (m Model) WithRand(r *rand.Rand) Model { m.rng = r; return m }

// weakFilter is the "weak topics" preset: the 3 weakest tags, in the user's band, unsolved.
func (m Model) weakFilter() (Filter, bool) {
	f := PickFilter(m.stats.Rating)
	for _, t := range m.stats.Weaknesses {
		f.AnyOf = append(f.AnyOf, t.Tag)
	}
	return f, len(f.AnyOf) > 0
}

// roll picks a Problem under the current Picker filter.
func (m Model) roll() Model {
	if m.rng == nil {
		m.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	p, n, ok := Pick(m.enabledProblems(), m.statusOf, m.pickFilter, m.rng, m.picked)
	m.pickMatches = n
	if !ok {
		m.picked = nil
		return m
	}
	m.picked = &p
	return m
}

// enterPicker prepares the Picker tab the first time it is shown.
func (m Model) enterPicker() Model {
	if !m.pickInit {
		m.pickInit, m.pickFilter = true, PickFilter(m.stats.Rating)
		m = m.roll()
	}
	return m
}

// withWeakPreset applies the weak-topics preset and re-rolls; without data it says why not.
func (m Model) withWeakPreset() Model {
	f, ok := m.weakFilter()
	m.pickInit = true
	if !ok {
		m.pickPreset = ""
		m.pickFilter = PickFilter(m.stats.Rating)
		m.pickNote = "no weak topics yet: sync more Submissions (a topic needs 20+ Problems in your band)"
		return m.roll()
	}
	m.pickPreset, m.pickFilter, m.pickExpr, m.pickNote = "weak topics", f, "", ""
	return m.roll()
}

func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "space", " ", "r":
		m.pickNote = ""
		return m.roll(), nil
	case "w":
		return m.withWeakPreset(), nil
	case "f":
		m.input, m.inputErr = &input{kind: 'p', text: m.pickExpr}, ""
	case "enter":
		if m.picked != nil {
			return m.openProblem(*m.picked)
		}
	}
	return m, nil
}

func (m Model) viewPicker(b *strings.Builder) string {
	st := m.styles()
	w := min(m.contentWidth()-2, 90)
	var card []string
	switch {
	case m.picked != nil:
		p := *m.picked
		card = append(card, st.Accent.Render(p.Code())+"  "+clean(p.Name), "")
		card = append(card, st.Dim.Render("rating ")+m.ratingText(p)+st.Dim.Render("    solved by ")+fmt.Sprint(p.SolvedCount))
		card = append(card, m.tagChips(p, w-6))
	case m.pickInit:
		card = append(card, st.Bad.Render("No Problem matches these filters."), st.Dim.Render("Press f to change them (e.g. widen the rating range) or r to retry."))
	default:
		card = append(card, st.Dim.Render("press space to draw a Problem"))
	}
	for _, l := range box("Problem Picker", card, "enter open · space re-roll", w, len(card)+2, true, st) {
		b.WriteString(" " + l + "\n")
	}
	b.WriteString("\n")
	f := m.pickFilter
	desc := fmt.Sprintf("rating %d-%d", f.MinRating, f.MaxRating)
	if m.pickPreset != "" {
		desc += ", preset: " + m.pickPreset + " (" + clean(strings.Join(f.AnyOf, ", ")) + ")"
	}
	if m.pickExpr != "" {
		desc = "filter: " + clean(m.pickExpr)
	}
	if f.Unsolved {
		desc += ", unsolved only"
	}
	b.WriteString("\n  " + st.Dim.Render(fmt.Sprintf("%d match(es)   %s", m.pickMatches, desc)) + "\n")
	if m.pickNote != "" {
		b.WriteString("  " + st.Warn.Render(clean(m.pickNote)) + "\n")
	}
	if m.input != nil && m.input.kind == 'p' {
		b.WriteString(fmt.Sprintf("\n%c %s_", 'f', m.input.text))
		if m.inputErr != "" {
			b.WriteString("   " + st.Bad.Render(m.inputErr))
		}
		return "enter apply  esc cancel"
	}
	return "space re-roll  enter open  f filters  w weak topics  ? help  q quit"
}
