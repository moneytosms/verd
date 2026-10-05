# Testing and stress

## Test Runs

A Test Run compiles the Solution (cached by content), then runs it against every complete test in `tests/`: the Sample Tests from the statement (`sample-N`) and your Custom Tests (`custom-N`). Each test gets a **Local Verdict**, a time, and peak memory.

| Verdict | Meaning |
| --- | --- |
| `AC` | Output matches under the Comparison Mode. |
| `WA` | Output differs. verd shows the first mismatch (line, column, wanted, got). |
| `TLE` | Over the Problem's time limit times `time_multiplier`. |
| `RE` | Non-zero exit or killed by a signal. |
| `MLE` | Peak memory over the Problem's limit. Reported and flagged, not enforced. |
| `CE` | The compiler failed. The compiler output is shown. |
| `??` | Comparison Mode `none`: output is shown, not judged. |

Output beyond 64 MB is cut and the test is `RE` with an "output limit exceeded" note, so a runaway print loop cannot fill your disk. Interactive Problems are not supported locally.

Run tests with `t` in the TUI, by saving the Solution (`autotest`), or with `verd test <file>`. The command exits `0` only if every test is `AC`, which makes it usable in scripts and editor jobs.

## Comparison Modes

| Mode | Matches when |
| --- | --- |
| `tokens` | Whitespace-separated tokens are equal. Like Codeforces' default checker. |
| `exact` | Equal after trimming trailing whitespace per line and trailing blank lines. |
| `float` | Tokens equal, with numbers compared within `float_eps` (absolute or relative). |
| `none` | Never judged. |

The default comes from the statement (a note like "answers within 1e-6" selects `float`), else `tokens`. Press `c` to cycle; the choice is saved per Problem.

## Custom Tests

Press `a`. verd creates the next `tests/custom-N.in` and `custom-N.ans`, empty, and opens them side by side. An incomplete pair (a lone `.in`) is ignored until the `.ans` exists. Existing files are never overwritten.

## Stress testing

A stress test generates random inputs until your Solution and a brute force disagree.

1. Press `S` (or run `verd stress <file>`). On first use verd creates `gen.<ext>` and `brute.<ext>` from your Templates and, in the TUI, opens them. Fill them in:
   - `gen` prints one random input; `argv[1]` is the seed.
   - `brute` is a slow but obviously correct Solution for the same Problem.
2. Press `S` again. For seed `1, 2, 3, ...` verd runs `gen <seed>`, feeds the input to the Solution and to `brute`, and compares outputs with the Problem's Comparison Mode.
3. It stops at the first of:

| Stop | Result |
| --- | --- |
| Outputs differ | Counterexample: seed, input, both outputs. In the TUI it opens in the diff overlay. |
| Solution crashes or exceeds the time limit | `solution-re` / `solution-tle` with the input. |
| `gen` or `brute` fails, or either does not build | An error, not a verdict on your Solution. |
| 1000 iterations or 30 seconds | "No counterexample". Raise it with `--iter N` and `--time S`. |
| You press `esc` or hit `Ctrl-C` | Cancelled. |

4. In the diff overlay press `w` to save the failing input as the next Custom Test, with **brute's output as the expected answer**. Now `t` reproduces the bug on every run.

Seeds are deterministic, so the same buggy Solution reports the same counterexample every time.

`verd stress` exits `0` only if no counterexample was found.
