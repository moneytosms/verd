# Embedded terminal pane in Bubble Tea (issue #5)

Question: which Go libs can run a PTY child (Neovim) and render it inside a Bubble Tea view? Researched 2026-10-05. Star/commit figures from the GitHub API on that date.

## Recommendation

1. **Primary path stays tmux** (outside this ticket). Embedding is the fallback only.
2. **Fallback v0: `tea.ExecProcess`** (suspend verd, give Neovim the real terminal, resume on exit). Zero emulation, full Neovim fidelity, ~10 lines. Ship this first.
3. **Fallback v1 (only if split-view is needed): `creack/pty` or `charmbracelet/x/xpty` + `charmbracelet/x/vt`** on **Bubble Tea v2**. It is the only stack that is actively maintained, from the same vendor as Bubble Tea, and covers alt screen, SGR mouse, bracketed paste, focus events and 256/truecolor.
4. Do not use `hinshun/vt10x` (last push 2023-12). Treat `taigrr/bubbleterm` as a reference, not a dependency (31 stars, Bubble Tea version unverified).
5. Risk to accept if v1 is built: `x/vt` and `x/xpty` have no tagged releases (pseudo-versions); pin a commit. Kitty keyboard / modifyOtherKeys not supported by `x/vt` (see below), so some Neovim chords may be lost.
6. Verd must target Bubble Tea v2 (`charm.land/bubbletea/v2`) for option 3. Check the existing verd go.mod before choosing.

## Candidates

