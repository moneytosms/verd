# verd

A terminal companion for Codeforces: find a problem, write a solution in Neovim, test it locally, submit it, and track progress without leaving the terminal.

## Language

**Problem**:
A single Codeforces task, identified by contest id and index (e.g. 1900A).
_Avoid_: Question, task, qn

**Sample Test**:
An input/expected-output pair published in a Problem's statement.
_Avoid_: Example, sample case

**Custom Test**:
An input/expected-output pair the user adds to a Problem themselves.
_Avoid_: User test, manual test

**Template**:
Per-language starter source a new Solution is created from.
_Avoid_: Boilerplate, snippet

**Solution**:
The user's source file for one Problem in one language.
_Avoid_: Code, attempt

**Workspace**:
The user-visible directory holding every Problem's Solutions and test files.
_Avoid_: Project dir, folder

**Test Run**:
One local execution of a Solution against a Problem's Sample Tests and Custom Tests.
_Avoid_: Check, judge run

**Local Verdict**:
Outcome of one test in a Test Run (AC, WA, TLE, RE, MLE, CE), decided on the user's machine.
_Avoid_: Result, status

**Comparison Mode**:
How a test's actual output is matched against its expected output (tokens, exact, float, none).
_Avoid_: Checker, diff mode

**Run Stats**:
Time and peak memory measured for each test in a Test Run.
_Avoid_: Metrics, perf

**Submission**:
A Solution sent to the Codeforces judge, with its judge-assigned Verdict.

**Verdict**:
The Codeforces judge's outcome for a Submission. Distinct from a Local Verdict.
_Avoid_: Result
_Avoid_: Attempt, upload

**Problem Picker**:
Chooses a random unsolved Problem matching user-given filters (rating range, tags).
_Avoid_: Recommender, randomizer

**Profile Stats**:
Aggregate view of a user's own Codeforces history: solved counts, topic strengths and weaknesses, rating progress.
_Avoid_: Metrics, analytics
