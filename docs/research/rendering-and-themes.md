# Rendering statements, images and themes in the terminal

Issue: #6 (part of #1). Researched 2026-10-05. Versions checked against module source in the Go module cache (not memory).

## Recommendation

1. **Math**: custom `$$$...$$$` to Unicode mapper (small symbol table + sub/superscript maps), run on the HTML *before* HTML->Markdown. No Go LaTeX->Unicode library found (searched); fall back to raw TeX for anything unmapped.
2. **Pipeline**: HTML -> (convert math to Unicode / placeholders) -> `html-to-markdown/v2` -> `glamour/v2` with a verd-owned style. Do not feed `$$$` through html-to-markdown as-is (it escapes `_` and `\`, verified).
3. **Images v1**: no inline images. Render `[image: alt]` as an OSC 8 hyperlink to the URL (`ansi.SetHyperlink`) plus an "open in browser" key. Inline images = later opt-in (kitty first, then sixel/iTerm2), behind detection and a config flag, in a full-screen image view.
4. **Themes**: Bubble Tea v2 / Lip Gloss v2 are stable (v2.0.x). Default theme = ANSI 16 indices (`lipgloss.Color("0".."15")` / `lipgloss.Red` etc.) so the terminal palette is inherited. Named themes = structs of `color.Color` (hex), chosen in config. Use `tea.RequestBackgroundColor` + `tea.BackgroundColorMsg.IsDark()` + `lipgloss.LightDark` only where a hex theme needs light/dark variants; default to dark if no reply.
5. Use the new import paths `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/glamour/v2` (not `github.com/charmbracelet/...`).

## (a) Statement HTML -> terminal text

### Math delimiter
Polygon/Codeforces statements wrap formulas in `$$$...$$$` for MathJax ([Polygon TeX manual](https://polygon.codeforces.com/docs/statements-tex-manual); [CF blog 75765](https://codeforces.com/blog/entry/75765)). Per search-result summary, MathJax problems (after 2021-06-01) allow any modern LaTeX inline, so coverage cannot be total; expect a long tail.

### Go libs for LaTeX -> Unicode
`gh search repos` for latex/unicode in Go returned nothing relevant (only `go-latex/latex`, a document renderer, and TeX-to-PDF tools). Non-Go prior art whose symbol tables could be ported (check licenses): [svenkreiss/unicodeit](https://github.com/svenkreiss/unicodeit), [HDembinski/unicodeitplus](https://github.com/HDembinski/unicodeitplus). Conclusion: write our own.

Custom mapping scope (validate on a corpus of real statements):
- Symbols: `\le \ge \ne \cdot \times \ldots \dots \sum \prod \infty \lfloor \rfloor \lceil \rceil \oplus \in \to \rightarrow \pm \approx \leq \geq \emptyset \cap \cup \subseteq \lvert \rvert` -> `≤ ≥ ≠ · × … … ∑ ∏ ∞ ⌊ ⌋ ⌈ ⌉ ⊕ ∈ → → ± ≈ ≤ ≥ ∅ ∩ ∪ ⊆ | |`; `\bmod`, `\pmod` -> text.
- `^{...}` / `_{...}`: Unicode super/subscripts exist for digits, `+ - = ( )` and some letters only (subscripts miss b, c, d, f, g, q, w, y, z; many capitals have no superscript). Fallback for unmappable content: keep readable ASCII such as `a_(i+1)`. Pick one form and use it consistently.
- `\frac{a}{b}` -> `(a)/(b)`; `\sqrt{x}` -> `√(x)`; `\text`/`\mathrm` -> plain; `\left`/`\right` dropped.
- Unmapped command: emit original TeX unchanged, never drop silently.
- Display math: own line, indented. Matrices/arrays: raw TeX (out of scope).
- Ship as a table-driven Go test fed with real statements.

### HTML -> Markdown
[`JohannesKaufmann/html-to-markdown/v2`](https://github.com/JohannesKaufmann/html-to-markdown) v2.5.2 (2026-06-07), `htmltomarkdown.ConvertString`. **Verified by running it**: `$$$a_1, \ldots, a_n$$$` came out as `$$$a\_1, \\ldots, a\_n$$$` (escaped); `<img src alt>` -> `![alt](src)`; `<pre><div class="test-example-line">` -> fenced block with a leading blank line. So convert math to Unicode (or opaque placeholders restored after conversion) on the HTML text first.
Sample `<pre>` blocks: extract separately (needed for the test runner anyway) and render as plain styled blocks, not through glamour.

### Glamour
[`charm.land/glamour/v2`](https://github.com/charmbracelet/glamour) v2.0.1 (2026-06-12). Verified from source: `WithAutoStyle()` was **removed**; default style is `dark`; options include `WithStandardStyle`, `WithStyles(ansi.StyleConfig)`, `WithStylesFromJSONBytes`, `WithWordWrap`, `WithPreservedNewLines`. Built-in styles in `styles/`: dark, light, dracula, tokyo-night, pink, ascii, notty. Built-ins use 256-color/hex values, not ANSI 16; for palette inheritance build an `ansi.StyleConfig` from the verd theme. Glamour does no math and no images (alt/link text only).
Wrap width from viewport via `WithWordWrap`; re-render on `tea.WindowSizeMsg`; cache by (problem, width, theme).

## (b) Images

### Facts
- Protocols: kitty graphics, sixel, iTerm2 inline. Support matrix from the [rasterm README](https://github.com/BourgeoisBear/rasterm): kitty, ghostty = kitty; iTerm2, WezTerm, rio, mintty, mlterm = sixel + iTerm2; xterm = sixel; putty = none.
- Windows Terminal: Sixel since 1.22 ([release post](https://devblogs.microsoft.com/commandline/windows-terminal-preview-1-22-release/)).
- tmux: sixel only if built with `--enable-sixel` (added 3.4, [CHANGES](https://github.com/tmux/tmux/blob/master/CHANGES)); other protocols need `allow-passthrough on|all` (>= 3.3, [FAQ](https://github.com/tmux/tmux/wiki/FAQ)) plus DCS wrapping. Kitty has Unicode placeholders meant for multiplexers/non-graphics-aware apps ([spec](https://sw.kovidgoyal.net/kitty/graphics-protocol/)).
- Detection: kitty spec says send graphics query `a=q` followed by `CSI c`; a graphics reply before the DA reply means supported. Sixel: DA1 attribute `4`. Bubble Tea v2 `raw.go` documents this pattern (`tea.Raw(ansi.RequestPrimaryDeviceAttributes)`).
- Go libs:
  - [BourgeoisBear/rasterm](https://github.com/BourgeoisBear/rasterm) v1.1.2 (last push 2025-12-21): encodes kitty/iTerm2/sixel; README lists tmux detection and terminal identification as TODO.
  - [mattn/go-sixel](https://github.com/mattn/go-sixel) (last push 2026-07-06): sixel only.
  - [charmbracelet/x/ansi](https://github.com/charmbracelet/x/tree/main/ansi) v0.11.8: sequence builders `KittyGraphics`, `SixelGraphics`, `ITerm2`, `SetHyperlink`/`ResetHyperlink`; already a Bubble Tea v2 dependency.
  - [charmbracelet/x/mosaic](https://github.com/charmbracelet/x/tree/main/mosaic) (half-block) and [blacktop/go-termimg](https://github.com/blacktop/go-termimg) (multi-protocol, Bubble Tea demo): named in bubbletea#163, not inspected.
- **Bubble Tea status**: [bubbletea#163 "Displaying images"](https://github.com/charmbracelet/bubbletea/issues/163) is open; no first-class image support. Maintainer comment (2024-11): terminals claim support but are buggy/inconsistent, e.g. cursor position after a sixel image deviates from spec across terminals. A later commenter on v2 reports difficulty keeping layout and image redraw correct. Likely cause (inference): the renderer diffs cells, raw escapes bypass its model, so images are not cleared/moved on scroll, resize, or view switch.
- OSC 8 hyperlinks: `ansi.SetHyperlink` and Lip Gloss `Style.Hyperlink` exist; `tea.View.Content` is documented as styled strings "with styles and hyperlinks". Unsupporting terminals ignore it.

### Recommendation
MVP: alt text + OSC 8 link + key to open URL (`xdg-open`/`open`/`start`). Later, optional image view owning the whole alt-screen (no overlap with diffed text), kitty via `ansi.KittyGraphics` or sixel after positive detection, deleting images (`a=d`) on exit. Never emit images when detection is inconclusive, under tmux without passthrough, or without a TTY.

## (c) Themes

### Status (verified via `gh` releases and module source)
- Bubble Tea v2.0.10 (2026-09-24), Lip Gloss v2.0.6 (2026-08-11), Glamour v2.0.1 (2026-06-12); none flagged prerelease (glamour v2.0.0: 2026-03-09). Import paths `charm.land/{bubbletea,lipgloss,glamour}/v2`. Stable: use v2.
- v2 changes relevant here: `View()` returns `tea.View` (fields `BackgroundColor`, `ForegroundColor`, `AltScreen`, `MouseMode`, `WindowTitle`, `ProgressBar`); colors are `image/color.Color`; adaptive colors replaced by `lipgloss.LightDark(isDark)(light, dark)` (`lipgloss/compat` exists as a shim); downsampling happens at output ([Lip Gloss upgrade guide](https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md)).

### Inheriting the terminal palette
`lipgloss.Color("0".."15")` returns `ansi.BasicColor`; constants `lipgloss.Black ... BrightWhite` exist (lipgloss `color.go`). Basic colors are SGR 30-37/90-97 (standard ANSI behavior), so the terminal's palette applies; 256-color and hex are fixed values and do not follow the palette. Caveat: ANSI 16 has no contrast guarantee across themes; keep body text on default fg/bg and use ANSI colors for accents.

### Light/dark detection
- Bubble Tea: return `tea.RequestBackgroundColor` from `Init`; reply is `tea.BackgroundColorMsg` with `.IsDark()`; needs input enabled (doc: use `WithInput`). `RequestForegroundColor` also exists.
- Standalone Lip Gloss: `lipgloss.HasDarkBackground(os.Stdin, os.Stdout)`; **returns true on error**; `BackgroundColor` errors when not a TTY.
- Styles from a bool: `ld := lipgloss.LightDark(isDark)`.
- Glamour has no auto-detect now; pick `dark`/`light` (or own `StyleConfig`) from the same bool.
- Detection is an OSC 11 query + reply, so it depends on the terminal answering and every hop (ssh, tmux, WSL) relaying it. Always default to dark and offer config `theme = "auto|dark|light|<name>"`.

### Named themes
`type Theme struct{ Name string; Accent, Muted, OK, Warn, Err color.Color; ... }` with a `Styles` constructor taking a Theme. Ship `ansi` (default, ANSI 16) plus a couple of hex themes. Codeforces rank colors (gray/green/cyan/blue/violet/orange/red) map to ANSI 16 in the default theme, hex in others. Keep themes as Go values; no theme engine until more than ~2 exist.

## Could not verify
- Live Codeforces statement markup (codeforces.com returned HTTP 403 to WebFetch): HTML for math, images, sample blocks is assumed, not observed. Next: capture a few statements into `testdata/` via browser/authenticated fetch.
- Unicode math coverage against real statements (no corpus tested).
- Actual image rendering in any terminal; kitty/sixel behavior inside Bubble Tea v2; tmux passthrough behavior; WSL specifics (only Windows Terminal Sixel support is sourced).
- OSC 11 background query behavior over SSH/tmux/WSL (reasoned, not tested).
- `x/mosaic` and `go-termimg` APIs not inspected; WezTerm kitty support, Windows Terminal kitty/iTerm2 support not sourced.
- Polygon claim "any modern LaTeX" comes from a search-result summary, not a read of the page.

## Sources
- bubbletea v2.0.10 source (`color.go`, `raw.go`, `tea.go`): https://github.com/charmbracelet/bubbletea
- lipgloss v2.0.6 source (`color.go`, `query.go`, `UPGRADE_GUIDE_V2.md`): https://github.com/charmbracelet/lipgloss
- glamour v2.0.1 source (`glamour.go`, `UPGRADE_GUIDE_V2.md`): https://github.com/charmbracelet/glamour
- charmbracelet/x/ansi: https://github.com/charmbracelet/x/tree/main/ansi
- html-to-markdown: https://github.com/JohannesKaufmann/html-to-markdown
- rasterm: https://github.com/BourgeoisBear/rasterm ; go-sixel: https://github.com/mattn/go-sixel
- bubbletea#163: https://github.com/charmbracelet/bubbletea/issues/163
- Kitty graphics protocol: https://sw.kovidgoyal.net/kitty/graphics-protocol/
- tmux FAQ / CHANGES: https://github.com/tmux/tmux/wiki/FAQ , https://github.com/tmux/tmux/blob/master/CHANGES
- Windows Terminal 1.22: https://devblogs.microsoft.com/commandline/windows-terminal-preview-1-22-release/
- Polygon TeX manual: https://polygon.codeforces.com/docs/statements-tex-manual
