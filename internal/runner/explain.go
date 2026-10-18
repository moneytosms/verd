package runner

import (
	"fmt"
	"regexp"
	"strings"
)

// Explanation is a plain-English reading of a failed run: what the error means and where in the
// Solution it happened ("" if unknown, else "file:line").
type Explanation struct{ Hint, Loc string }

// stderrHints are checked in order against stderr; the first match wins, so specific rules come first.
var stderrHints = []struct{ match, hint string }{
	{"Error: attempt to", "Container misuse caught by debug mode: index out of range, or front/back/pop on an empty container."},
	{"stack-overflow", "Stack overflow: recursion is too deep or never stops, or a huge local array. Check the base case; make big arrays global."},
	{"heap-buffer-overflow", "Read or write past the end of a heap array or vector. Check the index against the size."},
	{"stack-buffer-overflow", "Read or write past the end of a local array. Check the index against the size."},
	{"global-buffer-overflow", "Read or write past the end of a global array. Make it bigger or check the index."},
	{"use-after-free", "Using memory after it was freed (a dangling pointer or reference, e.g. into a vector that grew)."},
	{"use-after-return", "Using a local variable's address after its function returned."},
	{"double-free", "The same memory was freed twice."},
	{"SEGV on unknown address", "Invalid memory access: a bad index, null or dangling pointer, or an uninitialized vector."},
	{"signed integer overflow", "A signed int overflowed. Use long long (ll) for values that can exceed about 2e9."},
	{"shift exponent", "Shift by a negative amount or by >= the type's width. Use 1LL << k for k >= 31."},
	{"division by zero", "Division or modulo by zero."},
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
	cLoc  = regexp.MustCompile(`(?m)(?:\bin \S+ |\bat )?((?:/|\w)[^\s:()]*\.(?:cpp|cc|cxx|c|h|hpp)):(\d+)`)
	pyLoc = regexp.MustCompile(`File "([^"]+)", line (\d+)`)
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
	e := Explanation{Loc: location(r.Stderr)}
	for _, h := range stderrHints {
		if strings.Contains(r.Stderr, h.match) {
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

// location is the innermost Solution frame in stderr: the last Python traceback line, else the first
// C/C++ source location that is not a system header.
func location(stderr string) string {
	if all := pyLoc.FindAllStringSubmatch(stderr, -1); len(all) > 0 {
		m := all[len(all)-1]
		return m[1] + ":" + m[2]
	}
	for _, m := range cLoc.FindAllStringSubmatch(stderr, -1) {
		if !strings.HasPrefix(m[1], "/usr/") && !strings.HasPrefix(m[1], "/lib") {
			return m[1] + ":" + m[2]
		}
	}
	return ""
}
