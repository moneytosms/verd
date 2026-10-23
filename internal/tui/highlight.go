package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/moneytosms/verd/internal/theme"
)

// hlCode colors one line of source with the theme's styles, picking the lexer from the file name.
// A language chroma does not know comes back plain.
func hlCode(st theme.Styles, file, code string) string {
	lx := lexers.Match(file)
	if lx == nil {
		return code
	}
	it, err := chroma.Coalesce(lx).Tokenise(nil, code)
	if err != nil {
		return code
	}
	var b strings.Builder
	for _, t := range it.Tokens() {
		s := st.Fg()
		switch {
		case t.Type.InCategory(chroma.Comment):
			s = st.Dim
		case t.Type.InCategory(chroma.Keyword):
			s = st.Accent2
		case t.Type.InSubCategory(chroma.LiteralString):
			s = st.Good
		case t.Type.InSubCategory(chroma.LiteralNumber):
			s = st.Warn
		case t.Type == chroma.NameFunction:
			s = st.Accent
		}
		b.WriteString(s.Render(t.Value))
	}
	return b.String()
}
