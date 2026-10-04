package tui

import (
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

func TestFilterCombinations(t *testing.T) {
	p := cf.Problem{ContestID: 1900, Index: "A", Name: "Cover in Water", Rating: 800, Tags: []string{"constructive algorithms", "dp", "two pointers"}}
	for _, tc := range []struct {
		expr   string
		search string
		st     store.Status
		want   bool
	}{
		{"", "", 0, true},
		{"800-1200", "", 0, true},
		{"900-1200", "", 0, false},
		{"-800", "", 0, true}, // max only
		{"801-", "", 0, false},
		{"800", "", 0, true}, // exact
		{"+dp", "", 0, true},
		{"+dp +graphs", "", 0, false}, // include is AND
		{"+two_pointers", "", 0, true},
		{"+constr", "", 0, true}, // prefix
		{"-dp", "", 0, false},
		{"-graphs", "", 0, true},
		{"800-1000 +dp -graphs", "", 0, true},
		{"unsolved", "", store.StatusSolved, false},
		{"unsolved", "", store.StatusAttempted, true},
		{"", "1900a", 0, true},
		{"", "water", 0, true},
		{"", "nope", 0, false},
		{"+dp unsolved", "cover", store.StatusSolved, false},
	} {
		f, err := ParseFilter(tc.expr)
		if err != nil {
			t.Fatalf("%q: %v", tc.expr, err)
		}
		f.Search = tc.search
		if got := f.Match(p, tc.st); got != tc.want {
			t.Errorf("expr=%q search=%q st=%v: got %v", tc.expr, tc.search, tc.st, got)
		}
	}
	if f, _ := ParseFilter("800-1200"); f.Match(cf.Problem{Name: "unrated"}, 0) {
		t.Error("unrated Problem must not match a rating range")
	}
	for _, bad := range []string{"wat", "1200-800", "12x"} {
		if _, err := ParseFilter(bad); err == nil {
			t.Errorf("%q should error", bad)
		}
	}
}
