package cses

import (
	"os"
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

func TestParseList(t *testing.T) {
	b, _ := os.ReadFile("testdata/list.html")
	ps, err := ParseList(b)
	if err != nil || len(ps) < 5 {
		t.Fatalf("%d problems, %v", len(ps), err)
	}
	p := ps[0]
	if p.ContestID != 1068 || p.Index != cf.SourceCSES || p.Name != "Weird Algorithm" || len(p.Tags) != 1 || p.Tags[0] != "Introductory Problems" || p.SolvedCount == 0 {
		t.Fatalf("%+v", p)
	}
	for _, q := range ps {
		if q.Name == "Introduction" || q.Name == "Statistics" {
			t.Fatalf("non-task link listed: %+v", q)
		}
	}
}

func TestParseTask(t *testing.T) {
	b, _ := os.ReadFile("testdata/task1068.html")
	d, err := ParseTask(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.TimeLimitMS != 1000 || d.MemoryLimitMB != 512 {
		t.Fatalf("limits %d ms %d MB", d.TimeLimitMS, d.MemoryLimitMB)
	}
	if len(d.Samples) != 1 || d.Samples[0].Input != "3\n" || d.Samples[0].Output != "3 10 5 16 8 4 2 1\n" {
		t.Fatalf("samples %+v", d.Samples)
	}
	if strings.Contains(d.Statement, "Example") || strings.Contains(d.Statement, "<pre>") {
		t.Fatal("the example belongs in Samples, not the statement")
	}
	out, err := scrape.Render(d.Statement, 80, "dark", scrape.DefaultOptions())
	if err != nil || !strings.Contains(out, "Weird Algorithm") && !strings.Contains(out, "Consider an algorithm") || !strings.Contains(out, "10") {
		t.Fatalf("render: %v\n%s", err, out)
	}
}

func TestParseTaskWithSeveralExamples(t *testing.T) {
	b, _ := os.ReadFile("testdata/task1070.html")
	d, err := ParseTask(b)
	if err != nil || len(d.Samples) != 2 || d.Samples[1].Input != "3\n" || d.Samples[1].Output != "NO SOLUTION\n" {
		t.Fatalf("%+v %v", d.Samples, err)
	}
	if strings.Contains(d.Statement, "Example") {
		t.Fatal("example headings belong out of the statement")
	}
}
