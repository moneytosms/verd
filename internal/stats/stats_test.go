package stats

import (
	"fmt"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

func sub(id int64, contest int, index, verdict string, at time.Time) cf.Submission {
	s := cf.Submission{ID: id, Verdict: verdict, Created: at.Unix()}
	s.Problem.ContestID, s.Problem.Index = contest, index
	return s
}

var ist = time.FixedZone("IST", 5*3600+1800)

// totalsInput is a mixed history: duplicates, first tries, unrated and ignored verdicts.
func totalsInput(day time.Time) Input {
	return Input{
		Problems: []cf.Problem{
			{ContestID: 1, Index: "A", Name: "A", Rating: 800, Tags: []string{"math"}},
			{ContestID: 1, Index: "B", Name: "B", Rating: 850, Tags: []string{"math", "dp"}},
			{ContestID: 2, Index: "A", Name: "C", Rating: 1500, Tags: []string{"dp"}},
			{ContestID: 3, Index: "A", Name: "D", Tags: []string{"dp"}}, // unrated
		},
		Submissions: []cf.Submission{
			sub(1, 1, "A", "OK", day),
			sub(2, 1, "A", "OK", day.Add(time.Hour)), // duplicate AC: counts once
			sub(3, 1, "B", "WRONG_ANSWER", day),
			sub(4, 1, "B", "OK", day.Add(2*time.Hour)), // solved, not first try
			sub(5, 2, "A", "TIME_LIMIT_EXCEEDED", day), // attempted, never solved
			sub(6, 2, "A", "WRONG_ANSWER", day.Add(time.Hour)),
			sub(7, 3, "A", "OK", day),
			sub(8, 4, "Z", "OK", day),      // Problem missing from the cache: still solved, unrated
			sub(9, 5, "A", "TESTING", day), // ignored entirely
			sub(10, 1, "A", "", day),       // ignored (no verdict yet)
		},
		Now: day, Loc: ist,
	}
}

// tagStatByName returns the TagStat for name, or the zero value when absent.
func tagStatByName(ts []TagStat, name string) TagStat {
	for _, t := range ts {
		if t.Tag == name {
			return t
		}
	}
	return TagStat{}
}

func TestTotalsDuplicatesAndFirstTry(t *testing.T) {
	day := time.Date(2026, 10, 1, 12, 0, 0, 0, ist)
	st := Compute(totalsInput(day))
	t.Run("totals", func(t *testing.T) { checkTotals(t, st) })
	t.Run("buckets and verdicts", func(t *testing.T) { checkBucketsAndVerdicts(t, st) })
	t.Run("tags", func(t *testing.T) { checkTotalsTags(t, st) })
	t.Run("unsolved", func(t *testing.T) { checkTotalsUnsolved(t, st) })
}

func checkTotals(t *testing.T, st Stats) {
	if st.Solved != 4 || st.Attempted != 1 || st.Submissions != 8 {
		t.Fatalf("totals: solved=%d attempted=%d subs=%d", st.Solved, st.Attempted, st.Submissions)
	}
	if want := 5.0 / 8.0; st.ACRate != want { // 5 OK of 8 final
		t.Fatalf("AC rate %v want %v", st.ACRate, want)
	}
}

func checkBucketsAndVerdicts(t *testing.T, st Stats) {
	if st.SolvedByRating[800] != 2 || st.SolvedByRating[1500] != 0 || st.SolvedUnrated != 2 {
		t.Fatalf("buckets %v unrated %d", st.SolvedByRating, st.SolvedUnrated)
	}
	if st.Verdicts["OK"] != 5 || st.Verdicts["WRONG_ANSWER"] != 2 || st.Verdicts["TIME_LIMIT_EXCEEDED"] != 1 || st.Verdicts["TESTING"] != 0 {
		t.Fatalf("verdicts %v", st.Verdicts)
	}
}

func checkTotalsTags(t *testing.T, st Stats) {
	// math: solved 1A (first try) and 1B (not) -> 50%, avg rating 825
	if m := tagStatByName(st.Tags, "math"); m.Solved != 2 || m.FirstTryRate != 0.5 || m.AvgRating != 825 {
		t.Fatalf("math %+v", m)
	}
	// dp: solved 1B (not first try) and 3A (unrated, first try)
	if d := tagStatByName(st.Tags, "dp"); d.Solved != 2 || d.FirstTryRate != 0.5 || d.AvgRating != 850 {
		t.Fatalf("dp %+v", d)
	}
	if st.Tags[0].Tag != "dp" && st.Tags[0].Tag != "math" { // sorted by solved desc then name
		t.Fatalf("tag order %+v", st.Tags)
	}
}

func checkTotalsUnsolved(t *testing.T, st Stats) {
	if len(st.Unsolved) != 1 || st.Unsolved[0].ContestID != 2 || st.Unsolved[0].Attempts != 2 || st.Unsolved[0].Name != "C" {
		t.Fatalf("unsolved %+v", st.Unsolved)
	}
}

func TestRatingRankAndUnrated(t *testing.T) {
	st := Compute(Input{Now: time.Now()})
	if st.Rating != 0 || st.Rank != "unrated" || st.Band != [2]int{800, 1200} || st.ACRate != 0 || st.Solved != 0 {
		t.Fatalf("empty input: %+v", st)
	}
	st = Compute(Input{Rating: []cf.RatingChange{{NewRating: 1000}, {NewRating: 1650}, {NewRating: 1500}}, Now: time.Now()})
	if st.Rating != 1500 || st.MaxRating != 1650 || st.Rank != "specialist" || st.Band != [2]int{1300, 1800} {
		t.Fatalf("%+v", st)
	}
	for r, want := range map[int]string{0: "unrated", 1199: "newbie", 1200: "pupil", 1599: "specialist", 1600: "expert", 1900: "candidate master", 2100: "master", 2300: "international master", 2400: "grandmaster", 2600: "international grandmaster", 3000: "legendary grandmaster", 3800: "legendary grandmaster"} {
		if got := Rank(r); got != want {
			t.Errorf("Rank(%d) = %q want %q", r, got, want)
		}
	}
}

func TestStreakAcrossMidnight(t *testing.T) {
	at := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, ist) }
	// 23:50 on the 1st and 00:10 on the 2nd (local) are consecutive days, though 20 minutes apart
	in := Input{Loc: ist, Submissions: []cf.Submission{
		sub(1, 1, "A", "OK", at(1, 23, 50)),
		sub(2, 1, "B", "OK", at(2, 0, 10)),
		sub(3, 1, "C", "OK", at(2, 9, 0)), // same day: still one day
		sub(4, 1, "D", "OK", at(5, 8, 0)), // gap
		sub(5, 1, "E", "OK", at(6, 8, 0)),
		sub(6, 1, "F", "OK", at(7, 8, 0)),
	}}
	in.Now = at(7, 12, 0)
	st := Compute(in)
	if st.MaxStreak != 3 || st.Streak != 3 {
		t.Fatalf("streak=%d max=%d, want 3/3 (days 5,6,7)", st.Streak, st.MaxStreak)
	}
	in.Now = at(8, 12, 0) // no solve yet today: yesterday keeps the streak alive
	if st := Compute(in); st.Streak != 3 {
		t.Fatalf("streak should survive until end of today: %d", st.Streak)
	}
	in.Now = at(9, 12, 0) // a full day missed
	if st := Compute(in); st.Streak != 0 || st.MaxStreak != 3 {
		t.Fatalf("broken streak: %d max %d", st.Streak, st.MaxStreak)
	}
	// the same instants in UTC fall on different days: Loc matters
	in.Loc, in.Now = time.UTC, at(2, 12, 0)
	in.Submissions = in.Submissions[:2]
	if st := Compute(in); st.MaxStreak != 1 {
		t.Fatalf("23:50 IST and 00:10 IST are the same UTC day: max %d", st.MaxStreak)
	}
}

