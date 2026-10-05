# Submitting

`s` in the Problem view (or `verd submit <file>`) sends the Solution to Codeforces and tracks the result. There are two modes, set by `submit_mode`.

| | `browser` (default) | `direct` (experimental) |
| --- | --- | --- |
| How | Copies the Solution, opens the submit page, you paste and press Submit. | verd posts the form itself. |
| Credentials | None. | A saved browser session (`verd login`). |
| Risk | None. | Account risk, see below. |
| Verdict tracking | Yes | Yes |

Verdicts are tracked the same way in both modes: verd polls your public `user.status` for a Submission newer than the one it saw before you submitted, and shows `Testing on test 7`, `Accepted`, `Wrong answer on test 2`, and so on. Each update is saved, so the Problem is marked solved without a refresh. In the TUI the status opens in a Submission modal (`q` closes it, tracking continues) and ends with a green `✓ Accepted` or a red `✗ Wrong answer on test N`, plus time and memory. Tracking stops at a final Verdict or after 5 minutes.

## Browser mode

1. The Solution is copied with the first tool found: `pbcopy` (macOS), `clip.exe` (WSL), `wl-copy` (Wayland), `xclip` or `xsel` (X11). An OSC 52 sequence is also sent for terminals that support it, which works over ssh and tmux.
2. The submit page opens with `open`, `wslview` / `explorer.exe`, or `xdg-open`.
3. Paste, choose the language, submit.

Browser mode cannot preselect the language: the Codeforces submit page takes no language in its URL, so you pick it yourself (Codeforces remembers your last choice). To have verd choose the compiler for you, use direct mode, which sends the language's `cf_compiler_id` (see [Configuration](./config.md#languages)).

If no clipboard or opener is found, verd says so and prints the URL.

Why this is the default: Codeforces has no submit API, and Cloudflare blocks password logins from non-browser clients. See [ADR 0001](./adr/0001-browser-submit-default.md).

## Direct mode

> **Experimental. Read this first.**
>
> - Direct mode replays the cookies of a real browser session. Codeforces can treat that as automation. A flagged account can be restricted or banned from contests. verd cannot protect you from that.
> - The saved Cookie header is a login for your account. verd stores it in the OS keyring when one works. Otherwise it writes `~/.config/verd/credentials.json` with mode `0600` and warns you. Anyone who can read it can act as you until the session ends.
> - It can stop working at any time: Cloudflare or Codeforces may change their checks, and verd falls back to the browser when they do.
> - Codeforces does not endorse this. Use it on your own account, sparingly, and not in rated contests unless you accept the risk.

What it does: it fetches the submit page with a Chrome TLS fingerprint, reads the form and compiler list, posts your Solution to `/problemset/submit`, and tracks the Verdict as above. The `ftaa` / `bfaa` fields the page normally fills with JavaScript are sent as placeholders; Codeforces accepted that in testing, but it is a thing that could change.

### Set it up

1. Log in to Codeforces in **Chrome** (the TLS fingerprint imitates Chrome, and the User-Agent must match the browser the cookie came from).
2. Open <https://codeforces.com/problemset/submit>.
3. Open DevTools (`F12`), go to **Network**, and reload the page.
4. Click the first request, `submit` (type `document`).
5. Under **Request Headers**, right-click the `cookie:` value and choose **Copy value**.
6. Run `verd login`. Paste the cookie when asked (input is hidden), then paste the `user-agent:` value from the same list. Nothing is echoed.
7. Set the mode in `~/.config/verd/config.toml`:

   ```toml
   submit_mode = "direct"
   ```

8. Submit as usual with `s` or `verd submit <file>`.

You can also pipe both values: `printf '%s\n%s\n' "$COOKIE" "$UA" | verd login`.

Terminals limit a pasted line to about 4000 characters. If the paste is cut off, delete cookies you do not need from the header and try again.

`verd logout` deletes the saved credentials from the keyring and from the file.

### When it falls back

Any failure sends you to browser mode for that Submission and tells you why in the status notes. The common reasons:

| Reason shown | What to do |
| --- | --- |
| `not logged in: run verd login` | Run `verd login`. |
| `session expired? run verd login` | The cookie no longer works. Log in again in Chrome and repeat setup. |
| `Codeforces blocked the request (HTTP 403)` | Cloudflare rejected the session or fingerprint. Use browser mode. |
| `cf_compiler_id N is not offered by Codeforces; available: ...` | Set `cf_compiler_id` for that language to one of the listed ids. See [Configuration](./config.md#languages). |
| `Codeforces did not accept the Solution: ...` | Codeforces gave a reason, for example submitting identical code twice. |

Direct mode needs network; offline, `s` refuses before doing anything.
