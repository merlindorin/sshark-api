package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/apikey"
	"github.com/gin-gonic/gin"
	"github.com/merlindorin/sshark-api/api/authenticated"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/metrics"
)

// errorKey is the field ad-hoc error responses use. Named so the linter stops counting it,
// and so the shape stays consistent if these handlers ever move to the structured errors the
// newer endpoints return.
const (
	errorKey            = "error"
	errUnauthorizedText = "unauthorized"
)

type Server struct {
	logger          *zap.Logger
	keyServices     KeyServices
	profileServices ProfileServices
	taskServices    TaskServices
	metrics         *metrics.Metrics
}

//nolint:revive // method name from generated interface
func (s Server) ListApiKeys(c *gin.Context) {
	ctx := c.Request.Context()

	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok {
		s.logger.Error("failed to get user claims")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: errUnauthorizedText})
		return
	}

	params := &apikey.ListParams{
		Subject: &claims.Subject,
	}

	keyList, err := apikey.List(ctx, params)
	if err != nil {
		s.logger.Error("failed to list API keys", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{errorKey: "failed to list API keys"})
		return
	}

	response := gin.H{
		"api_keys": keyList.APIKeys,
	}

	c.JSON(http.StatusOK, response)
}

//nolint:revive // method name from generated interface
func (s Server) CreateApiKey(c *gin.Context) {
	ctx := c.Request.Context()

	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok {
		s.logger.Error("failed to get user claims")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: errUnauthorizedText})
		return
	}

	var req authenticated.CreateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to parse request body", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{errorKey: "invalid request body"})
		return
	}

	params := &apikey.CreateParams{
		Subject: &claims.Subject,
		Name:    &req.Name,
	}

	if req.Description != nil {
		params.Description = req.Description
	}

	if req.Claims != nil {
		claimsJSON, err := json.Marshal(req.Claims)
		if err != nil {
			s.logger.Error("failed to marshal claims", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{errorKey: "invalid claims format"})
			return
		}
		params.Claims = claimsJSON
	}

	if req.Scopes != nil {
		params.Scopes = *req.Scopes
	}

	if req.Expiration != nil {
		secondsUntilExpiration := *req.Expiration - time.Now().Unix()
		if secondsUntilExpiration < 0 {
			s.logger.Error("expiration time is in the past")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{errorKey: "expiration time must be in the future"})
			return
		}
		params.SecondsUntilExpiration = &secondsUntilExpiration
	}

	key, err := apikey.Create(ctx, params)
	if err != nil {
		s.logger.Error("failed to create API key", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{errorKey: "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, key)
}

//nolint:revive // method name from generated interface
func (s Server) DeleteApiKey(c *gin.Context, id string) {
	ctx := c.Request.Context()

	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok {
		s.logger.Error("failed to get user claims")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{errorKey: errUnauthorizedText})
		return
	}

	keyList, err := apikey.List(ctx, &apikey.ListParams{
		Subject: &claims.Subject,
	})
	if err != nil {
		s.logger.Error("failed to list API keys", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{errorKey: "failed to verify API key ownership"})
		return
	}

	var keyExists bool
	for _, key := range keyList.APIKeys {
		if key.ID == id {
			keyExists = true
			break
		}
	}

	if !keyExists {
		s.logger.Error("API key not found or doesn't belong to user", zap.String("key_id", id))
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{errorKey: "API key not found"})
		return
	}

	_, err = apikey.Delete(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete API key", zap.Error(err), zap.String("key_id", id))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{errorKey: "failed to delete API key"})
		return
	}

	c.Status(http.StatusNoContent)
}

func NewServer(
	logger *zap.Logger,
	keyServices KeyServices,
	profileServices ProfileServices,
	taskServices TaskServices,
	m *metrics.Metrics,
) *Server {
	return &Server{
		logger:          logger,
		keyServices:     keyServices,
		profileServices: profileServices,
		taskServices:    taskServices,
		metrics:         m,
	}
}

func (s Server) ListMyTasks(c *gin.Context) {
	ListMyTasks(c, s.logger, s.taskServices)
}

func (s Server) GetMyTask(c *gin.Context, id openapi_types.UUID) {
	GetMyTask(c, s.logger, s.taskServices, id)
}

func (s Server) DeleteMyProfile(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("profile_delete"))
	}
	DeleteMyProfile(c, s.logger, s.profileServices)
}

func (s Server) SetMyUsername(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("profile_set_username"))
	}
	SetMyUsername(c, s.logger, s.profileServices)
}

func (s Server) CheckUsernameAvailable(c *gin.Context, params authenticated.CheckUsernameAvailableParams) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("profile_check_username"))
	}
	CheckUsernameAvailable(c, s.logger, s.profileServices, params)
}

func (s Server) GetMe(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("profile_get"))
	}
	Me(c, s.logger, s.profileServices)
}

func (s Server) ListMyKeys(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("key_list"))
	}
	ListMyKeys(c, s.logger, s.keyServices)
}

func (s Server) RefreshMyKeys(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("key_refresh"))
	}
	RefreshMyKeys(c, s.logger, s.keyServices)
}

func (s Server) RevokeMyKey(c *gin.Context, id openapi_types.UUID) {
	if s.metrics != nil {
		s.metrics.APIUserOps.Add(c.Request.Context(), 1, metrics.WithOperationType("key_revoke"))
	}
	RevokeMyKey(c, s.logger, s.keyServices, id)
}
