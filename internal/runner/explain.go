package runner

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Explanation is a plain-English reading of a failed run: what the error means, where in the
// Solution it happened ("" if unknown, else "file:line"), the source line there, and the Solution's
// own calls that led to it.
type Explanation struct {
	Hint, Loc string
	Code      string // the source line at Loc, trimmed; "" if it cannot be read
	Trace     string // innermost call first, recursion folded: "dfs:5 ← dfs:6 ×4 ← main:8"; "" for one frame
}

// specificHints read the numbers and names out of a sanitizer message. They are matched against
// stderr with whitespace collapsed (debug mode wraps its lines) and win over stderrHints; a rule
// returning "" defers to the generic hint.
var specificHints = []struct {
	re *regexp.Regexp
	fn func(m []string) string
}{
	{regexp.MustCompile(`out-of-bounds index (\d+), but container only holds (\d+) elements`), func(m []string) string {
		return indexHint(m[1], m[2], "container")
	}},
	{regexp.MustCompile(`index (-?\d+) out of bounds for type '[^']*\[(\d+)\]'`), func(m []string) string {
		return indexHint(m[1], m[2], "array")
	}},
	{regexp.MustCompile(`(?:READ|WRITE) of size (\d+) .*? located (\d+) bytes (after|before) (\d+)-byte region`), func(m []string) string {
		return memHint(m[1], m[2], m[3], m[4], "heap array")
	}},
	{regexp.MustCompile(`(?:READ|WRITE) of size (\d+) .*? located (\d+) bytes (after|before) global variable '(\w+)'.*? of size (\d+)`), func(m []string) string {
		return memHint(m[1], m[2], m[3], m[5], "global array '"+m[4]+"'")
	}},
	{regexp.MustCompile(`(?:READ|WRITE) of size (\d+) .*?\[(\d+), (\d+)\) '(\w+)'[^<]*<== Memory access at offset (\d+) (?:overflows|underflows)`), func(m []string) string {
		lo, _ := strconv.Atoi(m[2])
		hi, _ := strconv.Atoi(m[3])
		at, _ := strconv.Atoi(m[5])
		what := "local array '" + m[4] + "'"
		if at < lo {
			return memHint(m[1], strconv.Itoa(lo-at), "before", strconv.Itoa(hi-lo), what)
		}
		return memHint(m[1], strconv.Itoa(at-hi), "after", strconv.Itoa(hi-lo), what)
	}},
	{regexp.MustCompile(`signed integer overflow: (.+?) cannot be represented in type '([^']+)'`), func(m []string) string {
		return fmt.Sprintf("%s overflows %s. Use long long (ll) for values that can exceed about 2e9, and cast before multiplying (1LL * a * b).", m[1], m[2])
	}},
}

