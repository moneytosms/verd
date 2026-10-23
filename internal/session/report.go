package session

import (
	"fmt"
	"strings"
	"time"
)

// ProblemStats is what verd saw of one problem during a session.
type ProblemStats struct {
	FirstOpen int64 `json:"first_open,omitempty"` // unix seconds it was first opened; 0 = never
	Seconds   int64 `json:"seconds,omitempty"`    // time it was the open problem
	Runs      int   `json:"runs,omitempty"`       // local Test Runs
	Green     int   `json:"green,omitempty"`      // Test Runs where every test passed
}

// Stats are a session's ProblemStats by problem index.
type Stats map[string]*ProblemStats

// Of returns idx's stats, adding them if absent.
func (s Stats) Of(idx string) *ProblemStats {
	if s[idx] == nil {
		s[idx] = &ProblemStats{}
	}
	return s[idx]
}

// Problem names one problem of the session's contest for a Report.
type Problem struct{ Index, Name, URL string }

// Report is the session as a Markdown page: when, the score line, a row per problem (result, time
// spent, Test Runs) and what is left to upsolve.
func Report(s Session, problems []Problem, res Result, st Stats, score string, now time.Time) string {
	end := now
	if s.End > 0 && now.Unix() > s.End {
		end = time.Unix(s.End, 0)
	}
	start := time.Unix(s.Start, 0)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s)\n\n", s.Name, s.Mode)
	fmt.Fprintf(&b, "- Date: %s to %s (%s)\n", start.Format("2006-01-02 15:04"), end.Format("15:04"), dur(end.Sub(start)))
	fmt.Fprintf(&b, "- Result: %s\n", score)
	fmt.Fprintf(&b, "- Contest: https://codeforces.com/contest/%d\n\n", s.ContestID)

	rows := map[string]Row{}
	for _, r := range res.Rows {
		rows[r.Index] = r
	}
	var spent, runs, green int64
	var upsolve []Problem
	b.WriteString("| Problem | Result | Wrong | Time spent | First opened | Test runs |\n|---|---|---|---|---|---|\n")
	for _, p := range problems {
		r, ps := rows[p.Index], st[p.Index]
		if ps == nil {
			ps = &ProblemStats{}
		}
		result := "-"
		if r.Solved {
			result = fmt.Sprintf("solved at %dm", r.Minute)
		} else {
			upsolve = append(upsolve, p)
		}
		opened := "-"
		if ps.FirstOpen > 0 {
			opened = fmt.Sprintf("%dm", (ps.FirstOpen-s.Start)/60)
		}
		fmt.Fprintf(&b, "| %s %s | %s | %d | %s | %s | %d (%d passing) |\n", p.Index, cell(p.Name), result, r.Wrong,
			dur(time.Duration(ps.Seconds)*time.Second), opened, ps.Runs, ps.Green)
		spent, runs, green = spent+ps.Seconds, runs+int64(ps.Runs), green+int64(ps.Green)
	}
	fmt.Fprintf(&b, "\n- Solved: %d/%d\n- Time on problems: %s\n- Test runs: %d (%d passing)\n", res.Solved, len(problems), dur(time.Duration(spent)*time.Second), runs, green)
	if res.Solved > 0 {
		var last int
		for _, r := range res.Rows {
			if r.Solved {
				last = max(last, r.Minute)
			}
		}
		fmt.Fprintf(&b, "- Last solve: %dm\n", last)
	}
	if len(upsolve) > 0 {
		b.WriteString("\n## Upsolve\n\n")
		for _, p := range upsolve {
			fmt.Fprintf(&b, "- [%s %s](%s)\n", p.Index, p.Name, p.URL)
		}
	}
	return b.String()
}

// dur is h:mm:ss, or m:ss under an hour.
func dur(d time.Duration) string {
	s := int(d.Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// cell keeps a table cell on one row.
func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }
