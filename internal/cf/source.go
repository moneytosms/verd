package cf

import "fmt"

// A Problem's source is part of its identity. Codeforces Problems have a contest id and a letter
// index. Every other source stores its own id in ContestID and its source tag in Index (no
// Codeforces index looks like a tag), so (ContestID, Index) stays unique across sources.
const (
	SourceCF   = "cf"
	SourceCSES = "cses"
)

// Sources lists every source in display order; the first is Codeforces.
var Sources = []string{SourceCF, SourceCSES}

// SourceName is the label shown for a source.
func SourceName(source string) string {
	switch source {
	case SourceCSES:
		return "CSES"
	}
	return "Codeforces"
}

// IndexSource reports the source an Index belongs to.
func IndexSource(index string) string {
	for _, s := range Sources[1:] {
		if index == s {
			return s
		}
	}
	return SourceCF
}

func (p Problem) Source() string { return IndexSource(p.Index) }

// Code is the short id shown in lists: "1900A", or "CSES1068".
func (p Problem) Code() string {
	if s := p.Source(); s != SourceCF {
		return fmt.Sprintf("%s%d", SourceName(s), p.ContestID)
	}
	return fmt.Sprintf("%d%s", p.ContestID, p.Index)
}

// URL is the Problem's statement page.
func (p Problem) URL() string {
	if p.Source() == SourceCSES {
		return fmt.Sprintf("https://cses.fi/problemset/task/%d", p.ContestID)
	}
	return fmt.Sprintf("%s/problemset/problem/%d/%s", BaseURL, p.ContestID, p.Index)
}
