package redisquery_test

import (
	"testing"

	"github.com/merlindorin/sshark-api/internal/redisquery"
)

func TestBuilder(t *testing.T) {
	tests := []struct {
		name     string
		build    func() string
		expected string
	}{
		// Basic terms
		{
			name:     "empty builder returns match all",
			build:    func() string { return redisquery.NewBuilder().Build() },
			expected: "*",
		},
		{
			name:     "match all",
			build:    func() string { return redisquery.NewBuilder().MatchAll().Build() },
			expected: "*",
		},
		{
			name:     "simple term",
			build:    func() string { return redisquery.NewBuilder().Term("hello").Build() },
			expected: "hello",
		},
		{
			name:     "multiple terms",
			build:    func() string { return redisquery.NewBuilder().Term("hello").Term("world").Build() },
			expected: "hello world",
		},
		{
			name:     "phrase",
			build:    func() string { return redisquery.NewBuilder().Phrase("hello world").Build() },
			expected: `"hello world"`,
		},
		{
			name:     "wildcard",
			build:    func() string { return redisquery.NewBuilder().Wildcard("hel").Build() },
			expected: "hel*",
		},
		{
			name:     "fuzzy distance 1",
			build:    func() string { return redisquery.NewBuilder().Fuzzy("hello", 1).Build() },
			expected: "%hello%",
		},
		{
			name:     "fuzzy distance 2",
			build:    func() string { return redisquery.NewBuilder().Fuzzy("hello", 2).Build() },
			expected: "%%hello%%",
		},

		// Negation
		{
			name:     "negation term",
			build:    func() string { return redisquery.NewBuilder().Not().Term("bad").Build() },
			expected: "-bad",
		},
		{
			name:     "negation phrase",
			build:    func() string { return redisquery.NewBuilder().Not().Phrase("bad thing").Build() },
			expected: `-"bad thing"`,
		},

		// Optional
		{
			name:     "optional term",
			build:    func() string { return redisquery.NewBuilder().Optional().Term("maybe").Build() },
			expected: "~maybe",
		},

		// Field searches
		{
			name:     "field eq",
			build:    func() string { return redisquery.NewBuilder().Field("username").Eq("john").Build() },
			expected: "@username:john",
		},
		{
			name:     "field prefix",
			build:    func() string { return redisquery.NewBuilder().Field("username").Prefix("joh").Build() },
			expected: "@username:joh*",
		},
		{
			name:     "field phrase",
			build:    func() string { return redisquery.NewBuilder().Field("title").Phrase("my key").Build() },
			expected: `@title:"my key"`,
		},
		{
			name:     "field tag single",
			build:    func() string { return redisquery.NewBuilder().Field("source").Tag("github").Build() },
			expected: "@source:{github}",
		},
		{
			name:     "field tag multiple",
			build:    func() string { return redisquery.NewBuilder().Field("source").Tag("github", "gitlab").Build() },
			expected: "@source:{github|gitlab}",
		},
		{
			name:     "field in (alias for tag)",
			build:    func() string { return redisquery.NewBuilder().Field("type").In("ssh-rsa", "ssh-ed25519").Build() },
			expected: "@type:{ssh-rsa|ssh-ed25519}",
		},

		// Negated fields
		{
			name:     "not field tag",
			build:    func() string { return redisquery.NewBuilder().NotField("source").Tag("gitlab").Build() },
			expected: "-@source:{gitlab}",
		},
		{
			name:     "not via term builder",
			build:    func() string { return redisquery.NewBuilder().Not().Field("type").Tag("ssh-rsa").Build() },
			expected: "-@type:{ssh-rsa}",
		},

		// Numeric ranges
		{
			name:     "field between",
			build:    func() string { return redisquery.NewBuilder().Field("age").Between(18, 65).Build() },
			expected: "@age:[18 65]",
		},
		{
			name:     "field gte",
			build:    func() string { return redisquery.NewBuilder().Field("score").Gte(100).Build() },
			expected: "@score:[100 +inf]",
		},
		{
			name:     "field lte",
			build:    func() string { return redisquery.NewBuilder().Field("score").Lte(50).Build() },
			expected: "@score:[-inf 50]",
		},

		// Grouping
		{
			name: "group",
			build: func() string {
				return redisquery.NewBuilder().Group(func(b *redisquery.Builder) {
					b.Term("hello").Term("world")
				}).Build()
			},
			expected: "(hello world)",
		},
		{
			name: "or",
			build: func() string {
				return redisquery.NewBuilder().Or(
					func(b *redisquery.Builder) { b.Field("source").Tag("github") },
					func(b *redisquery.Builder) { b.Field("source").Tag("gitlab") },
				).Build()
			},
			expected: "(@source:{github}|@source:{gitlab})",
		},

		// Complex queries
		{
			name: "complex query",
			build: func() string {
				return redisquery.NewBuilder().
					Field("username").Prefix("merlin").
					Field("source").Tag("github", "gitlab").
					NotField("type").Tag("ssh-rsa").
					Build()
			},
			expected: "@username:merlin* @source:{github|gitlab} -@type:{ssh-rsa}",
		},
		{
			name: "search with grouping",
			build: func() string {
				return redisquery.NewBuilder().
					Or(
						func(b *redisquery.Builder) { b.Field("source").Tag("github") },
						func(b *redisquery.Builder) { b.Field("source").Tag("gitlab") },
					).
					Field("username").Prefix("merlin").
					NotField("type").Tag("ssh-rsa").
					Build()
			},
			expected: "(@source:{github}|@source:{gitlab}) @username:merlin* -@type:{ssh-rsa}",
		},

		// Escaping
		{
			name:     "escape special chars in term",
			build:    func() string { return redisquery.NewBuilder().Term("hello-world").Build() },
			expected: `hello\-world`,
		},
		{
			name:     "escape quote in phrase",
			build:    func() string { return redisquery.NewBuilder().Phrase(`say "hello"`).Build() },
			expected: `"say \"hello\""`,
		},

		// Raw
		{
			name:     "raw query",
			build:    func() string { return redisquery.NewBuilder().Raw("@custom:[1 10]").Build() },
			expected: "@custom:[1 10]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.build()
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuilderQuery(t *testing.T) {
	// Test that built queries can be parsed back
	b := redisquery.NewBuilder().
		Field("source").Tag("github", "gitlab").
		Field("username").Prefix("merlin")

	query, err := b.Query()
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(query.Terms) != 2 {
		t.Errorf("expected 2 terms, got %d", len(query.Terms))
	}
}
