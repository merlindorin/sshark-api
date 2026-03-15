package postgres

import (
	"fmt"
	"strings"

	"github.com/merlindorin/sshark-api/internal/domain/query"
)

type Builder struct {
	fields map[string]FieldMapping
}

func NewBuilder(fields map[string]FieldMapping) *Builder {
	return &Builder{fields: fields}
}

func (b *Builder) Build(q *query.Query) (string, []any, error) {
	if q == nil || q.Expression == nil {
		return "", nil, nil
	}

	ctx := &buildContext{
		fields:   b.fields,
		args:     make([]any, 0),
		argIndex: 1,
	}

	clause, err := ctx.buildOrExpr(q.Expression)
	if err != nil {
		return "", nil, err
	}

	return clause, ctx.args, nil
}

type buildContext struct {
	fields   map[string]FieldMapping
	args     []any
	argIndex int
}

func (c *buildContext) addArg(value any) string {
	c.args = append(c.args, value)
	placeholder := fmt.Sprintf("$%d", c.argIndex)
	c.argIndex++
	return placeholder
}

func (c *buildContext) buildOrExpr(expr *query.OrExpr) (string, error) {
	if expr == nil {
		return "", nil
	}

	left, err := c.buildAndExpr(expr.Left)
	if err != nil {
		return "", err
	}

	if len(expr.Right) == 0 {
		return left, nil
	}

	parts := []string{left}
	for _, right := range expr.Right {
		rightClause, buildErr := c.buildAndExpr(right)
		if buildErr != nil {
			return "", buildErr
		}
		parts = append(parts, rightClause)
	}

	return "(" + strings.Join(parts, " OR ") + ")", nil
}

func (c *buildContext) buildAndExpr(expr *query.AndExpr) (string, error) {
	if expr == nil {
		return "", nil
	}

	left, err := c.buildUnary(expr.Left)
	if err != nil {
		return "", err
	}

	if len(expr.Right) == 0 {
		return left, nil
	}

	parts := []string{left}
	for _, right := range expr.Right {
		rightClause, buildErr := c.buildUnary(right)
		if buildErr != nil {
			return "", buildErr
		}
		parts = append(parts, rightClause)
	}

	return "(" + strings.Join(parts, " AND ") + ")", nil
}

func (c *buildContext) buildUnary(u *query.Unary) (string, error) {
	if u == nil {
		return "", nil
	}

	if u.SubExpr != nil {
		return c.buildOrExpr(u.SubExpr)
	}

	return c.buildField(u.Field)
}

func (c *buildContext) buildField(f *query.Field) (string, error) {
	if f == nil {
		return "", nil
	}

	mapping, ok := c.fields[f.Name]
	if !ok {
		return "", fmt.Errorf("unknown field: @%s", f.Name)
	}

	value := f.Value.String()
	hasWildcard := f.Value.HasWildcard()

	if mapping.IsArray {
		return c.buildArrayCondition(mapping.Column, value, hasWildcard)
	}

	return c.buildScalarCondition(mapping.Column, value, hasWildcard)
}

func (c *buildContext) buildScalarCondition(column, value string, hasWildcard bool) (string, error) {
	if hasWildcard {
		likeValue := strings.ReplaceAll(value, "*", "%")
		placeholder := c.addArg(likeValue)
		return fmt.Sprintf("%s LIKE %s", column, placeholder), nil
	}

	placeholder := c.addArg(value)
	return fmt.Sprintf("%s = %s", column, placeholder), nil
}

func (c *buildContext) buildArrayCondition(column, value string, hasWildcard bool) (string, error) {
	if hasWildcard {
		likeValue := strings.ReplaceAll(value, "*", "%")
		placeholder := c.addArg(likeValue)
		return fmt.Sprintf("EXISTS (SELECT 1 FROM unnest(%s) AS elem WHERE elem LIKE %s)", column, placeholder), nil
	}

	placeholder := c.addArg(value)
	return fmt.Sprintf("%s = ANY(%s)", placeholder, column), nil
}
