package query

import (
	"context"
)

// Validator validates queries against Redis and returns execution plans.
type Validator interface {
	ValidateQuery(ctx context.Context, query string) (string, error)
}
