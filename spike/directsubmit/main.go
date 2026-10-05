// Throwaway spike for issue #40: can verd POST a Codeforces Submission from Go using a
// user-pasted Cookie header and User-Agent over a Chrome TLS fingerprint?
//
// Credentials come only from env (CF_COOKIE, CF_UA) and are never printed or written.
// Own module so the root go.mod and CI stay free of tls-client.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const base = "https://codeforces.com"

var (
	contest = flag.String("contest", "4", "contest id")
	index   = flag.String("problem", "A", "problem index")
	lang    = flag.String("lang", "Python 3", "substring of the compiler option name")
	srcFile = flag.String("src", "", "solution file to submit (required with -post)")
	handle  = flag.String("handle", "", "your handle, to poll the Verdict")
	post    = flag.Bool("post", false, "actually POST the Submission (default: GET the page only)")
	fixture = flag.String("fixture", "", "write the scrubbed submit-page HTML here")
)

var (
	inputRe  = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	attrRe   = regexp.MustCompile(`(?is)\b(name|value|type)\s*=\s*("([^"]*)"|'([^']*)')`)
	optionRe = regexp.MustCompile(`(?is)<option\b[^>]*value=["'](\d+)["'][^>]*>\s*([^<]*?)\s*</option>`)
	scrubRe  = regexp.MustCompile(`(?is)(name=["'](?:csrf_token|ftaa|bfaa|_tta)["'][^>]*value=["'])[^"']*`)
	scrubRe2 = regexp.MustCompile(`(?is)(value=["'])[^"']*(["'][^>]*name=["'](?:csrf_token|ftaa|bfaa|_tta)["'])`)
	metaCSRF = regexp.MustCompile(`(?is)(<meta\s+name=["']X-Csrf-Token["']\s+content=["'])[^"']*`)
	cfPage   = regexp.MustCompile(`(?i)just a moment|cf-chl|challenges\.cloudflare\.com|turnstile`)
)

