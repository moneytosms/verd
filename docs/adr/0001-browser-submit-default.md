# Submit defaults to browser handoff, not password login

Codeforces has no submit API and Cloudflare blocks password login from non-browser clients (403 on `/enter`, even with Chrome TLS impersonation). So the default submit copies the Solution to the clipboard and opens the submit page in the user's browser, and verd tracks the Verdict through the public `user.status` API. Direct submission by replaying user-pasted browser cookies over a Chrome-TLS client is opt-in and experimental, and it falls back to browser handoff on any failure.

**Considered Options:** password login (blocked), headless browser (heavy, fragile, Turnstile), captcha solving (rejected outright), browser cookie auto-import (unreliable since Chrome 127 app-bound encryption).
