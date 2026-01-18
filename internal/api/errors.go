//nolint:revive // api is appropriate for API error handling
package api

import (
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

func NewAPIError(c *gin.Context, statusCode int, err *DetailedError) *APIError {
	return &APIError{
		Status:           http.StatusText(statusCode),
		StatusCode:       statusCode,
		DetailedError:    err,
		RequestID:        requestid.Get(c),
		DocumentationURL: "https://sshark.app/doc",
	}
}

type DetailedError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details"`
	Timestamp  time.Time `json:"timestamp"`
	Path       string    `json:"path"`
	Suggestion string    `json:"suggestion"`
}

func NewDetailedError(c *gin.Context, code string, message string, details string, suggestion string) *DetailedError {
	return &DetailedError{
		Code:       code,
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
		Path:       c.Request.URL.Path,
		Suggestion: suggestion,
	}
}

//nolint:revive // APIError clearly indicates this is an API-level error
type APIError struct {
	Status           string         `json:"status"`
	StatusCode       int            `json:"status_code"`
	DetailedError    *DetailedError `json:"error"`
	RequestID        string         `json:"request_id"`
	DocumentationURL string         `json:"documentation_url"`
}

func (a APIError) Error() string {
	return a.Status
}
