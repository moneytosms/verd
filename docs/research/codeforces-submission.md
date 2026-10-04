# Can verd submit to Codeforces from Go?

Issue: #2 (part of #1). Researched 2026-10-05. Probes were run from a Linux dev box (residential-ish IP, no browser cookies).

## Recommendation

1. **Ship the fallback first (v1):** copy source to clipboard, open the contest submit URL in the browser, then poll verdicts via the public API. Zero Cloudflare exposure, zero credentials, works today.
2. **Verdict polling: use `GET /api/user.status`** (no auth, plain Go `net/http` works). Throttle to 1 req / 2 s. Needs only the user's handle.
3. **Direct submit (v2, opt-in, "experimental"):** user logs in in their real browser, pastes a "Copy as cURL" / Cookie header (incl. `cf_clearance`, `JSESSIONID`, `39ce7`, and the same User-Agent). verd replays it with a Chrome-impersonating TLS client (`bogdanfinn/tls-client` or `refraction-networking/utls`). Do NOT attempt password login from Go.
4. **Do not** build headless-browser login (chromedp/rod) or captcha solving. Fragile, heavy, and adversarial to Cloudflare.
5. Direct submit has a hard unresolved blocker: the submit form now carries a Turnstile token (see below). Prototype it before committing to v2; if it fails, v2 is dead and the fallback is the product.

## Findings

