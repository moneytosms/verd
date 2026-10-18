# Stress testing

Your solution passes the samples and gets Wrong Answer on the judge? A stress test finds an input where it disagrees with a slow, obviously correct version.

## The idea

You provide two small programs next to your solution:

| File | Does |
| --- | --- |
| `gen.<ext>` | prints one random input; `argv[1]` is the seed |
| `brute.<ext>` | solves the problem slowly but surely |

verd runs seeds `1, 2, 3, ...`, feeds each generated input to your solution and to `brute`, and stops at the first disagreement.

## Try it

1. Press `S` in the Problem view (or run `verd stress main.cpp`). The first time, verd creates `gen` and `brute` from your Templates and opens them.
2. Fill them in. A generator for "read one integer":

   ```python
   import sys, random
   random.seed(int(sys.argv[1]))
   print(random.randint(1, 100))
   ```

3. Press `S` again. Progress shows live; a counterexample opens in the diff view with the seed, the input and both outputs.
4. Press `w` to save it as your next Custom Test (brute's output is the expected answer). Now `t` reproduces the bug every time.

```text
$ verd stress main.cpp
MISMATCH at seed 7 (iteration 7)
input:
37
brute:
74
solution:
75
$ echo $?
1
```

## Limits and results

By default it stops after 1000 iterations or 30 seconds. `--iter N` and `--time S` change that. A stress run ends with one of:

| Result | Meaning |
| --- | --- |
| mismatch | counterexample found (exit 1) |
| `solution-re` / `solution-tle` | your solution crashed or was too slow on that input (exit 1) |
| no counterexample | within the limits (exit 0) |
| error | `gen` or `brute` failed or did not build; not a verdict on your solution (exit 2) |

Seeds are deterministic: the same bug gives the same counterexample every time.

## Making it faster

Iterations already run in parallel (`--jobs N`, default one per CPU, at most 16). For tiny programs the cost is process startup, and two opt-in flags remove most of it:

```sh
verd stress --batch 100 main.cpp            # gen runs once per 100 seeds
verd stress --batch 100 --multi 100 main.cpp
```

- `--batch N`: `gen` is run as `gen <first seed> <N>` and prints N cases, each followed by a line `---`.
- `--multi N`: for problems with several test cases per input. Your solution and brute read `t` and then `t` cases, `gen` prints one case without `t`, and N cases go through one process pair. When a batch disagrees verd reruns its seeds one by one, so the reported seed is exact.

On trivial C++ programs: about 3.8k iterations/s by default, about 290k with both flags.

See [Testing and stress](../testing.md) for the details, and [Troubleshooting](./troubleshooting.md) if a build fails.

Next: [Submitting](./submit.md).
