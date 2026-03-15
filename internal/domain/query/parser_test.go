//nolint:testpackage // testing internal parser behavior
package query

import (
	"testing"
)

func TestParse_EmptyQuery(t *testing.T) {
	q, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q != nil {
		t.Fatal("expected nil query for empty input")
	}
}

func TestParse_SimpleField(t *testing.T) {
	q, err := Parse("@user:{torvalds}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q == nil || q.Expression == nil ||
		q.Expression.Left == nil || q.Expression.Left.Left == nil ||
		q.Expression.Left.Left.Field == nil {
		t.Fatal("expected non-nil field")
	}

	field := q.Expression.Left.Left.Field
	if field.Name != "user" {
		t.Errorf("expected field name 'user', got '%s'", field.Name)
	}
	if field.Value.String() != "torvalds" {
		t.Errorf("expected value 'torvalds', got '%s'", field.Value.String())
	}
}

func TestParse_HyphenInValue(t *testing.T) {
	q, err := Parse("@user:{foo-bar}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "foo-bar" {
		t.Errorf("expected value 'foo-bar', got '%s'", field.Value.String())
	}
}

func TestParse_EscapedBraces(t *testing.T) {
	q, err := Parse(`@user:{foo\{bar\}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "foo{bar}" {
		t.Errorf("expected value 'foo{bar}', got '%s'", field.Value.String())
	}
}

func TestParse_EscapedPipe(t *testing.T) {
	q, err := Parse(`@user:{foo\|bar}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "foo|bar" {
		t.Errorf("expected value 'foo|bar', got '%s'", field.Value.String())
	}
}

func TestParse_EscapedWildcard(t *testing.T) {
	q, err := Parse(`@user:{foo\*bar}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "foo*bar" {
		t.Errorf("expected value 'foo*bar', got '%s'", field.Value.String())
	}
	if field.Value.HasWildcard() {
		t.Error("escaped wildcard should not be treated as wildcard")
	}
}

func TestParse_WildcardPrefix(t *testing.T) {
	q, err := Parse("@user:{foo*}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "foo*" {
		t.Errorf("expected value 'foo*', got '%s'", field.Value.String())
	}
	if !field.Value.HasWildcard() {
		t.Error("expected wildcard")
	}
}

func TestParse_WildcardSuffix(t *testing.T) {
	q, err := Parse("@user:{*bar}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "*bar" {
		t.Errorf("expected value '*bar', got '%s'", field.Value.String())
	}
	if !field.Value.HasWildcard() {
		t.Error("expected wildcard")
	}
}

func TestParse_WildcardBoth(t *testing.T) {
	q, err := Parse("@user:{*bar*}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Value.String() != "*bar*" {
		t.Errorf("expected value '*bar*', got '%s'", field.Value.String())
	}
}

func TestParse_OrExpression(t *testing.T) {
	q, err := Parse("@user:{foo} | @provider:{bar}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Right) != 1 {
		t.Fatalf("expected 1 OR operand, got %d", len(q.Expression.Right))
	}

	leftField := q.Expression.Left.Left.Field
	if leftField.Name != "user" || leftField.Value.String() != "foo" {
		t.Errorf("unexpected left field: %s:{%s}", leftField.Name, leftField.Value.String())
	}

	rightField := q.Expression.Right[0].Left.Field
	if rightField.Name != "provider" || rightField.Value.String() != "bar" {
		t.Errorf("unexpected right field: %s:{%s}", rightField.Name, rightField.Value.String())
	}
}

func TestParse_AndExpression(t *testing.T) {
	q, err := Parse("@user:{foo} & @provider:{bar}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Left.Right) != 1 {
		t.Fatalf("expected 1 AND operand, got %d", len(q.Expression.Left.Right))
	}

	leftField := q.Expression.Left.Left.Field
	if leftField.Name != "user" || leftField.Value.String() != "foo" {
		t.Errorf("unexpected left field: %s:{%s}", leftField.Name, leftField.Value.String())
	}

	rightField := q.Expression.Left.Right[0].Field
	if rightField.Name != "provider" || rightField.Value.String() != "bar" {
		t.Errorf("unexpected right field: %s:{%s}", rightField.Name, rightField.Value.String())
	}
}

func TestParse_GroupedExpression(t *testing.T) {
	q, err := Parse("@user:{foo} & (@provider:{github} | @provider:{gitlab})")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Left.Right) != 1 {
		t.Fatalf("expected 1 AND operand, got %d", len(q.Expression.Left.Right))
	}

	subExpr := q.Expression.Left.Right[0].SubExpr
	if subExpr == nil || len(subExpr.Right) != 1 {
		t.Fatalf("expected subexpression with 1 OR operand")
	}

	leftField := subExpr.Left.Left.Field
	if leftField.Name != "provider" || leftField.Value.String() != "github" {
		t.Errorf("unexpected subexpr left: %s:{%s}", leftField.Name, leftField.Value.String())
	}

	rightField := subExpr.Right[0].Left.Field
	if rightField.Name != "provider" || rightField.Value.String() != "gitlab" {
		t.Errorf("unexpected subexpr right: %s:{%s}", rightField.Name, rightField.Value.String())
	}
}

func TestParse_MultipleOr(t *testing.T) {
	q, err := Parse("@a:{1} | @b:{2} | @c:{3}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Right) != 2 {
		t.Fatalf("expected 2 OR operands, got %d", len(q.Expression.Right))
	}

	firstField := q.Expression.Left.Left.Field
	if firstField.Name != "a" || firstField.Value.String() != "1" {
		t.Errorf("unexpected first field: %s:{%s}", firstField.Name, firstField.Value.String())
	}

	secondField := q.Expression.Right[0].Left.Field
	if secondField.Name != "b" || secondField.Value.String() != "2" {
		t.Errorf("unexpected second field: %s:{%s}", secondField.Name, secondField.Value.String())
	}

	thirdField := q.Expression.Right[1].Left.Field
	if thirdField.Name != "c" || thirdField.Value.String() != "3" {
		t.Errorf("unexpected third field: %s:{%s}", thirdField.Name, thirdField.Value.String())
	}
}

func TestParse_MultipleAnd(t *testing.T) {
	q, err := Parse("@a:{1} & @b:{2} & @c:{3}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Left.Right) != 2 {
		t.Fatalf("expected 2 AND operands, got %d", len(q.Expression.Left.Right))
	}

	firstField := q.Expression.Left.Left.Field
	if firstField.Name != "a" || firstField.Value.String() != "1" {
		t.Errorf("unexpected first field: %s:{%s}", firstField.Name, firstField.Value.String())
	}

	secondField := q.Expression.Left.Right[0].Field
	if secondField.Name != "b" || secondField.Value.String() != "2" {
		t.Errorf("unexpected second field: %s:{%s}", secondField.Name, secondField.Value.String())
	}

	thirdField := q.Expression.Left.Right[1].Field
	if thirdField.Name != "c" || thirdField.Value.String() != "3" {
		t.Errorf("unexpected third field: %s:{%s}", thirdField.Name, thirdField.Value.String())
	}
}

func TestParse_HyphenInFieldName(t *testing.T) {
	q, err := Parse("@key-bits:{256}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Name != "key-bits" {
		t.Errorf("expected field name 'key-bits', got '%s'", field.Name)
	}
	if field.Value.String() != "256" {
		t.Errorf("expected value '256', got '%s'", field.Value.String())
	}
}

func TestParse_UnderscoreInFieldName(t *testing.T) {
	q, err := Parse("@key_bits:{256}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Name != "key_bits" {
		t.Errorf("expected field name 'key_bits', got '%s'", field.Name)
	}
	if field.Value.String() != "256" {
		t.Errorf("expected value '256', got '%s'", field.Value.String())
	}
}

func TestParse_DottedFieldName(t *testing.T) {
	q, err := Parse("@source.username:{torvalds}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	field := q.Expression.Left.Left.Field
	if field.Name != "source.username" {
		t.Errorf("expected field name 'source.username', got '%s'", field.Name)
	}
	if field.Value.String() != "torvalds" {
		t.Errorf("expected value 'torvalds', got '%s'", field.Value.String())
	}
}

func TestParse_DottedFieldNameComplex(t *testing.T) {
	q, err := Parse("@source.username:{foo} & @source.provider:{github}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.Expression.Left.Right) != 1 {
		t.Fatalf("expected 1 AND operand, got %d", len(q.Expression.Left.Right))
	}

	leftField := q.Expression.Left.Left.Field
	if leftField.Name != "source.username" || leftField.Value.String() != "foo" {
		t.Errorf("unexpected left field: %s:{%s}", leftField.Name, leftField.Value.String())
	}

	rightField := q.Expression.Left.Right[0].Field
	if rightField.Name != "source.provider" || rightField.Value.String() != "github" {
		t.Errorf("unexpected right field: %s:{%s}", rightField.Name, rightField.Value.String())
	}
}

func TestParse_InvalidSyntax(t *testing.T) {
	tests := []string{
		"@user",
		"@user:",
		"@user:{}",
		"user:{foo}",
		"@:{foo}",
		"@user{foo}",
		"@user:{foo",
		"@user:foo}",
		"(",
		")",
		"@user:{foo} |",
		"| @user:{foo}",
	}

	for _, input := range tests {
		_, err := Parse(input)
		if err == nil {
			t.Errorf("expected error for input %q", input)
		}
	}
}
