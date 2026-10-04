package runner

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mode      Mode
		want, got string
		ok        bool
		mismatch  *Mismatch
	}{
		{"tokens ignore whitespace", Tokens, "1 2\n3\n", "1\n2   3", true, nil},
		{"tokens trailing newline", Tokens, "yes\n", "yes", true, nil},
		{"tokens mismatch position", Tokens, "1 2\n3 4\n", "1 2\n3 5\n", false, &Mismatch{2, 3, "4", "5"}},
		{"tokens missing output", Tokens, "1 2\n3\n", "1 2\n", false, &Mismatch{2, 1, "3", ""}},
		{"tokens extra output", Tokens, "1\n", "1\n2\n", false, &Mismatch{2, 1, "", "2"}},
		{"tokens case sensitive", Tokens, "YES", "Yes", false, &Mismatch{1, 1, "YES", "Yes"}},
		{"tokens empty both", Tokens, "", "\n", true, nil},
		{"exact trailing ws ok", Exact, "a b \nc\n\n", "a b\nc", true, nil},
		{"exact inner ws matters", Exact, "a b\n", "a  b\n", false, &Mismatch{1, 3, "a b", "a  b"}},
		{"exact missing line", Exact, "a\nb\n", "a\n", false, &Mismatch{2, 1, "b", ""}},
		{"float within eps", Float, "0.3333333333\n", "0.333333\n", true, nil},
		{"float abs tolerance near zero", Float, "0\n", "0.0000005\n", true, nil},
		{"float relative for large", Float, "1000000\n", "1000000.5\n", true, nil},
		{"float outside eps", Float, "1.0\n", "1.1\n", false, &Mismatch{1, 1, "1.0", "1.1"}},
		{"float non-numeric must match", Float, "abc 1.0\n", "abd 1.0\n", false, &Mismatch{1, 1, "abc", "abd"}},
		{"float nan rejected", Float, "1\n", "nan\n", false, &Mismatch{1, 1, "1", "nan"}},
		{"none never judges", None, "1", "2", true, nil},
	} {
		ok, m := Compare(tc.mode, tc.want, tc.got, 1e-6)
		if ok != tc.ok || !reflect.DeepEqual(m, tc.mismatch) {
			t.Errorf("%s: got ok=%v %+v, want ok=%v %+v", tc.name, ok, m, tc.ok, tc.mismatch)
		}
	}
}