| Lib | Role | Maturity (2026-10-05) | Notes |
|---|---|---|---|
| [creack/pty](https://github.com/creack/pty) | PTY spawn/resize (Unix) | 2096 stars, pushed 2026-06-01, 17 open issues, v1.1.24 | De facto standard; no Windows ConPTY |
| [charmbracelet/x/xpty](https://pkg.go.dev/github.com/charmbracelet/x/xpty) | PTY, Unix + Windows ConPTY | no tagged release (pseudo-version); v0.1.4 used by tuios | Windows: use `WaitProcess()`, `Wait` broken |
| [charmbracelet/x/vt](https://pkg.go.dev/github.com/charmbracelet/x/vt) | VT emulator | repo `charmbracelet/x`: 320 stars, pushed 2026-10-04; pkg has no tagged version | See feature table below |
| [hinshun/vt10x](https://github.com/hinshun/vt10x) | VT emulator | 52 stars, last push 2023-12-06 | Stale; do not pick |
| [taigrr/bubbleterm](https://github.com/taigrr/bubbleterm) | Emulator + Bubble Tea-style rendering | 31 stars, pushed 2026-10-03, 2 issues, 0BSD | README: 256/truecolor, alt screen + scrollback, mouse, resize; bracketed paste not documented; says it may swap emulator later |
| [grafviktor/termview](https://github.com/grafviktor/termview) | Bubble Tea v2 terminal component (x/vt based) | 1 star, v0.4.0, 43 commits, pushed 2026-10-01 | Ready-made `termview.New(WithCommand(...))`; too young to depend on |
| [Gaurav-Gosain/tuios](https://github.com/Gaurav-Gosain/tuios) | Full multiplexer on Bubble Tea v2 | 4670 stars, pushed 2026-10-04 | Proof the pattern works at scale; go.mod: bubbletea/v2 v2.0.8, ultraviolet, x/xpty v0.1.4 |

## What `charmbracelet/x/vt` supports

From [vt/mode.go](https://github.com/charmbracelet/x/blob/main/vt/mode.go) and the [pkg docs](https://pkg.go.dev/github.com/charmbracelet/x/vt):

| Neovim need | Support |
|---|---|
| Alt screen | Yes (?1047, ?1049, `IsAltScreen()`) |
| Mouse | Yes (?1000/1002/1003, SGR ?1006) via `SendMouse` |
| Bracketed paste | Yes (?2004) via `Paste()` |
| Focus events | Yes (?1004) |
| Cursor | `CursorPosition()`; cursor-key app mode handled in `SendKey` |
| Resize | `Resize(w,h)`; also call PTY resize |
| Colors | Indexed + RGB via cell styles; `Render()` returns ANSI string |
| Kitty keyboard / CSI u / modifyOtherKeys | **No**: `// TODO: Support Kitty, CSI u, and XTerm modifyOtherKeys` in [vt/key.go](https://github.com/charmbracelet/x/blob/main/vt/key.go) |
| Scrollback | Yes (`Scrollback()`) |

## Wiring (design sketch, not built)

- Child: `xpty.NewPty(w,h)` (or `creack/pty`), `Start(exec.Command("nvim"))`; goroutine reads PTY into `vt.Emulator.Write`, then sends a `tea.Msg` to trigger redraw (tuios: event-driven, renders only when PTY data arrives, per its README).
- Emulator answers terminal queries (DA, cursor position) on its `Read()` side; pump that back into the PTY.
- Input: Bubble Tea v2 `tea.KeyPressMsg` is `type KeyPressMsg Key` ([key.go](https://github.com/charmbracelet/bubbletea/blob/main/key.go)); `vt.KeyPressEvent` is an alias of ultraviolet's type. A field-copy conversion is expected. Mouse: Bubble Tea v2 reuses `uv.MouseButton`. Offset mouse coords by the pane origin.
- Render: `View()` returns `emu.Render()` inside a Lip Gloss box; set cursor via the v2 `View` cursor field. Alt-screen state is the emulator's, not verd's.
- Reserve one escape chord (e.g. ctrl+\) to return focus to verd; everything else goes to the PTY.

## Suspend/resume fallback: `tea.ExecProcess`

[bubbletea exec.go](https://github.com/charmbracelet/bubbletea/blob/main/exec.go): `func ExecProcess(c *exec.Cmd, fn ExecCallback) Cmd`; `ExecCallback func(error) Msg`. The [docs](https://pkg.go.dev/github.com/charmbracelet/bubbletea/v2#ExecProcess) say it is for spawning editors/shells; the program releases the terminal during the run and restores it after. `tea.Exec` takes a custom `ExecCommand` (Run/SetStdin/SetStdout/SetStderr). Use `nvim <file>`, handle the callback msg to refresh state (e.g. re-read the solution file).

## Bubble Tea v2

Latest release v2.0.10 (2026-09-24); module path `charm.land/bubbletea/v2` ([go.mod](https://github.com/charmbracelet/bubbletea/blob/main/go.mod)). x/vt requires `ultraviolet` and `x/ansi`, the same stack v2 is built on, so it is v2-native. v1 compatibility not verified; assume v1 needs a bridging conversion.

## Not verified

- No code was built or run; no Neovim end-to-end test with any lib. All "supports" claims come from source/docs reading.
- Render performance: no benchmark. Only tuios's self-reported claims (event-driven, no idle CPU, style caching); x/vt cost for a full-screen Neovim redraw unmeasured.
- Whether Neovim chords relying on Kitty/CSI-u keys degrade with x/vt in practice.
- Exact `tea.KeyPressMsg` to `uv.KeyPressEvent` conversion code.
- bubbleterm: Bubble Tea version, internal emulator, bracketed paste (README silent; go.mod fetch 404).
- termview and tuios: whether Neovim runs correctly inside; tuios README does not mention it.
- Open-issue content of x/vt (charmbracelet/x has 111 open issues total; not triaged for vt).
- Windows behavior beyond xpty docs.
- verd's current Bubble Tea version (no go.mod in this worktree yet).

## Sources

- https://github.com/creack/pty
- https://pkg.go.dev/github.com/charmbracelet/x/vt
- https://pkg.go.dev/github.com/charmbracelet/x/xpty
- https://github.com/charmbracelet/x/tree/main/vt
- https://github.com/hinshun/vt10x
- https://github.com/taigrr/bubbleterm
- https://github.com/grafviktor/termview
- https://github.com/Gaurav-Gosain/tuios
- https://github.com/charmbracelet/bubbletea
- https://pkg.go.dev/github.com/charmbracelet/bubbletea/v2#ExecProcess
