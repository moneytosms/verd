package runner

import (
	"strings"
	"testing"
)

func TestExplain(t *testing.T) {
	asan := "==1==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x1\n    #0 0x1 in main /home/u/1/A/main.cpp:12:9\n"
	py := "Traceback (most recent call last):\n  File \"/w/main.py\", line 4, in <module>\n    a[9]\nIndexError: list index out of range\n"
	for _, c := range []struct {
		name string
		r    Result
		hint string
		loc  string
	}{
		{"asan", Result{Verdict: RE, Stderr: asan}, "past the end of a heap", "/home/u/1/A/main.cpp:12"},
		{"python", Result{Verdict: RE, Stderr: py}, "List index", "/w/main.py:4"},
		{"segv", Result{Verdict: RE, Signal: "SIGSEGV"}, "Invalid memory access", ""},
		{"fpe", Result{Verdict: RE, Signal: "SIGFPE"}, "division or modulo by zero", ""},
		{"exit", Result{Verdict: RE, ExitCode: 3}, "code 3", ""},
		{"tle", Result{Verdict: TLE}, "Too slow", ""},
		{"wa", Result{Verdict: WA, Stderr: asan}, "", ""},
	} {
		got := Explain(c.r)
		if !strings.Contains(got.Hint, c.hint) || got.Loc != c.loc || (c.hint == "" && got.Hint != "") {
			t.Errorf("%s: got %+v", c.name, got)
		}
	}
}