// stderrHints are checked in order against stderr; the first match wins, so specific rules come first.
// Debug-mode messages lead: its formatter can trip ASan afterwards, and that report is not the bug.
var stderrHints = []struct{ match, hint string }{
	{"comparison doesn't meet irreflexive", "The sort comparator is not a strict weak order: it must return false for equal elements. Use < or >, never <= or >=."},
	{"are not partitioned by", "Binary search (lower_bound, upper_bound, binary_search, equal_range) on a range that is not sorted by the order you search with. Sort it first."},
	{"are not sorted", "This call needs a sorted range (merge, includes, set_union and the like). Sort it first, with the same comparator."},
	{"attempt to access an element in an empty container", "front(), back() or pop on an empty container. Check !empty() first."},
	{"past-the-end iterator", "Used an end() iterator: dereferenced, incremented or erased it. Check it != end() first (after find, lower_bound, prev)."},
	{"singular iterator", "Used an invalid iterator: never set, or invalidated because the container grew or the element was erased."},
	{"from different sequences", "Mixed iterators of two different containers (often a copy passed by value)."},
	{"from a different container", "Mixed iterators of two different containers (often a copy passed by value)."},
	{"Error: attempt to", "Container misuse caught by debug mode: index out of range, or front/back/pop on an empty container."},
	{"stack-overflow", "Stack overflow: recursion is too deep or never stops, or a huge local array. Check the base case; make big arrays global."},
	{"heap-buffer-overflow", "Read or write past the end of a heap array or vector. Check the index against the size."},
	{"stack-buffer-overflow", "Read or write past the end of a local array. Check the index against the size."},
	{"global-buffer-overflow", "Read or write past the end of a global array. Make it bigger or check the index."},
	{"heap-use-after-free", "Using memory after it was freed: a pointer, reference or iterator into a vector that has since grown (push_back reallocates)."},
	{"use-after-free", "Using memory after it was freed (a dangling pointer or reference, e.g. into a vector that grew)."},
	{"use-after-return", "Using a local variable's address after its function returned."},
	{"double-free", "The same memory was freed twice."},
	{"SEGV on unknown address", "Invalid memory access: a bad index, null or dangling pointer, or an uninitialized vector."},
	{"signed integer overflow", "A signed int overflowed. Use long long (ll) for values that can exceed about 2e9."},
	{"shift exponent", "Shift by a negative amount or by >= the type's width. Use 1LL << k for k >= 31."},
	{"division by zero", "Division or modulo by zero."},
	{"insufficient space for an object", "Read or write past the end of an array or vector through a pointer. Check the index against the size."},
	{"out of bounds for type", "Array index out of range."},
	{"null pointer", "Dereferenced a null pointer."},
	{"runtime error:", "Undefined behavior was detected at runtime (see the line below)."},
	{"stack smashing detected", "A local array was overrun (index out of bounds), corrupting the stack."},
	{"double free or corruption", "Heap corruption, usually an out-of-bounds write earlier in the run, or freeing twice."},
	{"corrupted size vs. prev_size", "Heap corruption: an out-of-bounds write earlier in the run."},
	{"free(): invalid", "Heap corruption, usually an out-of-bounds write earlier in the run."},
	{"munmap_chunk(): invalid pointer", "Heap corruption, usually an out-of-bounds write earlier in the run."},
	{"Assertion", "An assert() failed: the condition printed above was false."},
	{"std::bad_alloc", "Tried to allocate too much memory (a huge vector size, often a negative or uninitialized n)."},
	{"std::length_error", "Created a vector or string with an impossible length (often a negative or uninitialized n)."},
	{"std::out_of_range", "Index out of range in .at(), substr, stoi or similar. Check the index against the size."},
	{"terminate called after throwing", "An exception was thrown and never caught (see the type and what() above)."},
	{"RecursionError", "Recursion too deep. Raise sys.setrecursionlimit or make it iterative."},
	{"IndexError", "List index out of range. Check the index against len()."},
	{"KeyError", "Dictionary key not found. Use .get() or check `in` first."},
	{"ZeroDivisionError", "Division or modulo by zero."},
	{"EOFError", "Read more input than exists. Check how many values you read versus the input format."},
	{"ValueError: not enough values", "Unpacking fewer values than expected: the line has fewer tokens than you split."},
	{"ValueError: too many values", "Unpacking more values than expected: the line has more tokens than you split."},
	{"ValueError: invalid literal", "Converting a non-number to int. Check how the input line is split."},
	{"MemoryError", "Out of memory: a structure is far bigger than the limit."},
	{"NameError", "Used a variable that was never defined (typo or wrong scope)."},
	{"TypeError", "An operation got the wrong type (e.g. mixing str and int)."},
}

var signalHints = map[string]string{
	"SIGSEGV": "Invalid memory access: a bad index, null or dangling pointer, uninitialized vector, or infinite recursion. Compile with -fsanitize=address,undefined to get the exact line.",
	"SIGFPE":  "Arithmetic error: division or modulo by zero (or INT_MIN / -1).",
	"SIGABRT": "Program aborted: failed assert, uncaught exception, or a detected memory error. See stderr.",
	"SIGBUS":  "Invalid memory access (bad alignment or mapping), usually an out-of-bounds pointer.",
	"SIGILL":  "Illegal instruction, often from running off the end of a non-void function or stack corruption.",
	"SIGKILL": "Killed by the OS, almost always out of memory.",
	"SIGTERM": "Terminated from outside the program.",
}

var (
	cLoc    = regexp.MustCompile(`(?m)(?:\bin \S+ |\bat )?((?:/|\w)[^\s:()]*\.(?:cpp|cc|cxx|c|h|hpp)):(\d+)`)
	pyLoc   = regexp.MustCompile(`File "([^"]+)", line (\d+)(?:, in (\S+))?`)
	frameRe = regexp.MustCompile(`^\s*#\d+ 0x[0-9a-f]+ +(?:in (.+) )?(\S+?)(?::(\d+))?(?::\d+)?(?: \(BuildId: \w+\))?$`)
)

