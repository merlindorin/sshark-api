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

func UnauthorizedError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusUnauthorized, api.NewDetailedError(
		c,
		"UNAUTHORIZED",
		"Unauthorized",
		"This endpoint requires a signed-in user.",
		"Sign in and try again.",
	))
}

func KeyNotFoundError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusNotFound, api.NewDetailedError(
		c,
		"KEY_NOT_FOUND",
		"Key not found",
		"This key does not exist, or it is not published under an account you have connected.",
		"Refresh your keys, then try again.",
	))
}

func ProviderNotConnectedError(c *gin.Context, provider string) *api.APIError {
	return api.NewAPIError(c, http.StatusForbidden, api.NewDetailedError(
		c,
		"PROVIDER_NOT_CONNECTED",
		"Provider account not connected",
		fmt.Sprintf("No %s account is connected to your sshark account.", provider),
		fmt.Sprintf("Connect your %s account, then try again.", provider),
	))
}

func ProviderForbiddenError(c *gin.Context, provider string, scope string) *api.APIError {
	return api.NewAPIError(c, http.StatusForbidden, api.NewDetailedError(
		c,
		"PROVIDER_FORBIDDEN",
		"Provider refused the operation",
		fmt.Sprintf("%s did not allow sshark to manage your keys on your behalf.", provider),
		fmt.Sprintf("Reconnect your %s account and grant the `%s` permission.", provider, scope),
	))
}

func ProviderUnavailableError(c *gin.Context, provider string) *api.APIError {
	return api.NewAPIError(c, http.StatusBadGateway, api.NewDetailedError(
		c,
		"PROVIDER_UNAVAILABLE",
		"Provider unavailable",
		fmt.Sprintf("sshark could not reach %s. Your keys were left untouched.", provider),
		"Try again in a few moments.",
	))
}

func InvalidUsernameError(c *gin.Context, details string) *api.APIError {
	return api.NewAPIError(c, http.StatusBadRequest, api.NewDetailedError(
		c,
		"INVALID_USERNAME",
		"Not a valid username",
		details,
		"Use 3 to 39 letters, digits, dashes or underscores.",
	))
}

func UsernameTakenError(c *gin.Context, username string) *api.APIError {
	return api.NewAPIError(c, http.StatusConflict, api.NewDetailedError(
		c,
		"USERNAME_TAKEN",
		"Username already taken",
		fmt.Sprintf("Someone else already holds %q.", username),
		"Pick a different username.",
	))
}

func ProfileNotFoundError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusNotFound, api.NewDetailedError(
		c,
		"PROFILE_NOT_FOUND",
		"Profile not found",
		"No SShark account holds this username.",
		"Check the spelling, or search for the name instead.",
	))
}

func TaskNotFoundError(c *gin.Context) *api.APIError {
	return api.NewAPIError(c, http.StatusNotFound, api.NewDetailedError(
		c,
		"TASK_NOT_FOUND",
		"Task not found",
		"No task with this identifier belongs to you.",
		"Check the identifier, or list your recent tasks.",
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
