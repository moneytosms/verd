# Driving tmux and herdr panes from verd

Issue: #4 (part of #1). Researched 2026-10-05 against tmux 3.7c, herdr 0.9.3 (protocol 22), Neovim 0.12.5 on Linux.
"Verified" = run locally on throwaway servers (`tmux -L vt`, `herdr --session verdtest`). Everything else is doc-sourced or flagged unverified.

## TL;DR

- Detect: `$TMUX` + `$TMUX_PANE` => tmux. Else `$HERDR_ENV=1` + `$HERDR_PANE_ID` => herdr. Else none (fallback: no split, run nvim fullscreen or print a hint).
- Both can split, run a command, return the new pane id, close/kill it, and report pane exit.
- Neovim control goes over nvim's own RPC socket (`nvim --listen <sock>`), independent of the multiplexer. Same code for both.
- Biggest asymmetry: tmux can start nvim as the pane's process directly; herdr cannot (`pane split` has no command arg), so use `pane run <id> "exec nvim ..."`.
- herdr CLI cannot focus a pane by id (only by direction). The raw socket method `pane.focus` exists.

## Detection

| Mux | Signal | Source |
|---|---|---|
| tmux | `TMUX` (set for every process in a tmux pane), `TMUX_PANE` = pane id like `%3` | man tmux, "pane ID passed to the child process in TMUX_PANE" |
| herdr | `HERDR_ENV=1`, `HERDR_PANE_ID` (`w1:p2`), `HERDR_TAB_ID`, `HERDR_WORKSPACE_ID`, `HERDR_SOCKET_PATH`, `TERM_PROGRAM=herdr` | herdr cli-reference, socket-api |

