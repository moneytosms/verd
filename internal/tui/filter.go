package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

// Filter narrows the Problems list. Zero value matches everything.
type Filter struct {
	MinRating, MaxRating int // 0 = unbounded
	Include, Exclude     []string
	Unsolved             bool
	Search               string
}

// ParseFilter parses e.g. "800-1200 +dp -graphs unsolved". Tags match by prefix, `_` stands for a space
// ("+two_pointers"). Ranges: "800-1200", "800-", "-1200" (a leading dash followed by digits).
func ParseFilter(expr string) (Filter, error) {
	var f Filter
	for _, tok := range strings.Fields(expr) {
		switch {
		case tok == "unsolved" || tok == "u":
			f.Unsolved = true
		case strings.HasPrefix(tok, "+") && len(tok) > 1:
			f.Include = append(f.Include, tag(tok[1:]))
		case strings.HasPrefix(tok, "-") && len(tok) > 1 && !isDigit(tok[1]):
			f.Exclude = append(f.Exclude, tag(tok[1:]))
		case isDigit(tok[0]) || tok[0] == '-':
			lo, hi, ok := strings.Cut(tok, "-")
			if !ok { // single rating, e.g. "1200"
				hi = lo
			}
			var err error
			if f.MinRating, err = atoi(lo); err != nil {
				return f, fmt.Errorf("bad rating %q", tok)
			}
			if f.MaxRating, err = atoi(hi); err != nil {
				return f, fmt.Errorf("bad rating %q", tok)
			}
			if f.MaxRating != 0 && f.MinRating > f.MaxRating {
				return f, fmt.Errorf("empty rating range %q", tok)
			}
		default:
			return f, fmt.Errorf("unknown filter %q (use 800-1200, +tag, -tag, unsolved)", tok)
		}
	}
	return f, nil
}

func tag(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", " ")) }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func atoi(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

func hasTagPrefix(p cf.Problem, prefix string) bool {
	for _, t := range p.Tags {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

// Match reports whether p passes every active criterion.
func (f Filter) Match(p cf.Problem, st store.Status) bool {
	if (f.MinRating > 0 || f.MaxRating > 0) && (p.Rating == 0 || p.Rating < f.MinRating || (f.MaxRating > 0 && p.Rating > f.MaxRating)) {
		return false
	}
	for _, t := range f.Include {
		if !hasTagPrefix(p, t) {
			return false
		}
	}
	for _, t := range f.Exclude {
		if hasTagPrefix(p, t) {
			return false
		}
	}
	if f.Unsolved && st == store.StatusSolved {
		return false
	}
	if q := strings.ToLower(strings.TrimSpace(f.Search)); q != "" {
		id := strings.ToLower(fmt.Sprintf("%d%s", p.ContestID, p.Index))
		if !strings.Contains(id, q) && !strings.Contains(strings.ToLower(p.Name), q) {
			return false
		}
	}
	return true
}
