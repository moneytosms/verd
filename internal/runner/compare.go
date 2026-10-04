package runner

import (
	"math"
	"strconv"
	"strings"
)

// Mode is how actual output is matched against expected output.
type Mode string

const (
	Tokens Mode = "tokens" // whitespace-insensitive token equality, like Codeforces' wcmp
	Exact  Mode = "exact"  // equal after trimming trailing whitespace per line and trailing blank lines
	Float  Mode = "float"  // tokens; numeric tokens within abs/rel eps
	None   Mode = "none"   // show output only, never judge
)

// Mismatch locates the first difference. Line/Col are 1-based positions in the actual output.
type Mismatch struct {
	Line, Col int
	Want, Got string // empty Want = extra output; empty Got = missing output
}

type token struct {
	s         string
	line, col int
}

func tokenize(s string) []token {
	var out []token
	line, col := 1, 1
	start := -1
	var sl, sc int
	flush := func(end int) {
		if start >= 0 {
			out = append(out, token{s[start:end], sl, sc})
			start = -1
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			flush(i)
			if c == '\n' {
				line, col = line+1, 1
			} else {
				col++
			}
			continue
		}
		if start < 0 {
			start, sl, sc = i, line, col
		}
		col++
	}
	flush(len(s))
	return out
}

// endPos is the position just past the last character of s.
func endPos(s string) (line, col int) {
	line, col = 1, 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return
}

// Compare reports whether actual matches expected under mode. eps is used by Float.
func Compare(mode Mode, expected, actual string, eps float64) (bool, *Mismatch) {
	switch mode {
	case None:
		return true, nil
	case Exact:
		return compareExact(expected, actual)
	}
	want, got := tokenize(expected), tokenize(actual)
	for i := 0; i < max(len(want), len(got)); i++ {
		switch {
		case i >= len(got):
			l, c := endPos(actual)
			return false, &Mismatch{Line: l, Col: c, Want: want[i].s}
		case i >= len(want):
			return false, &Mismatch{Line: got[i].line, Col: got[i].col, Got: got[i].s}
		case want[i].s == got[i].s:
		case mode == Float && floatEq(want[i].s, got[i].s, eps):
		default:
			return false, &Mismatch{Line: got[i].line, Col: got[i].col, Want: want[i].s, Got: got[i].s}
		}
	}
	return true, nil
}

func floatEq(a, b string, eps float64) bool {
	x, e1 := strconv.ParseFloat(a, 64)
	y, e2 := strconv.ParseFloat(b, 64)
	if e1 != nil || e2 != nil || math.IsNaN(x) || math.IsNaN(y) {
		return false
	}
	// absolute or relative error, like the Codeforces statements say
	return math.Abs(x-y) <= eps*math.Max(1, math.Abs(x))
}

func trimLines(s string) []string {
	ls := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i := range ls {
		ls[i] = strings.TrimRight(ls[i], " \t\r")
	}
	for len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func compareExact(expected, actual string) (bool, *Mismatch) {
	want, got := trimLines(expected), trimLines(actual)
	for i := 0; i < max(len(want), len(got)); i++ {
		switch {
		case i >= len(got):
			return false, &Mismatch{Line: i + 1, Col: 1, Want: want[i]}
		case i >= len(want):
			return false, &Mismatch{Line: i + 1, Col: 1, Got: got[i]}
		case want[i] != got[i]:
			col := 1
			for col <= len(want[i]) && col <= len(got[i]) && want[i][col-1] == got[i][col-1] {
				col++
			}
			return false, &Mismatch{Line: i + 1, Col: col, Want: want[i], Got: got[i]}
		}
	}
	return true, nil
}
