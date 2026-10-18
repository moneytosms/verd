# Read, edit and test

## The Problem view

`enter` on a problem opens it. On a terminal at least 100 columns wide the view is split: the statement on the left; on the right the rating, limits and tags, the list of tests with their verdicts, and the selected test's input, expected and output.

| Key | Does |
| --- | --- |
| `tab` / `shift+tab` | move between statement, tests and detail |
| `j` `k` `pgup` `pgdown` | scroll or move |
| `y` | copy the focused pane (the whole question with difficulty and tags, for pasting into notes or a chat) |
| `v` | show or hide the tags |
| `o` | open the problem in the browser |
| `r` | refetch the statement |
| `esc` | close the problem |

Switching tabs does not close a problem; it waits for you on its own tab.

**Images** in statements are fetched in the background and drawn in place: as real pixels in kitty and Ghostty (the Kitty graphics protocol), and as colour half-blocks in every other terminal, Alacritty included. Until an image arrives, or when it cannot be fetched, you see `[image]` with its link. In tmux or zellij verd uses half-blocks. Change it with `verd config set images auto|kitty|blocks|off`.

## Edit

`e` creates `main.<ext>` from your [Template](./snippets.md) the first time and opens your editor.

- **Inside tmux or herdr** the editor opens in a split next to verd. Pressing `e` again reuses it.
- **Anywhere else** Neovim is drawn inside verd's own window (the embedded pane). Keys go to Neovim while it has focus; `ctrl+\` hands them back to verd.
- `split = "suspend"` hands the whole terminal to the editor instead.

```sh
verd config set split suspend          # the old behavior
verd config set editor "code -w"       # any editor; extra arguments are allowed
```

`l` switches the language for this problem (`cpp`, `c`, `python`, or your own). `N` opens `notes.md` for the problem. See [Neovim and editors](./editors.md).

## Test

`t` compiles the solution (cached by content, so unchanged code is never rebuilt) and runs it against every test: the samples from the statement and your own. Each test gets a verdict, a time and peak memory.

| Verdict | Meaning |
| --- | --- |
| `AC` | output matches |
| `WA` | output differs; the first mismatch (line, column, wanted, got) is shown |
| `TLE` / `MLE` | over the time or memory limit |
| `RE` | crashed or non-zero exit |
| `CE` | the compiler failed; its output is shown |

With `autotest` on (default) saving the file runs the tests. From the shell: `verd test main.cpp` (exit `0` only if everything is `AC`). Add `-v` to print every test's input, expected, stdout and stderr (your debug prints), and `-t sample-2` to run one test.

### Looking at a failure

- `d` opens a side-by-side diff of expected and actual output, scrolled to the first mismatch.
- `f` opens the selected test full-screen: input, expected, stdout and stderr, with panning for long lines.
- `I`, `E`, `O` copy only the input, the expected output or the output of the selected test.
- A compile error shows the compiler output; `e` jumps to the first error line.

### Comparison modes

`c` cycles how output is compared; the choice is saved per problem.

| Mode | Matches when |
| --- | --- |
| `tokens` | whitespace-separated tokens are equal (like the Codeforces checker) |
| `exact` | lines are equal after trimming trailing spaces |
| `float` | numbers within `float_eps` (a statement saying "within 1e-6" selects it) |
| `none` | never judged; output is only shown |

### Your own tests

```text
a      add a Custom Test (creates tests/custom-N.in and .ans and opens them)
T      manage tests: view, edit, copy a sample into an editable test, delete
```

Samples are read-only; copy one with `c` in the manager to change it.

Next: [Stress testing](./stress.md), or [submit](./submit.md).
