package scrape

import (
	"os"
	"reflect"
	"testing"
)

func TestParseFixtures(t *testing.T) {
	for _, tc := range []struct {
		file        string
		tl, ml      int
		interactive bool
		hint        string
		samples     []Sample
	}{
		{"1A.html", 1000, 256, false, "", []Sample{{"6 6 4\n", "4\n"}}},
		{"1010A.html", 1000, 256, false, "float", []Sample{
			{"2\n12\n11 8\n7 5\n", "10.0000000000\n"},
			{"3\n1\n1 4 1\n2 5 3\n", "-1\n"},
			{"6\n2\n4 6 3 3 5 6\n2 6 3 6 5 3\n", "85.4800000000\n"},
		}},
		{"1011D.html", 1000, 256, true, "", []Sample{{"5 2\n1\n-1\n-1\n1\n0\n", "1\n2\n4\n5\n3\n"}}},
		{"1167B.html", 1000, 256, true, "", []Sample{{"16\n64\n345\n672\n", "? 1 1\n? 2 2\n? 3 5\n? 4 6\n! 4 8 15 16 23 42\n"}}},
	} {
		b, err := os.ReadFile("testdata/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Parse(b)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if d.TimeLimitMS != tc.tl || d.MemoryLimitMB != tc.ml || d.Interactive != tc.interactive || d.Hint != tc.hint {
			t.Errorf("%s: got tl=%d ml=%d inter=%v hint=%q", tc.file, d.TimeLimitMS, d.MemoryLimitMB, d.Interactive, d.Hint)
		}
		if !reflect.DeepEqual(d.Samples, tc.samples) {
			t.Errorf("%s samples: got %q", tc.file, d.Samples)
		}
	}
}

func TestParseNewFormat(t *testing.T) {
	b, _ := os.ReadFile("testdata/1900A.html")
	d, err := Parse(b)
	if err != nil || len(d.Samples) != 1 {
		t.Fatalf("%v %+v", err, d)
	}
	if d.Samples[0].Input[:6] != "5\n3\n.." || d.Samples[0].Output != "2\n2\n5\n0\n2\n" {
		t.Errorf("new-format lines not joined: %q", d.Samples[0].Input)
	}
}

func TestParseNoStatement(t *testing.T) {
	if _, err := Parse([]byte("<html>nope</html>")); err != ErrNoStatement {
		t.Fatal(err)
	}
}
