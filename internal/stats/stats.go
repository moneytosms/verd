// Package stats computes Profile Stats from cached Problems, Submissions and rating history.
package stats

import (
	"fmt"
	"sort"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

// Band limits and thresholds from the Profile Stats design.
const (
	bandBelow    = 200
	bandAbove    = 300
	minBandTag   = 20 // tags with fewer Problems in the band are ignored for strengths/weaknesses
	unratedBandL = 800
	unratedBandH = 1200
)

type Input struct {
	Problems    []cf.Problem
	Submissions []cf.Submission
	Rating      []cf.RatingChange // oldest first
	Now         time.Time
	Loc         *time.Location // for streak days; nil = Local
}

type TagStat struct {
	Tag          string
	Solved       int
	AvgRating    float64 // of solved, rated Problems; 0 if none
	FirstTryRate float64 // share of solved Problems whose first Submission was OK
	InBand       int     // Problems with this tag in the user's rating band
	SolvedInBand int
	Coverage     float64 // SolvedInBand / InBand
}

type Attempt struct {
	ContestID int
	Index     string
	Name      string
	Attempts  int
	Last      time.Time
}

type Stats struct {
	Solved, Attempted, Submissions int
	ACRate                         float64
	Rating, MaxRating              int
	Rank                           string
	RatingHistory                  []cf.RatingChange
	SolvedByRating                 map[int]int // bucket (800, 900, ...) -> solved
	SolvedUnrated                  int
	Tags                           []TagStat // by solved desc, then name
	Strengths, Weaknesses          []TagStat
	Streak, MaxStreak              int
	Verdicts                       map[string]int
	Unsolved                       []Attempt // most recently attempted first
	Band                           [2]int
}

type key struct {
	contest int
	index   string
}

func final(v string) bool { return v != "" && v != "TESTING" }

// Rank is the Codeforces title for a rating; "unrated" for 0.
func Rank(r int) string {
	switch {
	case r <= 0:
		return "unrated"
	case r < 1200:
		return "newbie"
	case r < 1400:
		return "pupil"
	case r < 1600:
		return "specialist"
	case r < 1900:
		return "expert"
	case r < 2100:
		return "candidate master"
	case r < 2300:
		return "master"
	case r < 2400:
		return "international master"
	case r < 2600:
		return "grandmaster"
	case r < 3000:
		return "international grandmaster"
	}
	return "legendary grandmaster"
}

// Compute derives Profile Stats. Submissions still TESTING are ignored.
func Compute(in Input) Stats {
	loc := in.Loc
	if loc == nil {
		loc = time.Local
	}
	st := Stats{SolvedByRating: map[int]int{}, Verdicts: map[string]int{}, RatingHistory: in.Rating}
	byID := make(map[key]cf.Problem, len(in.Problems))
	for _, p := range in.Problems {
		byID[key{p.ContestID, p.Index}] = p
	}

	// Per-Problem history, oldest first (Submission ids grow with time).
	subs := make([]cf.Submission, 0, len(in.Submissions))
	for _, s := range in.Submissions {
		if final(s.Verdict) {
			subs = append(subs, s)
		}
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].ID < subs[j].ID })
	type hist struct {
		firstOK   bool
		solved    bool
		attempts  int
		last      time.Time
		everySeen bool
	}
	h := map[key]*hist{}
	days := map[string]bool{}
	ok := 0
	for _, s := range subs {
		k := key{s.Problem.ContestID, s.Problem.Index}
		x := h[k]
		if x == nil {
			x = &hist{firstOK: s.Verdict == "OK"}
			h[k] = x
		}
		x.attempts++
		x.last = time.Unix(s.Created, 0)
		st.Verdicts[s.Verdict]++
		if s.Verdict == "OK" {
			ok++
			x.solved = true
			days[time.Unix(s.Created, 0).In(loc).Format("2006-01-02")] = true
		}
	}
	st.Submissions = len(subs)
	if len(subs) > 0 {
		st.ACRate = float64(ok) / float64(len(subs))
	}

	// Rating.
	if n := len(in.Rating); n > 0 {
		st.Rating = in.Rating[n-1].NewRating
		for _, r := range in.Rating {
			st.MaxRating = max(st.MaxRating, r.NewRating)
		}
	}
	st.Rank = Rank(st.Rating)
	lo, hi := unratedBandL, unratedBandH
	if st.Rating > 0 {
		lo, hi = st.Rating-bandBelow, st.Rating+bandAbove
	}
	st.Band = [2]int{lo, hi}

	// Totals, difficulty buckets, unsolved attempts.
	type tagAgg struct {
		solved, firstTry, rated int
		ratingSum               float64
	}
	tags := map[string]*tagAgg{}
	solvedKeys := map[key]bool{}
	for k, x := range h {
		p, known := byID[k]
		if x.solved {
			st.Solved++
			solvedKeys[k] = true
			if known && p.Rating > 0 {
				st.SolvedByRating[p.Rating/100*100]++
			} else {
				st.SolvedUnrated++
			}
			if known {
				for _, t := range p.Tags {
					a := tags[t]
					if a == nil {
						a = &tagAgg{}
						tags[t] = a
					}
					a.solved++
					if x.firstOK {
						a.firstTry++
					}
					if p.Rating > 0 {
						a.rated++
						a.ratingSum += float64(p.Rating)
					}
				}
			}
			continue
		}
		st.Attempted++
		name := ""
		if known {
			name = p.Name
		}
		st.Unsolved = append(st.Unsolved, Attempt{k.contest, k.index, name, x.attempts, x.last})
	}
	sort.Slice(st.Unsolved, func(i, j int) bool {
		a, b := st.Unsolved[i], st.Unsolved[j]
		if !a.Last.Equal(b.Last) {
			return a.Last.After(b.Last)
		}
		if a.ContestID != b.ContestID {
			return a.ContestID > b.ContestID
		}
		return a.Index < b.Index
	})

	// Band coverage per tag.
	inBand := map[string]int{}
	solvedInBand := map[string]int{}
	for _, p := range in.Problems {
		if p.Rating < lo || p.Rating > hi {
			continue
		}
		for _, t := range p.Tags {
			inBand[t]++
			if solvedKeys[key{p.ContestID, p.Index}] {
				solvedInBand[t]++
			}
		}
	}
	seen := map[string]bool{}
	add := func(t string) {
		if seen[t] {
			return
		}
		seen[t] = true
		ts := TagStat{Tag: t, InBand: inBand[t], SolvedInBand: solvedInBand[t]}
		if a := tags[t]; a != nil {
			ts.Solved = a.solved
			ts.FirstTryRate = float64(a.firstTry) / float64(a.solved)
			if a.rated > 0 {
				ts.AvgRating = a.ratingSum / float64(a.rated)
			}
		}
		if ts.InBand > 0 {
			ts.Coverage = float64(ts.SolvedInBand) / float64(ts.InBand)
		}
		st.Tags = append(st.Tags, ts)
	}
	for t := range tags {
		add(t)
	}
	for t := range inBand {
		add(t)
	}
	sort.Slice(st.Tags, func(i, j int) bool {
		a, b := st.Tags[i], st.Tags[j]
		if a.Solved != b.Solved {
			return a.Solved > b.Solved
		}
		return a.Tag < b.Tag
	})
	var eligible []TagStat
	for _, t := range st.Tags {
		if t.InBand >= minBandTag {
			eligible = append(eligible, t)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.Coverage != b.Coverage {
			return a.Coverage > b.Coverage
		}
		return a.Tag < b.Tag
	})
	n := min(3, len(eligible))
	st.Strengths = append([]TagStat(nil), eligible[:n]...)
	rest := eligible[n:] // disjoint from strengths when few tags qualify
	w := min(3, len(rest))
	for i := len(rest) - 1; i >= len(rest)-w; i-- {
		st.Weaknesses = append(st.Weaknesses, rest[i]) // weakest first
	}

	st.Streak, st.MaxStreak = streaks(days, in.Now.In(loc))
	return st
}

