# Codeforces API coverage for v1

Issue: #3 (part of #1). Verified 2026-10-05 with live requests (curl, ~1 req/2s) unless marked UNVERIFIED.

## Summary

The official API covers all *metadata* verd v1 needs (problem list, tags, rating, solved counts, contests, submissions, rating history, user info). It does **not** serve statements, samples, time/memory limits, or an interactive flag. Those need HTML scraping of problem pages, which works with a browser-like User-Agent but is blocked by Cloudflare for the default curl UA.

## Endpoints (all anonymous GET, `https://codeforces.com/api/{method}`)

Envelope: `{status: "OK"|"FAILED", comment?, result?}` ([apiHelp](https://codeforces.com/apiHelp)).

| Need | Method | Verified shape / notes |
|---|---|---|
| Problems, tags, rating | `problemset.problems[?tags=a;b]` | One call returns everything: 11425 problems, ~2.2 MB. `result.problems[]`: `contestId, index, name, type, points?, rating?, tags[]`. `result.problemStatistics[]`: `contestId, index, solvedCount`. Join on `(contestId,index)`. 273/11425 lack `rating`; 3937 lack `points`. Cache locally, refresh rarely. |
| Contest list | `contest.list[?gym=true]` | `id,name,type,phase,frozen,durationSeconds,startTimeSeconds,relativeTimeSeconds`. Phases include `BEFORE`, so upcoming contests come free. |
| Standings | `contest.standings?contestId=N` | See gotcha below. |
| Submissions | `user.status?handle=H&from=1&count=N` | Newest first. Fields: `id, contestId, creationTimeSeconds, problem{...}, author, programmingLanguage, verdict, passedTestCount, timeConsumedMillis, memoryConsumedBytes`. Paginate via `from`/`count`. `includeSources` only for own account (auth). |
| Rating history | `user.rating?handle=H` | `contestId, contestName, rank, ratingUpdateTimeSeconds, oldRating, newRating`. |
| User | `user.info?handles=a;b` | `handle, rating, maxRating, rank, maxRank, country, organization, friendOfCount, avatar, ...`. Up to 10000 handles; follows handle history by default. |

**Standings gotcha (verified):** for regular public contests, non-admin users may only call `contest.standings?contestId=N` with *no other params*. Adding `from/count/showUnofficial` returns `FAILED` ("available only via anonymous GET requests with no extra parameters"). The no-param call returns the full standings: 6.6 MB for contest 1900. Paging/filtering (`handles`, `from`, `count`, `showUnofficial`) is documented for gym/mashup and authenticated calls only ([method docs](https://codeforces.com/apiHelp/methods#contest.standings)). Implication: for "my result in contest X" prefer `user.status` filtered by `contestId`, or `contest.ratingChanges`; avoid standings except on demand.

## Rate limits

Documented: at most 1 request per 2 seconds; excess gets `FAILED` / "Call limit exceeded" ([apiHelp](https://codeforces.com/apiHelp)). Not hit during testing at 2s spacing. UNVERIFIED: behaviour on bursts, any IP-level blocking, limits on HTML pages (none documented). Client needs a global 2s throttle + retry on "Call limit exceeded".

## Auth

Not needed for any v1 method above (all anonymous, public data). Needed only for private data: own `includeSources`, gym/mashup standings, `contest.hacks` during a contest, `user.friends`, `contest.list` with private contests.

Signing, if added later ([apiHelp](https://codeforces.com/apiHelp)): key+secret generated at https://codeforces.com/settings/api. Add `apiKey`, `time` (unix s, server skew max 5 min), `apiSig = rand(6 chars) + hex(sha512("<rand>/<method>?<params sorted by name then value, incl. apiKey,time, excl. apiSig>#<secret>"))`. Taken from docs only; not tested end-to-end (UNVERIFIED, no key available).

## What the API lacks

Statement text, input/output spec, samples, notes, time limit, memory limit, input/output file, interactive flag, per-problem contest name. All must come from problem page HTML. (Verified: `problemset.problems` objects have none of these.)

## Problem page scraping

URLs: `https://codeforces.com/contest/{cid}/problem/{idx}` and `/problemset/problem/{cid}/{idx}` (both 200, ~65 KB). Gym/group/edu/acmsguru variants exist ([competitive-companion patterns](https://github.com/jmerle/competitive-companion/blob/master/src/parsers/problem/CodeforcesProblemParser.ts)); recommend v1 supports contest/problemset only.

**Cloudflare (verified):** `curl` with default UA -> HTTP 403 "Just a moment..." challenge page (5 KB). Same URL with `User-Agent: Mozilla/5.0 ...` -> 200 full HTML. The `/api/...` path worked with default UA. Fragile: Cloudflare can start challenging any UA/IP at any time. UNVERIFIED: stability over time, other networks/IPs, heavy volume, and Go's TLS fingerprint (only curl was tested). Mitigations: browser-like UA, polite rate (>=2s), cache aggressively (statements are effectively immutable), detect `<title>Just a moment...` and show an "open in browser" fallback, optionally accept a user-supplied `cf_clearance` cookie (UNVERIFIED).

**Structure (verified on 1900/A, 4/A, 1520/F1):**

- Root: `.problem-statement`.
  - `> .header > .title`, e.g. `A. Cover in Water`.
  - `> .header > .time-limit`, `.memory-limit`, `.input-file`, `.output-file`: each is `<div class="property-title">..</div>` followed by a bare text node ("1 second", "256 megabytes", "standard input" or "stdin" on old problems, or a filename). Take the last text node; competitive-companion does the same.
  - Legend: first plain `<div>` after header (no class).
  - `.input-specification`, `.output-specification`, optional `.note`, each with a `.section-title` child.
  - Interactive: a `.section-title` equal to `Interaction` (ru: `Протокол взаимодействия`) exists (verified on 1520/F1). Same heuristic as competitive-companion; no explicit flag exists.
- Samples: `.sample-tests .sample-test` containing `.input pre` and `.output pre` pairs (multiple pairs possible; pair by index).
  - Modern pages: each line is `<div class="test-example-line test-example-line-N">text</div>` inside `<pre>`; join lines with `\n`. competitive-companion treats a line containing `<br>` as empty.
  - Old pages (4/A): plain `<pre>8<br /></pre>`; convert `<br>` to `\n`.
  - Entities need HTML-decoding (goquery `.Text()` does this; `.Html()` does not). For the old format, replace `br` nodes with `\n` first.
- Math: TeX in `$$$...$$$` delimiters (inline and display both; present even on old problem 4/A). Convert to terminal-friendly text (strip delimiters, Unicode substitutions for `\le`, `\ge`, `\cdot`, etc.); a TUI cannot render TeX. Keep raw TeX as fallback.
- Inline style: `<span class="tex-font-style-tt">` (monospace), `-bf`, `-it`; lists as `<ul><li>`; tables occasionally.
- Images: `<img class="tex-graphics" src="https://espresso.codeforces.com/<hash>.png">` (verified in 1900/A note). Terminal cannot show them: replace with `[image: URL]` (Kitty/sixel later, out of scope).
- Tags also appear as `.tag-box[title]` on the problemset page; API already gives tags, use the API.
- PDF-statement problems exist (`embed[type="application/pdf"]` or body starts `%PDF`, per competitive-companion). Detect and show "statement is a PDF, open in browser". Not encountered in my samples (UNVERIFIED live).

**Stability:** structure is old and stable. competitive-companion (actively maintained) uses CSS selectors identical to the above; cf-tool (last push 2024-07-30) uses plain regexes for samples (`class="input"[\s\S]*?<pre>`, [parse.go](https://github.com/xalanq/cf-tool/blob/master/client/parse.go)). The line-div sample format is the newer wrinkle; handle both. UNVERIFIED: when line-divs were introduced; only 3 pages sampled. Recommend a golden-file test set (old format, line-div format, interactive, image, PDF, file I/O) in `testdata/`.

## Go libraries

- HTML: [`github.com/PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery) (latest release v1.13.0 per `gh api`), jQuery-style selectors over `golang.org/x/net/html`. Covers every selector above. colly (v2.2.0) is a crawler framework, overkill for fetch-one-page.
- HTTP: stdlib `net/http` with custom UA, plus `golang.org/x/time/rate` or a 2s ticker for the API.
- JSON: stdlib `encoding/json`. Signing (later): stdlib `crypto/sha512`.

## Recommendation

1. Use the official API for all metadata; v1 needs no auth. Single client with global 1 req / 2 s throttle and retry on "Call limit exceeded".
2. Fetch `problemset.problems` once, store in SQLite (join `problems` + `problemStatistics` on contestId+index), refresh on a TTL (e.g. daily) or manual sync. No per-problem API calls.
3. Do not use `contest.standings` for per-user data; use `user.status` + `contest.list` + `user.rating`. Standings only as explicit on-demand full download (6+ MB).
4. Scrape statement, samples, limits, interactive flag lazily on first open of a problem, cache in SQLite, using goquery with a browser-like UA and the selectors above.
5. Treat Cloudflare as the main risk: detect challenge page, fall back to "open in browser", consider user-provided cookie later. Prototype the Go fetch early to confirm Go's TLS client is not challenged (UNVERIFIED).
6. Render TeX as plain-text/Unicode approximation, images as `[image: URL]`.
7. Defer apiKey/apiSig to post-v1 (own source code, friends, gym).

## Sources

- API docs: https://codeforces.com/apiHelp , https://codeforces.com/apiHelp/methods
- Live endpoints: `/api/problemset.problems`, `/api/contest.list`, `/api/contest.standings?contestId=1900`, `/api/user.status?handle=tourist`, `/api/user.rating`, `/api/user.info`
- Live pages: https://codeforces.com/contest/1900/problem/A , https://codeforces.com/problemset/problem/4/A , https://codeforces.com/contest/1520/problem/F1
- competitive-companion parser: https://github.com/jmerle/competitive-companion/blob/master/src/parsers/problem/CodeforcesProblemParser.ts
- cf-tool parser: https://github.com/xalanq/cf-tool/blob/master/client/parse.go
- goquery: https://github.com/PuerkitoBio/goquery
