# Prototype: verd TUI screens

Throwaway prototype for "What do the TUI screens, layout and keybindings look like?". ASCII mockups to react to, not a pixel spec. Colors come from the active theme (ANSI 16 by default).

## Navigation model

- Top tab bar with 4 sections, switched by `1`-`4`: **Problems**, **Contests**, **Stats**, **Picker**.
- Lists use vim keys: `j`/`k` move, `g`/`G` top/bottom, `/` filter, `enter` open, `esc` back.
- The Problem view is a stack page opened from any list; `esc` returns to the list it came from.
- `?` toggles a help overlay listing the current screen's keys (bubbles `help` component). `q` quits from a top-level tab.
- Status line (bottom): handle + rating, offline badge + last sync age, background job spinner (refresh, Test Run, Verdict polling).

## 1. Problems (problemset browser)

```
 verd  [1 Problems]  2 Contests  3 Stats  4 Picker                 tourist 3742
 ──────────────────────────────────────────────────────────────────────────────
  filter: rating 1400-1700  tags: dp,greedy  [x] unsolved          312 matches
 ──────────────────────────────────────────────────────────────────────────────
    ID      Name                               Rating  Tags              Solved
  ▸ 1900D   Small GCD                          1600    dp, math          8.1k
    1899E   Queue Sort                         1500    greedy, sort      9.4k
  ✓ 1896C   Matching Arrays                    1400    greedy, sort      14k
  ✗ 1895D   XOR Construction                   1700    bitmasks          7.0k
 ──────────────────────────────────────────────────────────────────────────────
  ● online · synced 3m ago                          enter open  f filter  ? help
```
`✓` solved, `✗` attempted-unsolved. `f` opens a filter form (rating range, tags include/exclude, unsolved toggle).

## 2. Problem view (verd pane next to Neovim in tmux/herdr)

verd keeps the left pane; Neovim opens in a right split. In embedded mode verd draws the same split itself.

```
┌ 1900D · Small GCD ─────────── 2s · 256MB ┐┌ nvim: ~/verd/1900/D/main.cpp ──────┐
│ Let a, b, c be integers. Define f(a,b,c) ││ #include <bits/stdc++.h>           │
│ = gcd(a, b) where a ≤ b ≤ c ...          ││ using namespace std;               │
│                                          ││                                    │
│ Input                                    ││ int main() {                       │
│ The first line contains t (1 ≤ t ≤ 10⁴)  ││     int t; cin >> t;               │
│ ...                                      ││     █                              │
├ Tests ─────────── cpp · tokens ──────────┤│                                    │
│  ✓ sample 1     12ms   3.1MB             ││                                    │
│  ✗ sample 2     WA     line 2 col 4      ││                                    │
│  ✓ custom 1      8ms   3.0MB             ││                                    │
│  last run: on save · 1/3 failing         ││                                    │
├──────────────────────────────────────────┤│                                    │
│ e edit  t test  s submit  a add test     ││                                    │
│ d diff  l lang  c compare  o browser     ││                                    │
└──────────────────────────────────────────┘└────────────────────────────────────┘
```
Statement and Tests panel scroll independently (`tab` switches focus). Keys:
`e` open/focus Neovim, `t` Test Run, `s` submit, `a` add Custom Test (opens `.in`/`.ans` in Neovim), `d` diff of the selected failing test, `l` switch language, `c` cycle Comparison Mode, `o` open Problem in browser, `r` re-fetch statement, `y` copy Solution.

## 3. Diff (overlay on Problem view)

```
┌ sample 2 · WA ───────────────────────────────────────────────┐
│ expected                     │ actual                        │
│ 3                            │ 3                             │
│ 1 2 [4]                      │ 1 2 [5]                       │
│ 7                            │ 7                             │
│                        first mismatch: line 2 col 5          │
└──────────────────────────────────────── esc close  j/k scroll┘
```

## 4. Live Verdict (toast in Problem view)

```
  ⟳ Submission 245112233 · Testing on test 17 ...
  ✓ Accepted · 46 ms · 3.9 MB                     (stays 5 s, then into history)
  ✗ Wrong answer on test 4                        (sticky until dismissed)
```
In `browser` submit mode the toast first reads "Copied · submit page opened · waiting for your Submission ...".

## 5. Contests

```
 verd  1 Problems  [2 Contests]  3 Stats  4 Picker
  Upcoming
    Codeforces Round 990 (Div. 2)      starts in 1d 4h      2h
  Past
  ▸ Codeforces Round 989 (Div. 1)      Oct 01             solved 3/7
    Educational Round 170              Sep 28             solved 5/6
```
`enter` lists the contest's Problems (same table as Problems, with status per index).

## 6. Stats

```
 verd  1 Problems  2 Contests  [3 Stats]  4 Picker
  Rating 1642 (max 1710) · Expert-ish     Solved 418 · AC rate 61% · streak 6 (max 21)
  ┌ rating ──────────────────────────────┐ ┌ solved by rating ─────────────────┐
  │                      ╭─╮   ╭──       │ │ 800  ████████████ 96             │
  │            ╭───╮ ╭──╯   ╰──╯         │ │ 1200 ████████ 71                 │
  │   ╭───────╯   ╰─╯                    │ │ 1600 ███ 28                      │
  └──────────────────────────────────────┘ └──────────────────────────────────┘
  Strengths  greedy 64% · sortings 58% · math 51%
  Weaknesses dp 12% · graphs 9% · trees 7%            p pick from weak topics
```

## 7. Picker

```
 verd  1 Problems  2 Contests  3 Stats  [4 Picker]
  rating 1600-1800   tags +dp  -geometry   unsolved only   preset: weak topics
  ┌──────────────────────────────────────────┐
  │  1895D · XOR Construction · 1700 · dp    │
  └──────────────────────────────────────────┘
             enter open   space re-roll   f filters
```

## Decisions this prototype encodes

- Tab bar of 4 sections + one stacked Problem view; no deeper navigation.
- verd never hosts the editor UI itself except in embedded mode; the Problem view is designed to be a narrow left pane (min ~44 cols).
- Charts are line/bar charts drawn with Unicode blocks/braille.
