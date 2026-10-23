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

### Going faster

Iterations run in parallel (`--jobs N`, default CPU count, max 16). Two opt-in flags cut process spawns further; the reported seed and verdicts are unchanged.

- `--batch N`: `gen` runs once per N seeds as `gen <first seed> <N>` and prints N cases, each followed by a line `---`.
- `--multi N`: for multi-test problems. Solution and brute read `t` then `t` cases, `gen` prints one case without `t`, and N cases go through one process pair. If a batch disagrees or fails, verd reruns its seeds one by one (input `1` then the case) to report the exact seed.

Trivial C++ programs, 16 threads: ~3.8k iterations/s default, ~5.4k with `--batch 100`, ~290k with `--batch 100 --multi 100` (`go test ./internal/stress -bench .`).

## Debugging

- `f` in the TUI opens a test full-screen: input, expected, stdout and stderr, for any verdict. `verd test <file> -v` prints the same for every test; `-t sample-2` runs one test.
- Failures get a `why:` line in plain English (segfault, division by zero, `std::out_of_range`, Python `IndexError`, ...) and an `at:` source location with the source line when stderr has one. Sanitizer reports are read for their numbers: `Index 4, but the container has size 4 (valid 0..3): one past the end`, the overflowing expression, or which local or global array was overrun. A `via:` line lists your own calls, innermost first, with recursion folded (`dfs:7 ← dfs:8 ×3 ← solve:19 ← main:21`).
- Sanitizer stderr is folded for reading: library stack frames, repeated recursive frames, libstdc++'s function signature and register or shadow-memory dumps each become one `⋯ N ...` line. In the full-screen test view `z` shows it as printed; `y` always copies the raw text, and `verd test -v` prints it unfolded.
- New C++ files include `debug.h` under `#ifdef LOCAL`. `debug(a, b)` prints `[L12] a = 5, b = [1, 2, 3]` to stderr for any value (containers, nested, pairs, tuples, maps, queues). Local test builds define `LOCAL`; on Codeforces `debug` compiles to nothing, so there is nothing to strip. C has `debug("%d", x)`; Python has `debug(f"{x=}", y)` (runs set the `LOCAL` environment variable).
- The same `debug(x)` handles 2D values: a matrix, grid, adjacency list or map of lists prints one row per line (`0: [1, 2]`), so there is no separate matrix or graph macro. On Codeforces `debug(...)` expands to `42`, as in tourist's template.
- verd writes `debug.h` into `~/.local/share/verd/include` and a `compile_flags.txt` into the Workspace root so clangd finds it. Re-run `verd init --force` to refresh your Templates (existing ones are never overwritten otherwise).

### Partial runs and failures

- `space` marks a test, `R` runs only the marked tests (or the selected one if none is marked), `F` reruns only the tests that did not pass. `verd test <file> -t sample-1,custom-2` does the same from the shell.
- When a test ends `WA` or `RE` and the compiler is `g++`/`gcc`, verd rebuilds the Solution once with `-fsanitize=address,undefined` and `-D_GLIBCXX_DEBUG` and reruns that test. If this finds a memory error, undefined behavior or an out-of-range container access, its message replaces stderr (marked `[verd: rerun with sanitizers]`) and the `why:` / `at:` lines point at your line. Verdicts are never changed by it. If the sanitizer build fails, verd carries on without it.
- A Custom Test whose `.ans` is empty is not judged: it shows `??` and the output, so it doubles as a scratch run.
- A failing line in the full view shows `want: ...` under it, and the header counts how many lines differ.

### Checker and validator (optional)

Drop a file next to the Solution in the same language and verd picks it up. Nothing to configure.

| File | Used by | Contract |
| --- | --- | --- |
| `checker.<ext>` | tests | run as `checker <input> <output> <answer>`; exit 0 accepts. Replaces the Comparison Mode, for problems with many valid answers. |
| `validator.<ext>` | stress | reads the generated input on stdin; non-zero exit stops the run with "validator rejected the generated input". |

`gen` also sees an environment variable `SIZE` that grows from 1 to 10 over the first 300 seeds, so the first counterexample is usually small. The default `gen` templates use it.
