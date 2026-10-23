package scrape

import (
	"regexp"
	"strings"
)

var multiSpace = regexp.MustCompile(` {2,}`)
var texSpan = regexp.MustCompile(`(?s)\$\$\$(.+?)\$\$\$`)

// ConvertTeX replaces every $$$...$$$ span in s with a Unicode rendering of its LaTeX.
// Run it on the raw HTML before HTML-to-markdown, which would escape `_` and `\`.
func ConvertTeX(s string) string {
	return convertTeX(s, unicodeMath)
}

// unicodeMath is the Unicode rendering of one LaTeX span (without its $$$ marks).
func unicodeMath(latex string) string {
	return multiSpace.ReplaceAllString(tex(latex), " ")
}

// convertTeX replaces each $$$...$$$ span in s with conv of its LaTeX source.
func convertTeX(s string, conv func(latex string) string) string {
	return texSpan.ReplaceAllStringFunc(s, func(m string) string {
		return conv(m[3 : len(m)-3])
	})
}

var symbols = map[string]string{
	"le": "≤", "leq": "≤", "ge": "≥", "geq": "≥", "ne": "≠", "neq": "≠", "cdot": "·", "times": "×",
	"dots": "…", "ldots": "…", "cdots": "…", "oplus": "⊕", "in": "∈", "to": "→", "rightarrow": "→",
	"leftarrow": "←", "sum": "Σ", "prod": "Π", "infty": "∞", "pm": "±", "lfloor": "⌊", "rfloor": "⌋",
	"lceil": "⌈", "rceil": "⌉", "bmod": " mod ", "mod": " mod ", "mid": "∣", "equiv": "≡", "approx": "≈",
	"lt": "<", "gt": ">", "land": "∧", "lor": "∨", "neg": "¬", "ell": "ℓ", "cap": "∩", "cup": "∪",
	"subset": "⊂", "subseteq": "⊆", "emptyset": "∅", "forall": "∀", "exists": "∃", "div": "÷",
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε", "theta": "θ",
	"lambda": "λ", "mu": "μ", "pi": "π", "sigma": "σ", "phi": "φ", "omega": "ω", "Delta": "Δ", "Sigma": "Σ",
	"Omega": "Ω", "log": "log", "min": "min", "max": "max", "gcd": "gcd", "lcm": "lcm", "binom": "C",
	"%": "%", "{": "{", "}": "}", "&": "&", "#": "#", "$": "$", "_": "_",
	",": " ", ";": " ", ":": " ", "!": "", " ": " ", "quad": "  ", "qquad": "    ",
}

// commands whose braced argument is passed through unchanged
var passthrough = map[string]bool{"text": true, "mathrm": true, "mathbf": true, "mathit": true, "operatorname": true,
	"textbf": true, "textit": true, "mathcal": true, "bf": true, "rm": true, "boldsymbol": true, "mathbb": true, "underline": true}

const (
	supFrom = "0123456789+-=()abcdefghijklmnoprstuvwxyz"
	supTo   = "⁰¹²³⁴⁵⁶⁷⁸⁹⁺⁻⁼⁽⁾ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻ"
	subFrom = "0123456789+-=()aehijklmnoprstuvx"
	subTo   = "₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎ₐₑₕᵢⱼₖₗₘₙₒₚᵣₛₜᵤᵥₓ"
)

func script(s, from, to string, mark string) string {
	f, t := []rune(from), []rune(to)
	var b strings.Builder
	for _, r := range s {
		i := indexRune(f, r)
		if i < 0 {
			if len([]rune(s)) == 1 {
				return mark + s
			}
			return mark + "(" + s + ")"
		}
		b.WriteRune(t[i])
	}
	return b.String()
}

func indexRune(rs []rune, r rune) int {
	for i, x := range rs {
		if x == r {
			return i
		}
	}
	return -1
}

// tex converts one LaTeX fragment. Not a TeX engine: covers what Codeforces statements use.
func tex(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); {
		switch c := r[i]; c {
		case '{', '}':
			i++
		case '^', '_':
			i = texScript(r, i, &b)
		case '\\':
			i = texCommand(r, i, &b)
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

// texScript writes the superscript or subscript at r[i] and returns the index after its argument.
func texScript(r []rune, i int, b *strings.Builder) int {
	a, j := readArg(r, i+1)
	a = tex(a)
	if r[i] == '^' {
		b.WriteString(script(a, supFrom, supTo, "^"))
	} else {
		b.WriteString(script(a, subFrom, subTo, "_"))
	}
	return j
}

// texCommand writes the command at the backslash r[i] and returns the index after it.
func texCommand(r []rune, i int, b *strings.Builder) int {
	j := cmdEnd(r, i)
	name := string(r[i+1 : j])
	switch name {
	case "left", "right":
		if j < len(r) && r[j] == '.' {
			j++
		}
		return j
	case "frac", "dfrac":
		n, d, k := readTwo(r, j)
		b.WriteString(paren(tex(n)) + "/" + paren(tex(d)))
		return k
	case "binom":
		n, d, k := readTwo(r, j)
		b.WriteString("C(" + tex(n) + ", " + tex(d) + ")")
		return k
	case "sqrt":
		a, k := readArg(r, j)
		b.WriteString("√" + paren(tex(a)))
		return k
	case "color":
		_, k := readArg(r, j)
		return skipSpace(r, k)
	case "textcolor":
		_, a, k := readTwo(r, j)
		b.WriteString(tex(a))
		return k
	}
	if passthrough[name] {
		a, k := readArg(r, j)
		b.WriteString(tex(a))
		return k
	}
	if v, ok := symbols[name]; ok {
		b.WriteString(v)
	} else {
		b.WriteString(name) // unknown command: keep its name, drop the backslash
	}
	return j
}

// readArg reads one argument starting at r[i]: a {group}, a \command, or a single rune.
func readArg(r []rune, i int) (string, int) {
	i = skipSpace(r, i)
	if i >= len(r) {
		return "", i
	}
	switch r[i] {
	case '{':
		return readGroup(r, i)
	case '\\':
		j := cmdEnd(r, i)
		return string(r[i:j]), j
	}
	return string(r[i]), i + 1
}

// readTwo reads two arguments in a row starting at r[i].
func readTwo(r []rune, i int) (string, string, int) {
	a, j := readArg(r, i)
	b, k := readArg(r, j)
	return a, b, k
}

// readGroup reads the {group} starting at r[i] and returns its contents.
func readGroup(r []rune, i int) (string, int) {
	depth, j := 1, i+1
	for ; j < len(r) && depth > 0; j++ {
		switch r[j] {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return string(r[i+1 : j-1]), j
}

// cmdEnd returns the index just past the command whose backslash is r[i].
func cmdEnd(r []rune, i int) int {
	j := i + 1
	for j < len(r) && isLetter(r[j]) {
		j++
	}
	if j == i+1 && j < len(r) {
		j++
	}
	return j
}

func skipSpace(r []rune, i int) int {
	for i < len(r) && r[i] == ' ' {
		i++
	}
	return i
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// paren wraps s unless it is a single token.
func paren(s string) string {
	if len([]rune(s)) <= 1 || !strings.ContainsAny(s, " +-·×/≤≥") {
		return s
	}
	return "(" + s + ")"
}
