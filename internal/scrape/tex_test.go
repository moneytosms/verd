package scrape

import "testing"

func TestConvertTeX(t *testing.T) {
	for in, want := range map[string]string{
		`$$$1 \le n \le 2 \cdot 10^5$$$`:                    "1 ≤ n ≤ 2 · 10⁵",
		`$$$a_i$$$`:                                         "aᵢ",
		`$$$10^{-6}$$$`:                                     "10⁻⁶",
		`$$$1 \le t \le 10^4$$$`:                            "1 ≤ t ≤ 10⁴",
		`$$$\sum_{i=1}^{n} a_i$$$`:                          "Σᵢ₌₁ⁿ aᵢ",
		`$$$\lfloor \frac{n}{2} \rfloor$$$`:                 "⌊ n/2 ⌋",
		`$$$\frac{a+b}{2}$$$`:                               "(a+b)/2",
		`$$$a \oplus b$$$`:                                  "a ⊕ b",
		`$$$x_{i,j}$$$`:                                     "x_(i,j)",
		`$$$a \bmod 10^9+7$$$`:                              "a mod 10⁹+7",
		`$$$\sqrt{n}$$$ and $$$\text{gcd}(a, b)$$$`:         "√n and gcd(a, b)",
		`$$$a \ne b$$$, $$$\left( x \right)$$$ <b>keep</b>`: "a ≠ b, ( x ) <b>keep</b>",
		`no math here_ \ `:                                  `no math here_ \ `,
		`$$$ 2^{n} $$$`:                                     " 2ⁿ ",
	} {
		if got := ConvertTeX(in); got != want {
			t.Errorf("%s\n got  %q\n want %q", in, got, want)
		}
	}
}
