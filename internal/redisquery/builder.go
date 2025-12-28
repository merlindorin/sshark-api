package redisquery

import (
	"fmt"
	"strings"
)

// Builder provides a fluent API for building RediSearch queries.
type Builder struct {
	terms []string
}

// NewBuilder creates a new query builder.
func NewBuilder() *Builder {
	return &Builder{
		terms: make([]string, 0),
	}
}

// Build returns the final query string.
func (b *Builder) Build() string {
	if len(b.terms) == 0 {
		return "*"
	}
	return strings.Join(b.terms, " ")
}

// Query returns the parsed Query struct.
func (b *Builder) Query() (*Query, error) {
	return Parse(b.Build())
}

// MatchAll adds a match all query (*).
func (b *Builder) MatchAll() *Builder {
	b.terms = append(b.terms, "*")
	return b
}

// Term adds a simple text search term.
func (b *Builder) Term(value string) *Builder {
	if value != "" {
		b.terms = append(b.terms, escapeWord(value))
	}
	return b
}

// Phrase adds an exact phrase search ("...").
func (b *Builder) Phrase(value string) *Builder {
	if value != "" {
		b.terms = append(b.terms, fmt.Sprintf(`"%s"`, escapePhrase(value)))
	}
	return b
}

// Wildcard adds a wildcard search term (value*).
func (b *Builder) Wildcard(prefix string) *Builder {
	if prefix != "" {
		b.terms = append(b.terms, escapeWord(prefix)+"*")
	}
	return b
}

// Fuzzy adds a fuzzy search term with specified distance (1-3).
func (b *Builder) Fuzzy(value string, distance int) *Builder {
	if value == "" || distance < 1 {
		return b
	}
	if distance > 3 {
		distance = 3
	}
	pct := strings.Repeat("%", distance)
	b.terms = append(b.terms, fmt.Sprintf("%s%s%s", pct, escapeWord(value), pct))
	return b
}

// Not negates the next term.
func (b *Builder) Not() *TermBuilder {
	return &TermBuilder{builder: b, prefix: "-"}
}

// Optional marks the next term as optional.
func (b *Builder) Optional() *TermBuilder {
	return &TermBuilder{builder: b, prefix: "~"}
}

// Field starts a field-specific search.
func (b *Builder) Field(name string) *FieldBuilder {
	return &FieldBuilder{builder: b, name: name, prefix: ""}
}

// NotField starts a negated field search.
func (b *Builder) NotField(name string) *FieldBuilder {
	return &FieldBuilder{builder: b, name: name, prefix: "-"}
}

// Group adds a grouped sub-query.
func (b *Builder) Group(fn func(*Builder)) *Builder {
	sub := NewBuilder()
	fn(sub)
	if len(sub.terms) > 0 {
		b.terms = append(b.terms, fmt.Sprintf("(%s)", sub.Build()))
	}
	return b
}

// Or adds an OR expression between two sub-queries.
func (b *Builder) Or(left, right func(*Builder)) *Builder {
	leftBuilder := NewBuilder()
	rightBuilder := NewBuilder()
	left(leftBuilder)
	right(rightBuilder)

	if len(leftBuilder.terms) > 0 && len(rightBuilder.terms) > 0 {
		b.terms = append(b.terms, fmt.Sprintf("(%s|%s)", leftBuilder.Build(), rightBuilder.Build()))
	}
	return b
}

// Raw adds a raw query string (use with caution).
func (b *Builder) Raw(query string) *Builder {
	if query != "" {
		b.terms = append(b.terms, query)
	}
	return b
}

// TermBuilder handles prefixed terms (negation, optional).
type TermBuilder struct {
	builder *Builder
	prefix  string
}

func (t *TermBuilder) Term(value string) *Builder {
	if value != "" {
		t.builder.terms = append(t.builder.terms, t.prefix+escapeWord(value))
	}
	return t.builder
}

func (t *TermBuilder) Phrase(value string) *Builder {
	if value != "" {
		t.builder.terms = append(t.builder.terms, fmt.Sprintf(`%s"%s"`, t.prefix, escapePhrase(value)))
	}
	return t.builder
}

func (t *TermBuilder) Field(name string) *FieldBuilder {
	return &FieldBuilder{builder: t.builder, name: name, prefix: t.prefix}
}

// FieldBuilder handles field-specific searches.
type FieldBuilder struct {
	builder *Builder
	name    string
	prefix  string
}

// Eq searches for an exact text match in the field.
func (f *FieldBuilder) Eq(value string) *Builder {
	if value != "" {
		f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:%s", f.prefix, f.name, escapeWord(value)))
	}
	return f.builder
}

// Match searches for text containing the value.
func (f *FieldBuilder) Match(value string) *Builder {
	return f.Eq(value)
}

// Prefix searches for text starting with the value.
func (f *FieldBuilder) Prefix(value string) *Builder {
	if value != "" {
		f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:%s*", f.prefix, f.name, escapeWord(value)))
	}
	return f.builder
}

// Phrase searches for an exact phrase in the field.
func (f *FieldBuilder) Phrase(value string) *Builder {
	if value != "" {
		f.builder.terms = append(f.builder.terms, fmt.Sprintf(`%s@%s:"%s"`, f.prefix, f.name, escapePhrase(value)))
	}
	return f.builder
}

// Tag searches for exact tag value(s).
func (f *FieldBuilder) Tag(values ...string) *Builder {
	if len(values) == 0 {
		return f.builder
	}

	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = escapeTag(v)
	}

	f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:{%s}", f.prefix, f.name, strings.Join(escaped, "|")))
	return f.builder
}

// In is an alias for Tag (for readability).
func (f *FieldBuilder) In(values ...string) *Builder {
	return f.Tag(values...)
}

// Between searches for numeric range (requires NUMERIC field).
func (f *FieldBuilder) Between(min, max float64) *Builder { //nolint:revive,predeclared // we are not redefining
	f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:[%v %v]", f.prefix, f.name, min, max))
	return f.builder
}

// Gte searches for >= value (requires NUMERIC field).
func (f *FieldBuilder) Gte(value float64) *Builder {
	f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:[%v +inf]", f.prefix, f.name, value))
	return f.builder
}

// Lte searches for <= value (requires NUMERIC field).
func (f *FieldBuilder) Lte(value float64) *Builder {
	f.builder.terms = append(f.builder.terms, fmt.Sprintf("%s@%s:[-inf %v]", f.prefix, f.name, value))
	return f.builder
}

// escapeWord escapes special characters in a word.
func escapeWord(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`'`, `\'`,
		`(`, `\(`,
		`)`, `\)`,
		`{`, `\{`,
		`}`, `\}`,
		`[`, `\[`,
		`]`, `\]`,
		`:`, `\:`,
		`;`, `\;`,
		`!`, `\!`,
		`@`, `\@`,
		`#`, `\#`,
		`$`, `\$`,
		`%`, `\%`,
		`^`, `\^`,
		`&`, `\&`,
		`*`, `\*`,
		`-`, `\-`,
		`+`, `\+`,
		`=`, `\=`,
		`~`, `\~`,
		`|`, `\|`,
		`<`, `\<`,
		`>`, `\>`,
		`?`, `\?`,
		`/`, `\/`,
	)
	return replacer.Replace(s)
}

// escapePhrase escapes characters within a phrase (inside quotes).
func escapePhrase(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// escapeTag escapes characters within a tag value.
func escapeTag(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`|`, `\|`,
		`{`, `\{`,
		`}`, `\}`,
	)
	return replacer.Replace(s)
}