Nesting rules:
- herdr strips inherited `TMUX`, screen, zellij, kitty, wezterm, iTerm2, WT markers from new panes (cli-reference). So herdr-inside-tmux shows only herdr vars. Correct: innermost wins.
- tmux-inside-herdr: pane inherits `HERDR_ENV` and sets `TMUX`. Both visible. Recommended rule: check tmux first (innermost). Reasoned from the two rules above, not tested.
- Do not rely on `TERM_PROGRAM` alone (tmux sets `TERM_PROGRAM=tmux` only in 3.2+; not verified here).
- herdr CLI uses `HERDR_SOCKET_PATH` / `HERDR_SESSION` automatically inside a pane. From outside, pass `--session <name>`; verd should refuse to control herdr when `HERDR_ENV` is unset (herdr's own skill file says the same).

## tmux

All verified on tmux 3.7c unless noted.

| Need | Command |
|---|---|
| Split right, don't steal focus, get id | `tmux split-window -h -d -P -F '#{pane_id}' -t "$TMUX_PANE" -c "$PWD" 'nvim --listen /path/sock /path/file'` prints `%1` |
| Run nvim as the pane process | pass `shell-command` to split-window (above). Pane closes when nvim exits |
| Focus | `tmux select-pane -t %1` |
| Close | `tmux kill-pane -t %1` |
| Is pane alive? | `tmux display-message -p -t %1 '#{pane_id}'` errors if gone; or `list-panes -a -F '#{pane_id}'` |
| Which pane am I | `$TMUX_PANE`, or `tmux display-message -p '#{pane_id}'` |
| Send text/keys | `tmux send-keys -t %1 -l 'text'` (literal) then `send-keys -t %1 Enter` |
| Read output | `tmux capture-pane -p -t %1` |
| Exit status | with `remain-on-exit on`: `#{pane_dead}`, `#{pane_dead_status}` (verified: shows 3 for `exit 3`) |

Detecting that nvim closed:
- Hook `pane-exited` ("Run when the program running in a pane exits") fires; verified: `set-hook -g pane-exited "run-shell 'echo #{hook_pane}'"` logged `%1`. With `remain-on-exit on` the `pane-died` hook is the one that applies instead, and `pane-exited` did not fire in my test.
- Hooks run inside the tmux server and need a side channel (`run-shell` writing to a FIFO/file, or `tmux wait-for -S channel`) to reach verd. Simpler: poll `display-message -p -t %1` every ~500ms from a Bubble Tea `tea.Tick`. Cheap, no hook installation, no global state.
- `tmux wait-for channel` can block a goroutine until `wait-for -S channel` is run by a hook. Not verified; polling is enough.

Quirks:
- Multiple tmux servers: pass `-S` from `$TMUX` (first comma field is the socket path; format from tmux source, not verified here) only if verd is run via `sudo` or other env mangling. Normally the inherited `$TMUX` suffices when calling `tmux` as a child process.
- `send-keys` without `-l` interprets key names like `Enter`, `Space`; user text must use `-l`.
- Quoting: pass shell-command as one string; tmux runs it via `default-shell -c`. In Go use `exec.Command("tmux", "split-window", ..., cmdString)`.
- Missing `-d` moves focus to the new pane, which is usually unwanted for verd (the user is typing in verd's TUI).
- Pane ids (`%N`) are stable for the server's life (man tmux).

## herdr

Verified on herdr 0.9.3 with a headless named session.

| Need | CLI | Notes |
|---|---|---|
| Split | `herdr pane split --current --direction right --cwd "$PWD" --no-focus` | JSON out; new id at `.result.pane.pane_id` (verified, e.g. `w1:p2`) |
| Run command | `herdr pane run <id> "exec nvim --listen /p/sock /p/file"` | sends text+Enter atomically; verified |
| Close | `herdr pane close <id>` | |
| Focus | CLI: only `herdr pane focus --direction left\|right\|up\|down [--pane ID\|--current]` | raw socket has `pane.focus` with `PaneTarget` (schema only; not called) |
| Which pane am I | `$HERDR_PANE_ID` or `herdr pane current --current` | |
| Alive? | `herdr pane get <id>` | verified: returns error code `pane_not_found` once gone |
| Send input | `pane send-text <id> <text>`, `pane send-keys <id> esc ctrl+c ...`, `pane run` | |
| Read output | `herdr pane read <id> --source recent-unwrapped --lines N`, `pane wait-output <id> --match T --timeout MS` | |

Details that matter:
- `pane split` takes no command. The new pane is an interactive shell. Run nvim with `exec` so the shell is replaced: verified that after `:qa!` the pane disappears (pane list no longer has it). Without `exec`, the shell stays after nvim quits and the pane remains (verified with `sleep 1`: no exit event, pane stays).
- Events: raw socket (newline-delimited JSON on `$HERDR_SOCKET_PATH`, default `~/.config/herdr/herdr.sock`, named sessions `~/.config/herdr/sessions/<name>/herdr.sock`): send `{"id":"s","method":"events.subscribe","params":{"subscriptions":[{"type":"pane.exited"},{"type":"pane.closed"}]}}`. Verified: first reply `{"result":{"type":"subscription_started"}}`, then on process exit `{"event":"pane_exited","data":{"pane_id":"w1:p4","workspace_id":"w1"}}`. I did not observe a `pane_closed` event for a pane that closed because its process exited (8s wait); treat `pane_exited` as the signal and do not rely on `pane_closed` for that case. `pane_closed` for explicit `pane close` not tested.
- Subscribing to a nonexistent pane id rejects the whole request and closes the connection (socket-api). Event history is not durable; on `events_lost` resubscribe and reconcile with `pane get`.
- Simpler than the socket: poll `herdr pane get <id>` (a process spawn per tick; ok at 1-2 Hz). Go client for the socket is ~40 lines (net.Dial unix + bufio + json), no deps.
- IDs are opaque `w1:p1` strings, never reused after close. `pane move` can rename them; verd does not move panes so ignore.
- Always parse JSON; errors are JSON on stderr, exit 1; syntax errors exit 2.
- `--no-focus` on split keeps user in verd; default behaviour on split focus not tested.
- Windows: herdr has a native Windows beta using named pipes (windows-beta.mdx). Not verified.
- `herdr pane` calls do not require the TUI to be attached; they need the server. Inside a pane it is guaranteed.

## Neovim control (shared)

Start: `nvim --listen <sock> <file>`. Verified: socket file appears, `nvim --server <sock> --remote-expr 'expand("%:t")'` returns the file name.

| Need | Command | Status |
|---|---|---|
| Open another file in running nvim | `nvim --server S --remote-send '<C-\><C-N>:edit +LINE path<CR>'` | `--remote-send` verified; `:edit +N` form not run |
| Run ex command | `--remote-send ':qa!<CR>'` | verified: nvim quits, socket file removed |
| Query state | `--remote-expr 'expand("%:t") . ":" . line(".")'` | verified |
| Dead server | `--server` to missing socket: `E247 ... connection refused`, exit 2 | verified |
| Programmatic | msgpack-RPC on the same socket: `nvim_command`, `nvim_call_function`; Go client `github.com/neovim/go-client/nvim` | not run; check with ctx7 before use |

Gotchas:
- `nvim --remote-silent +3 h.txt` in 0.12.5 opened a buffer literally named `+3` (verified). Do not pass `+N` as a `--remote*` argument; use `:edit +N path` via `--remote-send` or RPC.
- `--remote-send` types keys; start with `<C-\><C-N>` to leave insert/cmdline mode first. RPC `nvim_command` avoids that and is preferred over spawning `nvim --remote-send` per action.
- Socket path: unix socket paths have a ~104 (macOS) / 108 (Linux) byte limit (sockaddr_un; not verified here). Use a short dir like `$XDG_RUNTIME_DIR/verd/nvim-<pid>.sock` or `/tmp/verd-<uid>/`, never a deep repo path. Clean up if nvim crashes (stale socket).
- Pane-exit is better observed from the multiplexer than from the nvim socket; but "socket stops answering" is a mux-independent fallback.
- Prefer one nvim per verd session (reuse the pane via `--remote-send`/RPC) over a new pane per problem. Reuse requires a liveness check first.

## Platform quirks

- WSL2 (not verified, from general knowledge): tmux and herdr run inside the Linux distro so everything above applies. Keep the nvim socket on the Linux filesystem (`/tmp`, `$XDG_RUNTIME_DIR`), not `/mnt/c` (drvfs does not support unix sockets reliably). `$PWD` under `/mnt/c` works for `-c`/`--cwd`. If verd is a native Windows exe, tmux is absent; herdr Windows beta or Windows Terminal panes would be a separate adapter.
- macOS (not verified): same CLIs. tmux via Homebrew. `$TMPDIR` is long (`/var/folders/...`), so socket path length is the realistic trap. Prefer `/tmp/verd-<uid>/`. The `tmux` binary may not be on `PATH` for GUI-launched shells; use `exec.LookPath`.
- herdr stays Unix-socket based on Linux/macOS, named pipes on Windows (socket-api: "Socket transport").
- Other muxes (unverified, from docs knowledge only): zellij has `ZELLIJ` env and `zellij action new-pane`, no easy id return; wezterm has `WEZTERM_PANE` and `wezterm cli split-pane` printing the new pane id; kitty needs `allow_remote_control` and `kitten @ launch --location=vsplit`. The adapter below fits wezterm trivially; zellij and kitty are out of scope.

## Recommendation

One small interface, two implementations, and nvim control kept outside the adapter.

```go
// package pane
type Pane struct{ ID string }          // "%3" (tmux) or "w1:p2" (herdr)

type Mux interface {
    Name() string                                            // "tmux" | "herdr"
    OpenEditor(cwd string, argv []string) (Pane, error)      // split right, no focus, run argv, return id
    Alive(p Pane) bool                                       // false once the process exited / pane gone
    Focus(p Pane) error
    Close(p Pane) error
    Wait(ctx context.Context, p Pane) error                  // returns when pane is gone; poll Alive @500ms (v1)
}

func Detect(env func(string) string) Mux // nil => no multiplexer; tmux checked before herdr
```

Implementation notes:
- tmux: `OpenEditor` = `split-window -h -d -P -F '#{pane_id}' -t $TMUX_PANE -c cwd <shell-quoted argv>`. `Focus` = `select-pane`. `Close` = `kill-pane`. `Alive` = `display-message -p -t id` exit code.
- herdr: `OpenEditor` = `pane split --current --direction right --cwd cwd --no-focus`, parse `.result.pane.pane_id`, then `pane run id "exec <shell-quoted argv>"`. `Alive` = `pane get id` succeeds. `Close` = `pane close`. `Focus`: no CLI by id; v1 either skip it (user switches manually) or send raw `pane.focus` over `$HERDR_SOCKET_PATH`. Decide at implementation time.
- Choose the split direction in verd: herdr docs suggest right for wide panes, down for narrow (`pane layout`); tmux: `-h` always is fine for v1.
- Editor control is a separate small type: `Nvim{Sock string}` with `Open(path, line)`, `Quit()`, `Alive()`. Socket path chosen by verd before launch and passed with `--listen`. Use RPC (`nvim_command`) via the Go client if the dependency is acceptable, else `nvim --server S --remote-send` through `exec.Command` (zero deps, verified).
- Exit detection v1: `tea.Tick` 500ms calling `Mux.Alive` (and nvim socket as fallback). Upgrade to herdr `events.subscribe pane.exited` or a tmux `pane-exited` hook + `wait-for` only if polling cost or latency matters.
- Always shell-quote argv for both muxes (both pass a string to a shell). Keep one `quote()` helper; do not build commands by concatenation of user-controlled paths.
- No-mux fallback: `OpenEditor` returns `ErrNoMux`; verd shows a one-line hint ("run verd inside tmux or herdr") or execs `nvim` in the same terminal via `tea.ExecProcess`.

## Not verified

- macOS and WSL behaviour (no such host here); socket-path limits; Windows-native herdr.
- `$TMUX` field format; `TERM_PROGRAM=tmux` behaviour; tmux-inside-herdr both-vars case (reasoned from docs).
- `wait-for` hook plumbing for tmux.
- herdr: `pane_closed` event for explicit close; default focus behaviour of `pane split` without flags; raw `pane.focus` call and its params; run-right-after-split timing (I slept between split and run); herdr inside tmux.
- nvim: `:edit +N` via `--remote-send`; msgpack-RPC and the Go client library.
- zellij, kitty, wezterm details.

## Sources

- tmux manual (split-window, new-pane, send-keys, select-pane, kill-pane, display-message, hooks `pane-exited`/`pane-died`, `remain-on-exit`, pane ids, `TMUX_PANE`): https://man.openbsd.org/tmux (checked against local `man tmux`, 3.7c); project: https://github.com/tmux/tmux
- herdr docs v0.9.3 (raw): CLI reference https://raw.githubusercontent.com/herdrdev/herdr/v0.9.3/docs/next/website/src/content/docs/cli-reference.mdx ; Socket API https://raw.githubusercontent.com/herdrdev/herdr/v0.9.3/docs/next/website/src/content/docs/socket-api.mdx ; Agent automation https://raw.githubusercontent.com/herdrdev/herdr/v0.9.3/docs/next/website/src/content/docs/agent-automation.mdx ; index https://herdr.dev/llms.txt ; repo https://github.com/herdrdev/herdr
- herdr local: `herdr --help`, `herdr pane --help`, `herdr api schema`, `herdr --skill` (0.9.3)
- Neovim remote/RPC docs: `:help --listen`, `:help --remote`, `:help --server` (local `/usr/share/nvim/runtime/doc/remote.txt`); https://neovim.io/doc/user/remote.html ; https://neovim.io/doc/user/api.html
