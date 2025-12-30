package validate

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/merlindorin/sshark-api/internal/redisquery"
)

type URI struct {
	Query string `uri:"query" binding:"required"`
}

type Response struct {
	Query       string `json:"query"`
	IsValid     bool   `json:"is_valid"`
	Message     string `json:"message"`
	Explanation string `json:"explanation"`
}

func Validate() gin.HandlerFunc {
	return func(c *gin.Context) {
		uriParams := URI{}
		err := c.BindUri(&uriParams)
		if err != nil {
			_ = c.Error(fmt.Errorf("failed to parse uri: %w", err))
			return
		}

		_, parseErr := redisquery.Parse(uriParams.Query)
		if parseErr != nil {
			c.JSON(http.StatusOK, Response{
				Query:       uriParams.Query,
				IsValid:     false,
				Message:     "Invalid query syntax",
				Explanation: parseErr.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, Response{
			Query:       uriParams.Query,
			IsValid:     true,
			Message:     "Query is valid",
			Explanation: "",
		})
	}
}
