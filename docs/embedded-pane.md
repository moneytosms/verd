# Embedded pane

The embedded pane draws Neovim **inside** verd's window, on the right, with verd on the left. It is the default when you are not in tmux or herdr (`split = "auto"`); inside one of them verd still opens a real split. Set `split = "suspend"` to get the old behavior (verd hands the terminal to Neovim and resumes when you quit), or `split = "embedded"` to use the pane inside tmux too.

```toml
split = "embedded"
embed_ratio = 0.4          # share of the width verd keeps
embed_side = "right"        # which column Neovim takes: right | left
embed_zoom = false          # start the editor zoomed (fullscreen) when it opens
embed_focus_key = "ctrl+\\" # toggles keyboard focus between verd and Neovim
```

Keys go to Neovim while it has focus. The following editor keys work while the editor is open and has the keyboard:

| Key | Action |
|-----|--------|
| `ctrl+\` | toggle keyboard between verd and the editor |
| `alt+h` | keyboard to the left column |
| `alt+l` | keyboard to the right column |
| `alt+z` | zoom whichever side has the keyboard to the whole window (toggle) |
| `alt+]` | grow the editor column by 5% |
| `alt+[` | shrink the editor column by 5% |
| `alt+s` | swap editor between left and right column |

All of these are rebindable under `[keys.editor]`. Clicking a column also focuses it. If the window is too narrow for both, verd shows itself alone.

## Known gaps

verd runs Neovim in a pseudo-terminal and renders it with `charmbracelet/x/vt`. That emulator does not implement the Kitty keyboard protocol, so chords that a terminal can only report through that protocol (for example some `ctrl+shift` combinations) are not encoded for Neovim. Everything a plain xterm-style terminal can send works.

If you rely on such chords, use `tmux` or `herdr`, where Neovim runs in a real pane with full fidelity, or `split = "suspend"`.

See [ADR 0002](./adr/0002-multiplexer-split-over-embedded-terminal.md) for why a multiplexer split is preferred when one is available.

## Promotion checklist

The embedded pane was promoted to the default for `split = "auto"` outside a multiplexer ([#45](https://github.com/moneytosms/verd/issues/45)) **by the maintainer's decision, before this checklist was run on real terminals**: the results below are still unrecorded. Run it and report failures; if a row fails, it is a bug to fix or a gap to list above, and `split = "suspend"` is the escape hatch.

Run each item in an embedded session (`split = "embedded"`, LazyVim config) and record the result.

| # | Check | How |
| --- | --- | --- |
| 1 | LazyVim colors | Open a Solution. The colorscheme, statusline and diagnostics render with correct true color, no stray glyphs. |
| 2 | Mouse | Click to place the cursor, drag to select, scroll with the wheel inside the pane. |
| 3 | Resize | Resize the terminal and change `embed_ratio`. Neovim redraws to the new size with no corruption. |
| 4 | Paste | Paste a multi-line block in insert mode. Bracketed paste applies it once, without auto-indent staircase. |
| 5 | Common chords | `<leader>` mappings, `ctrl+w` window commands, `ctrl+o` / `ctrl+i`, `:` commands, `ctrl+v` visual block. |
| 6 | Focus | The focus key moves keys between verd and Neovim, in both directions, in the middle of an operator-pending command. |
| 7 | Round trip | `e` opens the Solution, `:w` triggers autotest, the Tests panel updates, `:q` closes the pane and verd takes the full width. |

| Terminal | OS | Result |
| --- | --- | --- |
| _not yet recorded_ | Linux | |
| _not yet recorded_ | macOS | |

List any degraded chords you find in [Known gaps](#known-gaps).
