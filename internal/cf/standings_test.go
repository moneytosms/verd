package cf

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const standingsBody = `{"status":"OK","result":{"contest":{"id":7,"name":"R","type":"CF","phase":"CODING","durationSeconds":7200,"startTimeSeconds":100},
"problems":[{"contestId":7,"index":"A","name":"a","points":500.0,"tags":[]},{"contestId":7,"index":"B","name":"b","points":1000.0}],
"rows":[
{"party":{"members":[{"handle":"other"}]},"rank":1,"points":1500.0,"penalty":0,"problemResults":[{"points":490.0,"rejectedAttemptCount":0,"type":"FINAL","bestSubmissionTimeSeconds":60},{"points":900.0,"rejectedAttemptCount":2,"type":"PRELIMINARY","bestSubmissionTimeSeconds":300}]},
{"party":{"members":[{"handle":"Me"}]},"rank":2,"points":490.0,"penalty":0,"problemResults":[{"points":490.0,"rejectedAttemptCount":1,"type":"PRELIMINARY","bestSubmissionTimeSeconds":120},{"points":0.0,"rejectedAttemptCount":0,"type":"FINAL"}]}
]}}`

func standingsClient(t *testing.T, body string) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/contest.standings" || r.URL.Query().Get("contestId") != "7" || len(r.URL.Query()) != 1 {
			t.Errorf("non-admin standings take only contestId, got %s", r.URL.RawQuery)
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL)
	c.sleep = func(time.Duration) {}
	return c
}

func TestStandingsKeepsOnlyMyRow(t *testing.T) {
	st, err := standingsClient(t, standingsBody).Standings(context.Background(), 7, "me")
	if err != nil {
		t.Fatal(err)
	}
	if st.Contest.Type != "CF" || st.Contest.Phase != "CODING" || len(st.Problems) != 2 || st.Problems[1].Points != 1000 {
		t.Fatalf("%+v", st)
	}
	r := st.Row
	if r == nil || r.Rank != 2 || r.Points != 490 || !r.Results[0].Solved() || r.Results[0].Rejected != 1 || r.Results[1].Solved() {
		t.Fatalf("row: %+v", r)
	}
	if st.Bytes != int64(len(standingsBody)) {
		t.Fatalf("bytes %d", st.Bytes)
	}
	if st, err := standingsClient(t, standingsBody).Standings(context.Background(), 7, "nobody"); err != nil || st.Row != nil {
		t.Fatalf("unregistered handle has no row: %+v %v", st, err)
	}
}

func TestStandingsProblemsOnlyStopsEarly(t *testing.T) {
	// everything after "rows": is garbage: an empty handle must not read it
	broken := `{"status":"OK","result":{"contest":{"id":7},"problems":[{"contestId":7,"index":"A","name":"a"}],"rows":[<<<not json`
	ps, err := standingsClient(t, broken).ContestProblems(context.Background(), 7)
	if err != nil || len(ps) != 1 || ps[0].Index != "A" {
		t.Fatalf("%v %v", ps, err)
	}
}

func TestStandingsFailureIsAPIError(t *testing.T) {
	_, err := standingsClient(t, `{"status":"FAILED","comment":"contestId: Contest has not started"}`).Standings(context.Background(), 7, "me")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Comment != "contestId: Contest has not started" {
		t.Fatalf("%v", err)
	}
}

func TestStandingsKeepsNeighbours(t *testing.T) {
	row := func(h string, rank int) string {
		return fmt.Sprintf(`{"party":{"members":[{"handle":%q}]},"rank":%d,"points":0,"penalty":0,"problemResults":[]}`, h, rank)
	}
	var rows []string
	for i := 1; i <= 10; i++ {
		h := fmt.Sprint("u", i)
		if i == 6 {
			h = "me"
		}
		rows = append(rows, row(h, i))
	}
	body := `{"status":"OK","result":{"contest":{"id":7},"problems":[],"rows":[` + strings.Join(rows, ",") + `]}}`
	st, err := standingsClient(t, body).Standings(context.Background(), 7, "me")
	if err != nil {
		t.Fatal(err)
	}
	ranks := func(rs []StandingRow) (out []int) {
		for _, r := range rs {
			out = append(out, r.Rank)
		}
		return
	}
	if fmt.Sprint(ranks(st.Top)) != "[1 2 3]" || fmt.Sprint(ranks(st.Near)) != "[4 5 6 7 8]" || st.Total != 10 || st.Row.Handle != "me" {
		t.Fatalf("top %v near %v total %d row %+v", ranks(st.Top), ranks(st.Near), st.Total, st.Row)
	}
}