// streaks counts consecutive days with at least one solve. The current streak is still alive if
// the last solve was yesterday (today may not have happened yet).
func streaks(days map[string]bool, now time.Time) (cur, best int) {
	var all []string
	for d := range days {
		all = append(all, d)
	}
	sort.Strings(all)
	run := 0
	var prev time.Time
	for _, d := range all {
		t, _ := time.ParseInLocation("2006-01-02", d, now.Location())
		if run > 0 && sameNextDay(prev, t) {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
		prev = t
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for d := today; ; d = d.AddDate(0, 0, -1) {
		if days[d.Format("2006-01-02")] {
			cur++
			continue
		}
		if d.Equal(today) {
			continue // no solve yet today: yesterday may still be the end of the streak
		}
		break
	}
	return cur, best
}

// sameNextDay reports whether b is the calendar day after a (DST-safe).
func sameNextDay(a, b time.Time) bool {
	y, m, d := a.AddDate(0, 0, 1).Date()
	by, bm, bd := b.Date()
	return y == by && m == bm && d == bd
}

// String renders a TagStat's coverage for lists, e.g. "dp 62% (31/50)".
func (t TagStat) String() string {
	return fmt.Sprintf("%s %.0f%% (%d/%d)", t.Tag, t.Coverage*100, t.SolvedInBand, t.InBand)
}
