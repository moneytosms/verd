package cses

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

// A fake cses.fi that follows the real flow: csrf form, PHPSESSID, then the task list with
// full (solved) and zero (tried) icons only for a logged-in cookie.
func fakeCSES() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		form := `<form method="post"><input type="hidden" name="csrf_token" value="tok"><input name="nick"><input name="pass"></form>`
		if r.Method == "GET" {
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "anon"})
			w.Write([]byte(form))
			return
		}
		r.ParseForm()
		if r.Form.Get("csrf_token") != "tok" || r.Form.Get("nick") != "me" || r.Form.Get("pass") != "pw" {
			w.Write([]byte(form))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "authed"})
		w.Write([]byte(`<a class="account" href="/user/1">me</a>`))
	})
	mux.HandleFunc("/problemset/", func(w http.ResponseWriter, r *http.Request) {
		if ck, err := r.Cookie("PHPSESSID"); err != nil || ck.Value != "authed" {
			w.Write([]byte(`<a class="account" href="/login">Login</a><ul class="task-list"><li class="task"><a href="/problemset/task/1068">A</a></ul>`))
			return
		}
		w.Write([]byte(`<a class="account" href="/user/1">me</a><ul class="task-list">
<li class="task"><a href="/problemset/task/1068">A</a><span class="task-score icon full"></span>
<li class="task"><a href="/problemset/task/1069">B</a><span class="task-score icon zero"></span>
<li class="task"><a href="/problemset/task/1070">C</a><span class="task-score icon "></span>
<li class="task"><a href="/problemset/task/1083">D</a><span class="task-score icon full"></span></ul>`))
	})
	return httptest.NewServer(mux)
}

func TestLoginAndSolved(t *testing.T) {
	srv := fakeCSES()
	defer srv.Close()
	c := &Client{Base: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}}
	if _, err := c.Login(context.Background(), "me", "wrong"); err == nil {
		t.Fatal("a wrong password must fail")
	}
	cookie, err := c.Login(context.Background(), "me", "pw")
	if err != nil || cookie != "PHPSESSID=authed" {
		t.Fatalf("%q %v", cookie, err)
	}
	ids, err := c.Solved(context.Background(), cookie)
	if err != nil || !slices.Equal(ids, []int{1068, 1083}) {
		t.Fatalf("only full icons are solved: %v %v", ids, err)
	}
	if _, err := c.Solved(context.Background(), "PHPSESSID=anon"); err != ErrNotLoggedIn {
		t.Fatalf("expired session: %v", err)
	}
}
