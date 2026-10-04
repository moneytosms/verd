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
	return texSpan.ReplaceAllStringFunc(s, func(m string) string {
		return multiSpace.ReplaceAllString(tex(m[3:len(m)-3]), " ")
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
	"textbf": true, "textit": true, "mathcal": true, "bf": true, "rm": true, "boldsymbol": true, "mathbb": true}

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
	// arg reads one argument starting at r[i]: a {group}, a \command, or a single rune.
	arg := func(i int) (string, int) {
		for i < len(r) && r[i] == ' ' {
			i++
		}
		if i >= len(r) {
			return "", i
		}
		switch r[i] {
		case '{':
			depth, j := 1, i+1
			for ; j < len(r) && depth > 0; j++ {
				if r[j] == '{' {
					depth++
				} else if r[j] == '}' {
					depth--
				}
			}
			return string(r[i+1 : j-1]), j
		case '\\':
			j := i + 1
			for j < len(r) && isLetter(r[j]) {
				j++
			}
			if j == i+1 && j < len(r) {
				j++
			}
			return string(r[i:j]), j
		}
		return string(r[i]), i + 1
	}
	for i := 0; i < len(r); {
		switch c := r[i]; c {
		case '{', '}':
			i++
		case '^', '_':
			a, j := arg(i + 1)
			a = tex(a)
			if c == '^' {
				b.WriteString(script(a, supFrom, supTo, "^"))
			} else {
				b.WriteString(script(a, subFrom, subTo, "_"))
			}
			i = j
		case '\\':
			j := i + 1
			for j < len(r) && isLetter(r[j]) {
				j++
			}
			if j == i+1 && j < len(r) {
				j++
			}
			name := string(r[i+1 : j])
			i = j
			switch {
			case name == "left" || name == "right":
				if i < len(r) && r[i] == '.' {
					i++
				}
			case name == "frac" || name == "dfrac":
				n, k := arg(i)
				d, k := arg(k)
				i = k
				b.WriteString(paren(tex(n)) + "/" + paren(tex(d)))
			case name == "binom":
				n, k := arg(i)
				d, k := arg(k)
				i = k
				b.WriteString("C(" + tex(n) + ", " + tex(d) + ")")
			case name == "sqrt":
				a, k := arg(i)
				i = k
				b.WriteString("√" + paren(tex(a)))
			case passthrough[name]:
				a, k := arg(i)
				i = k
				b.WriteString(tex(a))
			default:
				if v, ok := symbols[name]; ok {
					b.WriteString(v)
				} else {
					b.WriteString(name) // unknown command: keep its name, drop the backslash
				}
			}
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// paren wraps s unless it is a single token.
func paren(s string) string {
	if len([]rune(s)) <= 1 || !strings.ContainsAny(s, " +-·×/≤≥") {
		return s
	}
	return "(" + s + ")"
}
