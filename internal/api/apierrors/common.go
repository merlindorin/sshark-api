package apierrors

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func InvalidPathParamError(c *gin.Context) *APIError {
	return NewAPIError(c, http.StatusBadRequest, NewDetailedError(
		c,
		"INVALID_PATH_PARAM",
		"Not a valid path param",
		"The URL is not valid. This is unexpected",
		"Return to the home page.",
	))
}

func InvalidQueryParamError(c *gin.Context, availableQuery []string) *APIError {
	return NewAPIError(c, http.StatusBadRequest, NewDetailedError(
		c,
		"INVALID_QUERY_PARAM",
		"Not a valid query param",
		fmt.Sprintf("The URL is not valid. The following query params are available: %s", strings.Join(availableQuery, ",")),
		"Return to the home page.",
	))
}

func InternalError(c *gin.Context) *APIError {
	return NewAPIError(c, http.StatusInternalServerError, NewDetailedError(
		c,
		"INTERNAL_ERROR",
		"Internal error",
		"An internal error has occurred. Please try again later.",
		"Return to the home page or try again.",
	))
}

func InvalidSearchQueryError(c *gin.Context, err error, q string, examples []string) *APIError {
	return NewAPIError(c, http.StatusBadRequest, NewDetailedError(
		c,
		"INVALID_SEARCH_QUERY",
		"Not a valid search query",
		fmt.Sprintf(
			"The search query `%s` is not a valid search. We found the following technical error: %s",
			q,
			err,
		),
		fmt.Sprintf(
			"You can use one the following query `%s` or get mode examples in the documentation.",
			strings.Join(examples, "`,`"),
		),
	))
}