### 1. API cannot submit
The official API (`/apiHelp`) exposes read methods only; "submit" does not appear on the page. Rate limit: "at most 1 time per two seconds", otherwise `FAILED` / `Call limit exceeded`. [apiHelp](https://codeforces.com/apiHelp) (fetched via curl_cffi; WebFetch and curl get 403).

### 2. Cloudflare: plain Go clients are blocked (verified)
- `curl https://codeforces.com/enter` -> `403`, header `cf-mitigated: challenge`, `server: cloudflare`, CSP allowing `challenges.cloudflare.com`. Same for `/problemset` and `/contest/1/submit`. Adding a Chrome User-Agent + Accept headers did not help (still 403). Go's `net/http` has a distinct TLS ClientHello (JA3/JA4) from curl but both are non-browser; expect the same. Not tested with Go directly.
- Python `curl_cffi` with `impersonate="chrome"` (browser TLS+HTTP2 fingerprint, no cookies): `/problemset` 200, `/contest/1/submit` 200 (redirected to `/` since logged out), `/api/user.status` 200, but **`/enter` still 403 challenge** (also with `chrome120`, 3 tries). So TLS impersonation clears the general bot gate but the login page has a stricter interactive challenge. Login from a non-browser client is not viable.
- The API (`/api/*`) answered 200 to plain `curl` with no cookies. Fine for polling; a Cloudflare rule change could break it (xalanq/cf-tool issue #176 reports API failing for "some days", since fixed).
- Real-world breakage: xalanq/cf-tool is archived (last push 2024-07-30) and open issues report `Cannot find csrf` / "CF ADDED BOT PROTECTION": [#176](https://github.com/xalanq/cf-tool/issues/176), [#177](https://github.com/xalanq/cf-tool/issues/177), [#179](https://github.com/xalanq/cf-tool/issues/179). Its login (GET /enter, regex `csrf='...'`, POST with random ftaa and constant bfaa, `_tta`) gets the challenge page instead: [client/login.go](https://github.com/xalanq/cf-tool/blob/master/client/login.go).

### 3. Login / submit form fields
From xalanq/cf-tool (old, pre-Cloudflare), [client/login.go](https://github.com/xalanq/cf-tool/blob/master/client/login.go) and [client/submit.go](https://github.com/xalanq/cf-tool/blob/master/client/submit.go):
- Login POST `/enter`: `csrf_token, action=enter, ftaa, bfaa, handleOrEmail, password, _tta, remember=on`.
- Submit POST `/contest/{id}/submit?csrf_token=...`: `csrf_token, ftaa, bfaa, action=submitSolutionFormSubmitted, submittedProblemIndex, programTypeId, contestId, source, tabSize, _tta, sourceCodeConfirmed=true`.

Current behaviour from a 2026 tool, [anpaure/cf-tools-plus](https://github.com/anpaure/cf-tools-plus) (`cf_tools_plus.py`, pushed 2026-08, macOS-only, Python+curl_cffi):
- csrf now read from `<meta name="X-Csrf-Token" content="...">`.
- `ftaa`/`bfaa` are **scraped from the submit page** (`window._ftaa = "..."`, `window._bfaa = "..."`), not generated.
- Payload: `csrf_token, action=submitSolutionFormSubmitted, submittedProblemIndex, programTypeId, source, tabSize, sourceFile="", ftaa, bfaa, turnstileToken=""`, with `Origin`/`Referer` headers.
- It sends an **empty `turnstileToken`** and reports working, so server-side enforcement of Turnstile on submit may be lenient/session-risk-based. I could not verify this myself (needs a logged-in account). njlane314/cfx's browser extension, by contrast, waits up to 60 s for `[name="cf-turnstile-response"]` to be filled, i.e. the form can require it ([connector.js](https://github.com/njlane314/cfx/blob/main/src/browser/connector.js)). Treat as the main risk.
- Duplicate source -> page text "You have submitted exactly the same code before"; success detected by leaving `/submit` URL; the tool snapshots submission ids before POST and diffs afterwards to find the new one.

### 4. Auth/session reuse, how the survivors do it
| Tool | Approach | Evidence |
|---|---|---|
| cf-tool_modify (fork, active 2026-10) | user logs in in browser, pastes Cookie header incl. `cf_clearance`; injects into Go cookie jar; custom UA transport. Notes cf_clearance expires in "a few days" | [CLOUDFLARE_FIX.md](https://github.com/programboys/cf-tool_modify/blob/master/CLOUDFLARE_FIX.md) |
| cf-tools-plus | "Copy as cURL"/HAR/clipboard -> keeps only `cookie, user-agent, sec-ch-ua*` headers in macOS Keychain; replays with `curl_cffi` Chrome impersonation | README + source above |
| cfx (C++) | Chrome MV3 extension in the user's signed-in browser fills and submits the real form; loopback token protocol; no cookies stored. Falls back to clipboard + open page (`cfx submit --manual`) | [README](https://github.com/njlane314/cfx) |

Both cookie-replay tools must pair the cookies with the **same User-Agent**; Cloudflare says `cf_clearance` is "securely tied to the specific visitor and device" and re-evaluated continuously; its docs do not state IP/UA/TLS binding or a default TTL ([docs](https://developers.cloudflare.com/cloudflare-challenges/concepts/clearance/)). Expect periodic re-import.

### 5. Go libraries
| Need | Library | Status (GitHub API, 2026-10) |
|---|---|---|
| Chrome TLS/H2 fingerprint | [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) | pushed 2026-09, ~1.9k stars, wraps utls, ready-made Chrome profiles |
| Lower level | [refraction-networking/utls](https://github.com/refraction-networking/utls) | pushed 2026-09, BSD-3, ~2.6k stars; you hand-roll H2 + profile |
| Alternatives | Noooste/azuretls-client, Danny-Dasilva/CycleTLS | active-ish; not evaluated |
| Read cookies from browser | [browserutils/kooky](https://github.com/browserutils/kooky) | MIT, last push 2026-06; open [#92](https://github.com/browserutils/kooky/issues/92): Chrome 127+ Windows app-bound encryption (v20) cookies cannot be decrypted. Auto-import is therefore unreliable on Windows Chrome; Firefox/Linux OK in principle (not tested) |
| Headless | chromedp (13k stars), go-rod (7k) | active; would still need the user to solve the challenge once. Not recommended |

Caveat: whether Go utls/tls-client matches what `curl_cffi` achieved was not tested here (curl_cffi result is the proxy evidence).

### 6. Captcha
Login (`/enter`) is gated by a Cloudflare interactive challenge (above). Submit form may carry Turnstile (cfx). No sane way to automate; users do it in the browser.

### 7. Compiler ids (`programTypeId`)
- Ids are server-side and change as Codeforces adds compilers; old lists are stale (cf-tool's table still says Python 3.7.2 = 31, PyPy 3.6 = 41, G++17 7.3.0 = 54, GCC C11 5.1.0 = 43).
- Confirmed current via 2026 sources: **89 = "GNU G++20 13.2 (64 bit, winlibs)"** (cfx test fixture, cf-tool_modify list). `user.status` showed contemporary submissions as "C++23 (GCC 14-64, msys2)".
- **Not verified:** current ids for C++23, Python 3, PyPy 3. (Could not read a live submit form without login.) Do not hardcode. Parse `<select name="programTypeId"><option value="N">label` from the submit page like cf-tools-plus' `cf compilers` (regex `<option\s+value="(\d+)"[^>]*>(.*?)</option>`). For the clipboard-only v1 no id is needed.

### 8. Verdict polling
- `GET https://codeforces.com/api/user.status?handle=H&from=1&count=N`: verified 200, no auth, no cookies. Fields seen: `id, contestId, creationTimeSeconds, problem{contestId,index,name,rating,tags}, programmingLanguage, verdict ("OK", ...), testset, passedTestCount, timeConsumedMillis, memoryConsumedBytes`. Pending submissions have no `verdict` / `TESTING` (from API docs knowledge; not observed). Match the new submission by id diffing before/after (cf-tools-plus does this) or by problem + `creationTimeSeconds >= submit time`.
- Poll every >= 2 s (documented limit); stop on a final verdict. Page scraping is unnecessary and Cloudflare-exposed.
- Limitation: `user.status` does not cover private group / some gym contests visibility (cf-tools-plus needs scraping for groups). Out of scope for v1.

### 9. Fallback viability (clipboard + open submit URL)
URL: `https://codeforces.com/contest/{id}/submit?submittedProblemIndex={X}` (query param prefill not verified; plain `/contest/{id}/submit` is safe, and `/problemset/submit` for problemset).
| OS | Open | Copy |
|---|---|---|
| Linux Wayland | `xdg-open` (present on this box) | `wl-copy` (present) |
| Linux X11 | `xdg-open` | `xclip -selection clipboard` or `xsel -b` (xclip absent on this box) |
| macOS | `open` | `pbcopy` |
| WSL2 | `wslview` (wslu) or `explorer.exe URL`, `cmd.exe /c start` | `clip.exe` (UTF-8 caveats: pipe through `iconv`/PowerShell `Set-Clipboard`) |
| Windows | `rundll32 url.dll,FileProtocolHandler` / `cmd /c start` | `clip.exe` |
Only `xdg-open` and `wl-copy` were confirmed present locally; the macOS/WSL/Windows commands were not run. Pure-Go option: `atotto/clipboard` (pushed 2026-10, shells out to those same tools); or OSC 52 for SSH. Detect order: `$WSL_DISTRO_NAME` -> WSL tools, `$WAYLAND_DISPLAY` -> wl-copy, else xclip/xsel, and print a "copied manually / install xclip" hint on failure. Also print the URL so the user can click it.

## Not verified
- Direct submit end-to-end (no Codeforces account used; no POST made).
- Whether Go utls/tls-client passes the same gates as `curl_cffi`.
- Whether an empty `turnstileToken` is accepted today.
- `cf_clearance` lifetime and IP/UA binding.
- Current compiler ids besides 89.
- Pending-verdict shape of `user.status`; `?submittedProblemIndex=` prefill.
- Non-Linux clipboard/open commands.

## Next step
Prototype in a scratch Go program: tls-client (Chrome profile) + pasted Cookie/UA, GET `/contest/{id}/submit`, check csrf/`_ftaa`/`_bfaa`/Turnstile presence. Then decide v2. Meanwhile build v1 (clipboard + open + `user.status` poller).