// Explain says what a failed result means. The zero Explanation means nothing to add (AC, WA, ...).
func Explain(r Result) Explanation {
	switch r.Verdict {
	case TLE:
		return Explanation{Hint: "Too slow: an infinite loop, or complexity too high for the input size. Compare n against the time limit."}
	case MLE:
		return Explanation{Hint: "Memory over the limit: arrays or containers too big, or unbounded recursion."}
	case RE:
	case WA: // a wrong answer is explained only when the sanitizer rerun found the reason
		if !strings.Contains(r.Stderr, "rerun with sanitizers") {
			return Explanation{}
		}
	default:
		return Explanation{}
	}
	e := Explanation{Loc: location(r.Stderr), Trace: trace(r.Stderr)}
	e.Code = sourceLine(e.Loc)
	flat := strings.Join(strings.Fields(r.Stderr), " ")
	for _, h := range specificHints {
		if m := h.re.FindStringSubmatch(flat); m != nil {
			if e.Hint = h.fn(m); e.Hint != "" {
				return e
			}
		}
	}
	for _, h := range stderrHints {
		if strings.Contains(flat, h.match) {
			e.Hint = h.hint
			return e
		}
	}
	if h, ok := signalHints[r.Signal]; ok {
		e.Hint = h
	} else if r.ExitCode > 0 {
		e.Hint = fmt.Sprintf("Exited with code %d. main must return 0; check for exit(%d) or an uncaught error in stderr.", r.ExitCode, r.ExitCode)
	}
	return e
}

// indexHint explains index idx into what of n elements. Debug mode prints the index as a size_t, so
// a negative one arrives as a huge number.
func indexHint(idxS, nS, what string) string {
	n, _ := strconv.ParseInt(nS, 10, 64)
	idx, err := strconv.ParseInt(idxS, 10, 64)
	if err != nil {
		u, _ := strconv.ParseUint(idxS, 10, 64)
		idx = int64(u)
	}
	s := fmt.Sprintf("Index %d, but the %s has size %d", idx, what, n)
	switch {
	case n == 0:
		return s + ": it is empty. Was it sized (vector<int> v(n)) or filled before use?"
	case idx < 0:
		return s + fmt.Sprintf(" (valid 0..%d): a negative index, e.g. i-1 at i=0.", n-1)
	case idx == n:
		return s + fmt.Sprintf(" (valid 0..%d): one past the end. Off by one: a 1-based value used as a 0-based index, or a loop to <= n.", n-1)
	}
	return s + fmt.Sprintf(" (valid 0..%d): far past the end. Is it sized for the input (n+1, 2*n, the constraints)?", n-1)
}

// memHint turns an ASan report (an access of acc bytes, off bytes after or before a size-byte region)
// into an element index, taking elements to be acc bytes wide. "" if the sizes do not divide.
func memHint(accS, offS, side, sizeS, what string) string {
	acc, _ := strconv.Atoi(accS)
	off, _ := strconv.Atoi(offS)
	size, _ := strconv.Atoi(sizeS)
	if acc == 0 || size%acc != 0 || off%acc != 0 {
		return ""
	}
	idx := size/acc + off/acc
	if side == "before" {
		idx = -max(off/acc, 1)
	}
	return indexHint(strconv.Itoa(idx), strconv.Itoa(size/acc), what)
}

// location is the innermost Solution frame in stderr: the last Python traceback line, else the first
// C/C++ source location that is not a system header.
func location(stderr string) string {
	if all := pyLoc.FindAllStringSubmatch(stderr, -1); len(all) > 0 {
		m := all[len(all)-1]
		return m[1] + ":" + m[2]
	}
	all := cLoc.FindAllStringSubmatch(stderr, -1)
	for _, m := range all {
		if system(m[1]) {
			continue
		}
		if !strings.HasPrefix(m[1], "/") { // UBSan names the file as compiled; a stack frame has the full path
			for _, f := range all {
				if strings.HasSuffix(f[1], "/"+m[1]) && f[2] == m[2] {
					return f[1] + ":" + f[2]
				}
			}
		}
		return m[1] + ":" + m[2]
	}
	return ""
}

func system(path string) bool {
	return strings.HasPrefix(path, "/usr/") || strings.HasPrefix(path, "/lib")
}

// sourceLine is the trimmed text of loc ("file:line"), or "".
func sourceLine(loc string) string {
	i := strings.LastIndexByte(loc, ':')
	if i < 0 {
		return ""
	}
	n, err := strconv.Atoi(loc[i+1:])
	b, rerr := os.ReadFile(loc[:i])
	if err != nil || rerr != nil || n < 1 {
		return ""
	}
	if ls := strings.Split(string(b), "\n"); n <= len(ls) {
		return strings.TrimSpace(ls[n-1])
	}
	return ""
}

