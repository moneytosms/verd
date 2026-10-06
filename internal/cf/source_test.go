package cf

import "testing"

func TestSourceIdentity(t *testing.T) {
	cf := Problem{ContestID: 1900, Index: "A"}
	cses := Problem{ContestID: 1068, Index: SourceCSES}
	if cf.Source() != SourceCF || cses.Source() != SourceCSES {
		t.Fatal("source from index")
	}
	if cf.Code() != "1900A" || cses.Code() != "CSES1068" {
		t.Fatalf("%s %s", cf.Code(), cses.Code())
	}
	if cses.URL() != "https://cses.fi/problemset/task/1068" {
		t.Fatal(cses.URL())
	}
}
