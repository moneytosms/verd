package tui

import (
	"strings"
	"unicode"

	"github.com/moneytosms/verd/internal/cf"
)

// fuzzyScore reports whether query is a case-insensitive subsequence of target and how good the
// match is: consecutive runs and word starts score higher, gaps cost.
func fuzzyScore(query, target string) (int, bool) {
	q, t := []rune(strings.ToLower(query)), []rune(strings.ToLower(target))
	if len(q) == 0 {
		return 0, true
	}
	score, qi, last := 0, 0, -2
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if t[ti] != q[qi] {
			continue
		}
		switch {
		case ti == last+1:
			score += 8 // run
		case ti == 0 || !unicode.IsLetter(t[ti-1]) && !unicode.IsDigit(t[ti-1]):
			score += 6 // word start
		default:
			score += 1 - min(3, ti-last-1) // gap
		}
		if ti == 0 {
			score += 4
		}
		last = ti
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	return score - len(t)/8, true // prefer tighter targets
}

// Search modes cycled by tab in the search prompt.
var searchModes = []string{"all", "name", "tag", "id"}

// searchScore matches every space-separated word of query (AND) against the fields selected by mode.
func searchScore(query, mode string, p cf.Problem) (int, bool) {
	id := p.Code()
	total := 0
	for _, w := range strings.Fields(query) {
		best, ok := wordScore(w, mode, id, p)
		if !ok {
			return 0, false
		}
		total += best
	}
	return total, true
}

// wordScore is the best score of one query word against the fields of p that mode selects.
func wordScore(w, mode, id string, p cf.Problem) (int, bool) {
	field := w
	if strings.HasPrefix(w, "#") && len(w) > 1 { // "#dp" always searches tags
		field, mode = w[1:], "tag"
	}
	best, ok := 0, false
	try := func(target string, bonus int) {
		if s, hit := fuzzyScore(field, target); hit && (!ok || s+bonus > best) {
			best, ok = s+bonus, true
		}
	}
	if mode == "all" || mode == "id" {
		try(id, 10)
	}
	if mode == "all" || mode == "name" {
		try(p.Name, 0)
	}
	if mode == "all" || mode == "tag" {
		for _, t := range p.Tags {
			try(t, -2)
		}
	}
	return best, ok
}
