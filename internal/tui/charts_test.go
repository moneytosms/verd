package tui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func join(ls []string) string { return strings.Join(ls, "\n") }

// Golden outputs at fixed widths: any change to chart rendering must be a conscious one.
func TestLineChartGolden(t *testing.T) {
	got := join(LineChart([]int{1000, 1100, 1050, 1300, 1250, 1500, 1600, 1550}, 20, 4))
	want := `⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⠤⠒⠉⠑⠒⠤
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⡰⠁⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⡔⠉⠑⠒⠊⠀⠀⠀⠀⠀⠀⠀⠀
⣀⠤⠒⠒⠤⠤⠊⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀`
	if got != want {
		t.Fatalf("width 20:\n%s\nwant\n%s", got, want)
	}
	got = join(LineChart([]int{1, 2, 3, 2, 1}, 10, 3))
	want = `⠀⠀⠀⢠⠊⠢⡀⠀⠀⠀
⠀⢀⠔⠁⠀⠀⠑⢄⠀⠀
⡠⠃⠀⠀⠀⠀⠀⠀⠑⢄`
	if got != want {
		t.Fatalf("width 10:\n%s\nwant\n%s", got, want)
	}
}

func TestLineChartEdges(t *testing.T) {
	if LineChart(nil, 10, 3) != nil || LineChart([]int{1}, 0, 3) != nil || LineChart([]int{1}, 10, 0) != nil {
		t.Fatal("degenerate sizes give no chart")
	}
	for name, vals := range map[string][]int{"single": {5}, "flat": {7, 7, 7, 7}, "two": {1, 9}} {
		ls := LineChart(vals, 12, 3)
		if len(ls) != 3 {
			t.Fatalf("%s: rows %d", name, len(ls))
		}
		dots := 0
		for _, l := range ls {
			if utf8.RuneCountInString(l) != 12 {
				t.Fatalf("%s: width %d, want 12", name, utf8.RuneCountInString(l))
			}
			for _, r := range l {
				if r != 0x2800 {
					dots++
				}
			}
		}
		if dots == 0 {
			t.Fatalf("%s: nothing drawn", name)
		}
	}
	// narrow pane: width 1 still works
	if ls := LineChart([]int{1, 5, 2}, 1, 2); len(ls) != 2 || utf8.RuneCountInString(ls[0]) != 1 {
		t.Fatalf("%v", ls)
	}
}

func TestBarsGolden(t *testing.T) {
	got := join(Bars([]int{0, 1, 2, 4, 8, 3, 0, 1}, 4))
	want := `    █   
    █   
   ██▄  
 ▄████ ▄`
	if got != want {
		t.Fatalf("\n%s\nwant\n%s", got, want)
	}
	if Bars([]int{0, 0}, 2)[1] != "  " {
		t.Fatal("all-zero draws nothing")
	}
	if Bars(nil, 3) != nil || Bars([]int{1}, 0) != nil {
		t.Fatal("degenerate")
	}
	if got := Bars([]int{1, 1000}, 1)[0]; []rune(got)[0] != '▁' {
		t.Fatalf("a non-zero count must stay visible next to a huge one: %q", got)
	}
	if got := Axis("800", "3500", 28); got != "800                     3500" || len(got) != 28 {
		t.Fatalf("%q", got)
	}
	if got := Axis("800", "3500", 5); got != "800 3500" {
		t.Fatalf("narrow axis: %q", got)
	}
}
