//nolint:testpackage // testing internal builder behavior
package postgres

import (
	"testing"

	"github.com/merlindorin/sshark-api/internal/domain/query"
)

func TestBuilder_NilQuery(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	clause, args, err := builder.Build(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clause != "" {
		t.Errorf("expected empty clause, got %q", clause)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %d", len(args))
	}
}

func TestBuilder_SimpleField(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, err := query.Parse("@source.username:{torvalds}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "s.username = $1"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 1 || args[0] != "torvalds" {
		t.Errorf("expected args ['torvalds'], got %v", args)
	}
}

func TestBuilder_WildcardPrefix(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@source.username:{foo*}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "s.username LIKE $1"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 1 || args[0] != "foo%" {
		t.Errorf("expected args ['foo%%'], got %v", args)
	}
}

func TestBuilder_WildcardBoth(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@source.username:{*bar*}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "s.username LIKE $1"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 1 || args[0] != "%bar%" {
		t.Errorf("expected args ['%%bar%%'], got %v", args)
	}
}

func TestBuilder_OrExpression(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@source.username:{foo} | @source.provider:{bar}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "(s.username = $1 OR s.provider = $2)"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
	if args[0] != "foo" || args[1] != "bar" {
		t.Errorf("expected args ['foo', 'bar'], got %v", args)
	}
}

func TestBuilder_AndExpression(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@source.username:{foo} & @source.provider:{bar}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "(s.username = $1 AND s.provider = $2)"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuilder_GroupedExpression(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@source.username:{foo} & (@source.provider:{github} | @source.provider:{gitlab})")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "(s.username = $1 AND (s.provider = $2 OR s.provider = $3))"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuilder_UnknownField(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, _ := query.Parse("@unknown:{value}")
	_, _, err := builder.Build(q)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if err.Error() != "unknown field: @unknown" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuilder_GPGArrayField(t *testing.T) {
	builder := NewBuilder(GPGFieldMapping)

	q, _ := query.Parse("@user_ids:{user@example.com}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "$1 = ANY(gm.user_ids)"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 1 || args[0] != "user@example.com" {
		t.Errorf("expected args ['user@example.com'], got %v", args)
	}
}

func TestBuilder_GPGArrayFieldWildcard(t *testing.T) {
	builder := NewBuilder(GPGFieldMapping)

	q, _ := query.Parse("@user_ids:{*@kernel.org}")
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "EXISTS (SELECT 1 FROM unnest(gm.user_ids) AS elem WHERE elem LIKE $1)"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 1 || args[0] != "%@kernel.org" {
		t.Errorf("expected args ['%%@kernel.org'], got %v", args)
	}
}

func TestBuilder_AllSSHFields(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	tests := []struct {
		input          string
		expectedColumn string
	}{
		{"@source.username:{test}", "s.username = $1"},
		{"@source.provider:{github}", "s.provider = $1"},
		{"@algorithm:{ed25519}", "sm.algorithm = $1"},
		{"@fingerprint:{SHA256:abc}", "pk.fingerprint = $1"},
		{"@comment:{mykey}", "sm.comment = $1"},
		{"@key_bits:{256}", "sm.key_bits = $1"},
	}

	for _, tc := range tests {
		q, err := query.Parse(tc.input)
		if err != nil {
			t.Errorf("parse error for %q: %v", tc.input, err)
			continue
		}

		clause, _, err := builder.Build(q)
		if err != nil {
			t.Errorf("build error for %q: %v", tc.input, err)
			continue
		}

		if clause != tc.expectedColumn {
			t.Errorf("for %q: expected %q, got %q", tc.input, tc.expectedColumn, clause)
		}
	}
}

func TestBuilder_DottedFieldNames(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q, err := query.Parse("@source.username:{torvalds} & @source.provider:{github}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "(s.username = $1 AND s.provider = $2)"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
	if args[0] != "torvalds" || args[1] != "github" {
		t.Errorf("expected args ['torvalds', 'github'], got %v", args)
	}
}

func TestBuilder_ComplexQuery(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	input := "@source.username:{linus*} & @algorithm:{ed25519} & " +
		"(@source.provider:{github} | @source.provider:{gitlab})"
	q, _ := query.Parse(input)
	clause, args, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	expected := "(s.username LIKE $1 AND sm.algorithm = $2 AND (s.provider = $3 OR s.provider = $4))"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(args) != 4 {
		t.Errorf("expected 4 args, got %d", len(args))
	}
	if args[0] != "linus%" {
		t.Errorf("expected first arg 'linus%%', got %v", args[0])
	}
}

func TestBuilder_SQLInjection_LexerRejectsMaliciousInput(t *testing.T) {
	maliciousInputs := []struct {
		name  string
		input string
	}{
		{"single quote", `@source.username:{admin'--}`},
		{"double quote", `@source.username:{admin"}`},
		{"semicolon", `@source.username:{admin;DROP TABLE}`},
		{"OR injection", `@source.username:{x' OR '1'='1}`},
		{"SQL comment block", `@source.username:{admin/*comment*/}`},
		{"UNION injection", `@source.username:{' UNION SELECT}`},
		{"backslash without escape", `@source.username:{admin\x}`},
		{"backtick", "@source.username:{admin`id`}"},
		{"dollar sign", `@source.username:{$variable}`},
		{"equals sign", `@source.username:{1=1}`},
		{"less than", `@source.username:{1<2}`},
		{"greater than", `@source.username:{1>0}`},
		{"parentheses", `@source.username:{admin()}`},
		{"square brackets", `@source.username:{admin[0]}`},
		{"percent in non-wildcard", `@source.username:{100%}`},
		{"null byte", "@source.username:{admin\x00}"},
	}

	for _, tc := range maliciousInputs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := query.Parse(tc.input)
			if err == nil {
				t.Errorf("lexer should reject malicious input: %q", tc.input)
			}
		})
	}
}

func TestBuilder_SQLInjection_ValuesAreParameterized(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	tests := []struct {
		name         string
		input        string
		expectedArg  string
		clauseMustBe string
	}{
		{
			name:         "escaped braces are unescaped in value",
			input:        `@source.username:{admin\{test\}}`,
			expectedArg:  "admin{test}",
			clauseMustBe: "s.username = $1",
		},
		{
			name:         "escaped pipe in value",
			input:        `@source.username:{foo\|bar}`,
			expectedArg:  "foo|bar",
			clauseMustBe: "s.username = $1",
		},
		{
			name:         "escaped ampersand in value",
			input:        `@source.username:{foo\&bar}`,
			expectedArg:  "foo&bar",
			clauseMustBe: "s.username = $1",
		},
		{
			name:         "value with special allowed chars",
			input:        `@source.username:{user@domain.com}`,
			expectedArg:  "user@domain.com",
			clauseMustBe: "s.username = $1",
		},
		{
			name:         "value with colon",
			input:        `@fingerprint:{SHA256:abcdef123456}`,
			expectedArg:  "SHA256:abcdef123456",
			clauseMustBe: "pk.fingerprint = $1",
		},
		{
			name:         "SQL comment line syntax is just a value",
			input:        `@source.username:{admin--comment}`,
			expectedArg:  "admin--comment",
			clauseMustBe: "s.username = $1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q, err := query.Parse(tc.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			clause, args, err := builder.Build(q)
			if err != nil {
				t.Fatalf("build error: %v", err)
			}

			if clause != tc.clauseMustBe {
				t.Errorf("clause should use parameter placeholder, got %q", clause)
			}

			if len(args) != 1 {
				t.Fatalf("expected 1 arg, got %d", len(args))
			}

			if args[0] != tc.expectedArg {
				t.Errorf("expected arg %q, got %q", tc.expectedArg, args[0])
			}
		})
	}
}

func TestBuilder_SQLInjection_FieldNameWhitelisted(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	injectionAttempts := []string{
		"@users; DROP TABLE--:{value}",
		"@1=1 OR username:{value}",
	}

	for _, input := range injectionAttempts {
		_, err := query.Parse(input)
		if err == nil {
			t.Errorf("parser should reject malformed field name: %q", input)
		}
	}

	_, _, err := builder.Build(mustParse(t, "@nonexistent:{value}"))
	if err == nil {
		t.Error("builder should reject unknown field names")
	}
}

func TestBuilder_SQLInjection_NoInterpolation(t *testing.T) {
	builder := NewBuilder(SSHFieldMapping)

	q := mustParse(t, "@source.username:{test}")
	clause, _, err := builder.Build(q)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}

	if clause == "s.username = 'test'" {
		t.Error("value should not be interpolated directly into SQL")
	}

	if clause != "s.username = $1" {
		t.Errorf("expected parameterized query, got %q", clause)
	}
}

func mustParse(t *testing.T, input string) *query.Query {
	t.Helper()
	q, err := query.Parse(input)
	if err != nil {
		t.Fatalf("parse error for %q: %v", input, err)
	}
	return q
}
