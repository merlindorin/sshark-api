package query

import "github.com/alecthomas/participle/v2/lexer"

//nolint:gochecknoglobals // lexer is stateless and read-only
var queryLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "Escaped", Pattern: `\\[{}|&()@:\\*]`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_.-]*`},
	{Name: "Number", Pattern: `[0-9]+`},
	{Name: "Wildcard", Pattern: `\*`},
	{Name: "ValueChar", Pattern: `[@.:+]`},
	{Name: "At", Pattern: `@`},
	{Name: "Colon", Pattern: `:`},
	{Name: "LBrace", Pattern: `\{`},
	{Name: "RBrace", Pattern: `\}`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "Or", Pattern: `\|`},
	{Name: "And", Pattern: `&`},
})
