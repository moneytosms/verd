# Submitting

`s` in the Problem view, or `verd submit main.cpp`, sends your solution to Codeforces and follows the result.

## Browser mode (default)

Nothing to set up and no credentials.

1. verd copies the solution to the clipboard (`pbcopy`, `clip.exe`, `wl-copy`, `xclip` or `xsel`, plus an OSC 52 escape that works over ssh and tmux).
2. It opens the problem's submit page in your browser.
3. You paste, pick the language and press Submit.
4. verd watches your submissions and shows `Testing on test 7`, then `Accepted` or `Wrong answer on test 2`, with time and memory. The status opens in a modal (`q` closes it, tracking goes on) and the problem is marked solved without a refresh.

```sh
verd submit ~/verd/1900/A/main.cpp   # copies, opens the page, then prints each status
echo $?                              # 0 only if the submission was Accepted
```

Tracking stops at a final verdict or after five minutes. Submitting needs a connection; offline verd says so instead of guessing.

## Direct mode (experimental)

verd posts the form itself and picks the compiler for you. It replays a real browser session, so Codeforces can treat it as automation and a flagged account can be restricted. Read the risks first in [Submitting](../submit.md#direct-mode), then:

```sh
verd login                       # paste the Cookie header and User-Agent (hidden)
verd config set submit_mode direct
```

Any failure falls back to browser mode for that submission and says why. `verd logout` deletes the saved session.

## CSES

CSES has no API, so submitting always opens the CSES page and you mark the task solved yourself with `m`, or pull your solved tasks in with `verd sync cses`. See [Finding problems](./problems.md#other-sources).

Next: [Contests and sessions](./contests.md).
