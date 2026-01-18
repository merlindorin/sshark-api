package common

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/internal/api"
)

func InvalidPathParamError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
		c,
		"INVALID_PATH_PARAM",
		"Not a valid path param",
		"The URL is not valid. This is unexpected",
		"Return to the home page.",
	))
}

func InvalidQueryParamError(c *gin.Context, availableQuery []string) *api.APIError {
	return api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
		c,
		"INVALID_QUERY_PARAM",
		"Not a valid query param",
		fmt.Sprintf("The URL is not valid. The following query params are available: %s", strings.Join(availableQuery, ",")),
		"Return to the home page.",
	))
}

func InternalError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusInternalServerError, api.NewDetailedError(
		c,
		"INTERNAL_ERROR",
		"Internal error",
		"An internal error has occurred. Please try again later.",
		"Return to the home page or try again.",
	))
}

func InvalidSearchQueryError(c *gin.Context, err error, q string, examples []string) *api.APIError {
	return api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
		c,
		"INVALID_SEARCH_QUERY",
		"Not a valid search query",
		fmt.Sprintf(
			"The search query `%s` is not a valid search. We found the following technical error: %s",
			q,
			err.Error(),
		),
		fmt.Sprintf(
			"You can use one the following query `%s` or get mode examples in the documentation.",
			strings.Join(examples, "`,`"),
		),
	))
}