// bandInput has eight tags with 20 Problems each in band; tag i has i*2+1 solved.
// It also adds a tag below the 20-Problem minimum and out-of-band Problems.
func bandInput() Input {
	var problems []cf.Problem
	var subs []cf.Submission
	id := int64(0)
	for ti := 0; ti < 8; ti++ {
		tag := fmt.Sprintf("tag%d", ti)
		for pi := 0; pi < 20; pi++ {
			p := cf.Problem{ContestID: 100 + ti, Index: fmt.Sprintf("P%d", pi), Name: "x", Rating: 1400, Tags: []string{tag}}
			problems = append(problems, p)
			if pi < ti*2+1 && pi < 20 { // tag0:1 ... tag7:15 solved
				id++
				subs = append(subs, sub(id, p.ContestID, p.Index, "OK", time.Unix(1e9, 0)))
			}
		}
	}
	// a tag below the 20-Problem minimum must never appear as a strength or weakness
	for pi := 0; pi < 5; pi++ {
		problems = append(problems, cf.Problem{ContestID: 900, Index: fmt.Sprintf("S%d", pi), Rating: 1400, Tags: []string{"tiny"}})
	}
	// out-of-band Problems are not counted
	for pi := 0; pi < 30; pi++ {
		problems = append(problems, cf.Problem{ContestID: 901, Index: fmt.Sprintf("H%d", pi), Rating: 3000, Tags: []string{"tag0"}})
	}
	return Input{Problems: problems, Submissions: subs, Rating: []cf.RatingChange{{NewRating: 1500}}, Now: time.Now()}
}

