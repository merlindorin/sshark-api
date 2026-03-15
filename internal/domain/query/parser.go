package query

import (
	"strings"

	"github.com/alecthomas/participle/v2"
)

//nolint:gochecknoglobals // parser is stateless and read-only
var parser = participle.MustBuild[Query](
	participle.Lexer(queryLexer),
	participle.Elide("Whitespace"),
)

func Parse(input string) (*Query, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil //nolint:nilnil // nil query is valid for empty input
	}
	return parser.ParseString("", input)
}