func main() {
	flag.Parse()
	cookie, ua := os.Getenv("CF_COOKIE"), os.Getenv("CF_UA")
	if cookie == "" || ua == "" {
		die("CF_COOKIE and CF_UA must be set")
	}
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithNotFollowRedirects(),
	)
	if err != nil {
		die("client: %v", err)
	}
	do := func(method, u string, body string, extra map[string]string) (*http.Response, string) {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, u, rd)
		h := http.Header{
			"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
			"accept-language": {"en-US,en;q=0.9"},
			"cookie":          {cookie},
			"user-agent":      {ua},
		}
		order := []string{"accept", "accept-language", "cookie", "user-agent"}
		for k, v := range extra {
			h[k] = []string{v}
			order = append(order, k)
		}
		h[http.HeaderOrderKey] = order
		req.Header = h
		resp, err := client.Do(req)
		if err != nil {
			die("%s %s: %v", method, u, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	// 1. GET the submit page.
	resp, page := do("GET", base+"/problemset/submit", "", nil)
	fmt.Printf("GET /problemset/submit -> %d, %d bytes, content-type %q\n", resp.StatusCode, len(page), resp.Header.Get("content-type"))
	switch {
	case resp.StatusCode == 403 || resp.StatusCode == 503:
		fmt.Println("OUTCOME: BLOCKED (403/503 on GET). Direct mode not viable with these cookies.")
		return
	case resp.StatusCode != 200:
		fmt.Printf("OUTCOME: UNEXPECTED GET status %d, Location %q\n", resp.StatusCode, resp.Header.Get("location"))
		return
	}
	if *fixture != "" {
		scrubbed := metaCSRF.ReplaceAllString(scrubRe2.ReplaceAllString(scrubRe.ReplaceAllString(page, "${1}REDACTED"), "${1}REDACTED${2}"), "${1}REDACTED")
		if err := os.WriteFile(*fixture, []byte(scrubbed), 0o644); err != nil {
			die("fixture: %v", err)
		}
		fmt.Printf("fixture written: %s (csrf/ftaa/bfaa scrubbed)\n", *fixture)
	}

	// 2. Scrape the form.
	form := url.Values{}
	var csrf string
	for _, tag := range inputRe.FindAllString(page, -1) {
		var name, val string
		for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
			v := m[3]
			if v == "" {
				v = m[4]
			}
			switch strings.ToLower(m[1]) {
			case "name":
				name = v
			case "value":
				val = v
			}
		}
		if name == "" || name == "sourceFile" {
			continue
		}
		form.Set(name, val)
		if name == "csrf_token" {
			csrf = val
		}
	}
	if m := cfPage.FindString(page); m != "" {
		fmt.Printf("note: body mentions %q (a challenge page has no form; a real page may embed Turnstile)\n", m)
	}
	fmt.Printf("hidden/input fields found: %s\n", fieldNames(form))
	if csrf == "" {
		if cfPage.MatchString(page) {
			fmt.Println("OUTCOME: BLOCKED (200 but Cloudflare challenge page, no form).")
		} else {
			fmt.Println("OUTCOME: NOT LOGGED IN or page layout changed (no csrf_token). Check the Cookie header.")
		}
		return
	}
	var typeID, typeName string
	var names []string
	for _, m := range optionRe.FindAllStringSubmatch(page, -1) {
		names = append(names, m[1]+"="+strings.TrimSpace(m[2]))
		if typeID == "" && strings.Contains(strings.ToLower(m[2]), strings.ToLower(*lang)) {
			typeID, typeName = m[1], strings.TrimSpace(m[2])
		}
	}
	fmt.Printf("compiler options: %d found; chosen for %q: %s (%s)\n", len(names), *lang, typeID, typeName)
	if typeID == "" {
		fmt.Println("OUTCOME: no compiler option matched -lang; available:", strings.Join(names, " | "))
		return
	}
	if !*post {
		fmt.Println("OUTCOME: GET OK, form parsed. Re-run with -post to test the POST.")
		return
	}

	// 3. POST. Field names follow the live page; ftaa/bfaa fall back to dummies if JS would fill them.
	src, err := os.ReadFile(*srcFile)
	if err != nil || *handle == "" {
		die("-post needs -src and -handle")
	}
	form.Set("action", "submitSolutionFormSubmitted")
	form.Set("contestId", *contest)
	form.Set("submittedProblemIndex", *index)
	form.Set("programTypeId", typeID)
	form.Set("source", string(src))
	form.Set("tabSize", "4")
	if form.Get("ftaa") == "" {
		form.Set("ftaa", "0123456789abcdef01")
	}
	if form.Get("bfaa") == "" {
		form.Set("bfaa", "f1b3f18c715565b589b7823cda7448ce")
	}
	before := lastSubmissionID(do, *handle)
	resp, out := do("POST", base+"/problemset/submit?csrf_token="+csrf, form.Encode(), map[string]string{
		"content-type": "application/x-www-form-urlencoded",
		"origin":       base,
		"referer":      base + "/problemset/submit",
	})
	loc := resp.Header.Get("location")
	fmt.Printf("POST -> %d, Location %q, %d bytes\n", resp.StatusCode, loc, len(out))
	switch {
	case resp.StatusCode == 403 || resp.StatusCode == 503:
		fmt.Println("OUTCOME: BLOCKED on POST (403 or Cloudflare/Turnstile).")
		return
	case resp.StatusCode != 302 && resp.StatusCode != 303:
		fmt.Println("OUTCOME: POST not accepted (no redirect). Page text hints:", hints(out))
		return
	}

	// 4. Poll the public API for the Verdict (no cookies needed).
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		id, verdict := lastSubmission(do, *handle)
		if id != 0 && id != before {
			fmt.Printf("  submission %d verdict %q\n", id, verdict)
			if verdict != "" && verdict != "TESTING" {
				fmt.Println("OUTCOME: WORKS. Submission posted and Verdict observed.")
				return
			}
		}
	}
	fmt.Println("OUTCOME: POST redirected but no new Submission/Verdict seen within 60 s (maybe duplicate code or queue).")
}

type doFn func(method, u string, body string, extra map[string]string) (*http.Response, string)

func lastSubmission(do doFn, handle string) (id int64, verdict string) {
	_, b := do("GET", base+"/api/user.status?handle="+url.QueryEscape(handle)+"&from=1&count=1", "", nil)
	var r struct {
		Result []struct {
			ID      int64  `json:"id"`
			Verdict string `json:"verdict"`
		} `json:"result"`
	}
	if json.Unmarshal([]byte(b), &r) != nil || len(r.Result) == 0 {
		return 0, ""
	}
	return r.Result[0].ID, r.Result[0].Verdict
}

func lastSubmissionID(do doFn, handle string) int64 { id, _ := lastSubmission(do, handle); return id }

func fieldNames(v url.Values) string {
	var ns []string
	for k := range v {
		ns = append(ns, k)
	}
	return strings.Join(ns, ", ")
}

var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

func hints(body string) string {
	for _, m := range regexp.MustCompile(`(?is)class="error[^"]*"[^>]*>(.*?)</`).FindAllStringSubmatch(body, 3) {
		return strings.TrimSpace(tagRe.ReplaceAllString(m[1], ""))
	}
	return "(none found)"
}

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(2)
}