// tagNames lists the tag names of ts in order.
func tagNames(ts []TagStat) []string {
	var o []string
	for _, t := range ts {
		o = append(o, t.Tag)
	}
	return o
}

func TestBandCoverageStrengthsWeaknesses(t *testing.T) {
	// rating 1500 -> band 1300..1800
	in := bandInput()
	st := Compute(in)
	t.Run("strengths and weaknesses", func(t *testing.T) { checkStrengthsWeaknesses(t, st) })
	t.Run("tag0 band numbers", func(t *testing.T) { checkTag0Band(t, st) })
	t.Run("no overlap", func(t *testing.T) { checkNoOverlap(t, in) })
}

func checkStrengthsWeaknesses(t *testing.T, st Stats) {
	if got := fmt.Sprint(tagNames(st.Strengths)); got != "[tag7 tag6 tag5]" {
		t.Fatalf("strengths %s", got)
	}
	if got := fmt.Sprint(tagNames(st.Weaknesses)); got != "[tag0 tag1 tag2]" {
		t.Fatalf("weaknesses (weakest first) %s", got)
	}
}

func checkTag0Band(t *testing.T, st Stats) {
	tag0 := tagStatByName(st.Tags, "tag0")
	if tag0.InBand != 20 || tag0.SolvedInBand != 1 || tag0.Coverage != 0.05 {
		t.Fatalf("tag0 band numbers: %+v", tag0)
	}
	if tag0.String() != "tag0 5% (1/20)" {
		t.Fatal(tag0.String())
	}
}

// few qualifying tags: strengths and weaknesses never overlap
func checkNoOverlap(t *testing.T, in Input) {
	in.Problems, in.Submissions = in.Problems[:60], in.Submissions[:3]
	st := Compute(in)
	seen := map[string]bool{}
	for _, ts := range append(append([]TagStat{}, st.Strengths...), st.Weaknesses...) {
		if seen[ts.Tag] {
			t.Fatalf("%s is both strength and weakness", ts.Tag)
		}
		seen[ts.Tag] = true
	}
}

func TestDaysCountsACPerDay(t *testing.T) {
	day := time.Date(2026, 10, 1, 12, 0, 0, 0, ist)
	st := Compute(Input{
		Submissions: []cf.Submission{sub(1, 1, "A", "OK", day), sub(2, 1, "B", "OK", day), sub(3, 1, "C", "WRONG_ANSWER", day), sub(4, 2, "A", "OK", day.AddDate(0, 0, 1))},
		Now:         day, Loc: ist,
	})
	if st.Days["2026-10-01"] != 2 || st.Days["2026-10-02"] != 1 || len(st.Days) != 2 {
		t.Fatalf("Days = %v", st.Days)
	}
}