// trace is the Solution's own calls in the first stack in stderr, innermost first, with repeats of
// the same call folded ("dfs:6 ×4"). "" when there is at most one such call: Loc already says it.
func trace(stderr string) string {
	type call struct{ fn, line string }
	var calls []call
	if py := pyLoc.FindAllStringSubmatch(stderr, -1); len(py) > 0 {
		for i := len(py) - 1; i >= 0; i-- { // Python prints outermost first
			calls = append(calls, call{py[i][3], py[i][2]})
		}
	} else {
		started := false
		for _, l := range strings.Split(stderr, "\n") {
			m := frameRe.FindStringSubmatch(l)
			if m == nil {
				if started {
					break // only the first stack: later ones say where memory was allocated or freed
				}
				continue
			}
			started = true
			if m[3] != "" && !system(m[2]) {
				calls = append(calls, call{funcName(m[1]), m[3]})
			}
		}
	}
	if len(calls) < 2 {
		return ""
	}
	var parts []string
	for i := 0; i < len(calls); {
		j := i
		for j < len(calls) && calls[j] == calls[i] {
			j++
		}
		p := calls[i].fn + ":" + calls[i].line
		if j-i > 1 {
			p += fmt.Sprintf(" ×%d", j-i)
		}
		parts = append(parts, p)
		i = j
	}
	return strings.Join(parts, " ← ")
}

// funcName shortens a demangled C++ name to what the Solution wrote: "dfs(int, ...)" is dfs and
// "main::{lambda(int, int)#1}::operator()(...)" is lambda.
func funcName(s string) string {
	if strings.Contains(s, "{lambda") {
		return "lambda"
	}
	s, _, _ = strings.Cut(s, "(")
	if s == "" {
		return "?"
	}
	return s
}

var (
	libHeader = regexp.MustCompile(`^/usr/\S+:\d+:$`)
	regLine   = regexp.MustCompile(`^\s*r\w+ = 0x`)
	dropLine  = regexp.MustCompile(`^(=+|AddressSanitizer:DEADLYSIGNAL|==\d+==ABORTING|HINT: this may be a false positive.*|\s+\(longjmp and C\+\+ exceptions \*are\* supported\)|.*AddressSanitizer: ABRT.*)$`)
)

// Fold shortens sanitizer stderr for reading. Library stack frames, repeats of a recursive frame,
// libstdc++'s function signature and object dump, and register and shadow-memory dumps each collapse
// into one "⋯ N ..." line; banners and the ABRT report (the abort is explained above it) are dropped.
// Anything else is kept as is.
func Fold(stderr string) string {
	var out, run []string
	label, block, prev := "", "", ""
	flush := func() {
		if len(run) == 1 {
			out = append(out, run[0])
		} else if len(run) > 1 {
			out = append(out, fmt.Sprintf("⋯ %d %s", len(run), label))
		}
		run = nil
	}
	for _, l := range strings.Split(stderr, "\n") {
		switch {
		case block == "sig" && strings.TrimSpace(l) == "",
			block == "objects" && !strings.HasPrefix(l, " "),
			block == "shadow" && strings.HasPrefix(l, "=="),
			block == "regs" && !regLine.MatchString(l) && !strings.HasPrefix(l, "AddressSanitizer can not"):
			block = ""
		}
		switch {
		case libHeader.MatchString(l) || l == "In function:":
			block = "sig"
		case l == "Objects involved in the operation:":
			block = "objects"
		case strings.HasPrefix(l, "Shadow bytes around"):
			block = "shadow"
		case strings.HasSuffix(l, "Register values:"):
			block = "regs"
		}
		lab := map[string]string{"sig": "lines of library function signature", "objects": "lines of debug-mode object details",
			"shadow": "lines of shadow memory", "regs": "lines of register dump"}[block]
		if m := frameRe.FindStringSubmatch(l); m != nil && block == "" {
			key := funcName(m[1]) + " " + m[2] + ":" + m[3]
			switch {
			case m[3] == "" || system(m[2]):
				lab = "library frames"
			case key == prev:
				lab = "repeats of the frame above (recursion)"
			}
			prev = key
		} else if block == "" {
			prev = ""
		}
		if block == "" && dropLine.MatchString(l) {
			continue
		}
		if lab != label {
			flush()
			label = lab
		}
		if lab == "" {
			out = append(out, l)
		} else {
			run = append(run, l)
		}
	}
	flush()
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}
