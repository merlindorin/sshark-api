package query

import (
	"context"
)

// Explainer validates queries against Redis and returns execution plans.
type Explainer interface {
	ExplainQuery(ctx context.Context, query string) (string, error)
}
