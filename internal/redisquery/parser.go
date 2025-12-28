//nolint:govet // specific to participle way of managing tags
package redisquery

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// Query represents a parsed RediSearch query.
type Query struct {
	Terms []*Term `@@*`
}

// Term represents a single search term.
type Term struct {
	Negation bool       `@"-"?`
	Optional bool       `@"~"?`
	Field    *FieldExpr `( @@`
	Phrase   *string    `| @String`
	Fuzzy    *string    `| @Fuzzy`
	Group    *Group     `| @@`
	MatchAll bool       `| @"*"`
	Word     *string    `| @Word )`
	Or       *OrExpr    `@@?`
}

// FieldExpr represents a field search expression (@field:value or @field:{tag}).
type FieldExpr struct {
	Name string    `@Field ":"`
	Tag  *TagValue `( @@`
	Text *string   `| @(Word | String) )`
}

// TagValue represents a tag field value with optional OR: {val1|val2}.
type TagValue struct {
	Values []string `"{" @Word ( "|" @Word )* "}"`
}

// Group represents a grouped expression: (...)
type Group struct {
	Query *Query `"(" @@ ")"`
}

// OrExpr represents an OR expression: |term.
type OrExpr struct {
	Term *Term `"|" @@`
}

// Custom lexer for RediSearch queries.
func buildQueryLexer() lexer.Definition {
	return lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Fuzzy", Pattern: `%{1,3}[a-zA-Z0-9_-]+%{1,3}`},
		{Name: "String", Pattern: `"[^"]*"`},
		{Name: "Field", Pattern: `@[a-zA-Z_][a-zA-Z0-9_]*`},
		{Name: "Word", Pattern: `[a-zA-Z0-9_*-]+`},
		{Name: "Punct", Pattern: `[:{}()|~-]`},
		{Name: "Star", Pattern: `\*`},
		{Name: "Whitespace", Pattern: `\s+`},
	})
}

func newParser() *participle.Parser[Query] {
	return participle.MustBuild[Query](
		participle.Lexer(buildQueryLexer()),
		participle.Elide("Whitespace"),
		participle.UseLookahead(10),
	)
}

// Parse parses a RediSearch query string.
func Parse(s string) (*Query, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty query")
	}

	query, err := newParser().ParseString("", s)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return query, nil
}

// String converts the query back to RediSearch syntax.
func (q *Query) String() string {
	var parts []string
	for _, t := range q.Terms {
		parts = append(parts, t.String())
	}
	return strings.Join(parts, " ")
}

func (t *Term) String() string {
	var sb strings.Builder

	if t.Negation {
		sb.WriteString("-")
	}
	if t.Optional {
		sb.WriteString("~")
	}

	switch {
	case t.Field != nil:
		sb.WriteString(t.Field.String())
	case t.Phrase != nil:
		sb.WriteString(*t.Phrase)
	case t.Fuzzy != nil:
		sb.WriteString(*t.Fuzzy)
	case t.Group != nil:
		sb.WriteString("(")
		sb.WriteString(t.Group.Query.String())
		sb.WriteString(")")
	case t.MatchAll:
		sb.WriteString("*")
	case t.Word != nil:
		sb.WriteString(*t.Word)
	}

	if t.Or != nil {
		sb.WriteString("|")
		sb.WriteString(t.Or.Term.String())
	}

	return sb.String()
}

func (f *FieldExpr) String() string {
	var sb strings.Builder
	sb.WriteString(f.Name)
	sb.WriteString(":")

	if f.Tag != nil {
		sb.WriteString("{")
		sb.WriteString(strings.Join(f.Tag.Values, "|"))
		sb.WriteString("}")
	} else if f.Text != nil {
		sb.WriteString(*f.Text)
	}

	return sb.String()
}

// FieldName returns the field name without the @ prefix.
func (f *FieldExpr) FieldName() string {
	return strings.TrimPrefix(f.Name, "@")
}

// IsTagField returns true if this is a tag field search.
func (f *FieldExpr) IsTagField() bool {
	return f.Tag != nil
}
