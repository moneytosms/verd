// Package session scores a timed contest session (virtual replay or practice) from Submissions.
package session

import (
	"sort"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

const (
	Virtual  = "virtual"  // replay a finished contest from now, for its duration
	Practice = "practice" // same view, no deadline and no penalty
	Live     = "live"     // the real contest: its own clock, results from the Codeforces standings
)

// PenaltyMinutes is added per wrong attempt before the accepted one (ICPC style).
const PenaltyMinutes = 20

// Session is the running contest session; there is at most one.
type Session struct {
	ContestID int
	Name      string
	Mode      string
	Start     int64 // unix seconds
	End       int64 // unix seconds; 0 = no deadline (practice)
}

// Started reports whether the clock is running; a live session can be created before the start.
func (s Session) Started(now time.Time) bool { return now.Unix() >= s.Start }

// Over reports whether the deadline has passed.
func (s Session) Over(now time.Time) bool { return s.End > 0 && now.Unix() >= s.End }

// Left is the time remaining, 0 once over; practice has no limit and returns 0 too.
func (s Session) Left(now time.Time) time.Duration {
	if s.End == 0 {
		return 0
	}
	return max(0, time.Unix(s.End, 0).Sub(now))
}

// Elapsed is the time since the start.
func (s Session) Elapsed(now time.Time) time.Duration { return max(0, now.Sub(time.Unix(s.Start, 0))) }

// UntilStart is the wait before a live session begins, 0 once it has.
func (s Session) UntilStart(now time.Time) time.Duration {
	return max(0, time.Unix(s.Start, 0).Sub(now))
}

// Row is one problem's result.
type Row struct {
	Index  string
	Solved bool
	Minute int // minutes from the start to the accepted submission
	Wrong  int // wrong attempts before it (all of them while unsolved)
}

// Result is the local score: problems solved and total penalty minutes.
type Result struct {
	Rows    []Row
	Solved  int
	Penalty int
}

// ignored verdicts do not count as an attempt: still judging, or not the contestant's fault.
var ignored = map[string]bool{"": true, "TESTING": true, "COMPILATION_ERROR": true, "SKIPPED": true, "CHALLENGED": true, "REJECTED": true}

// Score computes a session's result for the contest's problem indexes. Only Submissions made inside the
// session window count, so a replay never sees what was solved before it.
func Score(s Session, indexes []string, subs []cf.Submission) Result {
	mine := make([]cf.Submission, 0, len(subs))
	for _, x := range subs {
		if x.Problem.ContestID != s.ContestID || x.Created < s.Start || (s.End > 0 && x.Created > s.End) {
			continue
		}
		mine = append(mine, x)
	}
	sort.SliceStable(mine, func(i, j int) bool { return mine[i].Created < mine[j].Created })
	byIndex := map[string]*Row{}
	res := Result{Rows: make([]Row, len(indexes))}
	for i, idx := range indexes {
		res.Rows[i].Index = idx
		byIndex[idx] = &res.Rows[i]
	}
	for _, x := range mine {
		r := byIndex[x.Problem.Index]
		if r == nil || r.Solved || ignored[x.Verdict] {
			continue
		}
		if x.Verdict == "OK" {
			r.Solved, r.Minute = true, int((x.Created-s.Start)/60)
			continue
		}
		r.Wrong++
	}
	for _, r := range res.Rows {
		if r.Solved {
			res.Solved++
			if s.Mode != Practice {
				res.Penalty += r.Minute + PenaltyMinutes*r.Wrong
			}
		}
	}
	return res
}
