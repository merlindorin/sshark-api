package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/requestid"

	"github.com/merlindorin/sshark-api/internal/api/apierrors"
	"github.com/merlindorin/sshark-api/internal/api/probe"
	"github.com/merlindorin/sshark-api/internal/api/search"
	"github.com/merlindorin/sshark-api/internal/api/sshkeys"
	"github.com/merlindorin/sshark-api/internal/api/validate"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/infra/github"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"

	"github.com/gin-contrib/timeout"
	ginzap "github.com/gin-contrib/zap"

	"github.com/gin-gonic/gin"

	"github.com/merlindorin/go-shared/pkg/cmd"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Serve struct {
	Host string `env:"HOST" help:"Host to bind the server to" default:"0.0.0.0"`
	Port int    `env:"PORT" help:"Port to bind the server to" default:"8080"`

	Timeout time.Duration `default:"5s" help:"HTTP request timeout"`

	RedisHost         string `env:"REDIS_HOST" help:"Redis host" default:"localhost"`
	RedisPort         int    `env:"REDIS_PORT" help:"Redis port" default:"6379"`
	RedisPassword     string `env:"REDIS_PASSWORD" help:"Redis password"`
	RedisDB           int    `env:"REDIS_DB" help:"Redis db" default:"0"`
	RedisForceReindex bool   `env:"REDIS_FORCE_REINDEX" help:"Redis force reindex"`
}

func (s *Serve) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s *Serve) RedisAddr() string {
	return fmt.Sprintf("%s:%d", s.RedisHost, s.RedisPort)
}

func (s *Serve) Run(common *cmd.Commons) error {
	logger := common.MustLogger()

	logger.Info(
		"Starting server...",
		zap.String("name", common.Version.Name()),
		zap.String("version", common.Version.Version()),
		zap.String("address", s.Addr()),
		zap.String("redis.address", s.RedisAddr()),
		zap.Int("redis.db", s.RedisDB),
		zap.String("gin.version", gin.Version),
	)

	rdb := redis.NewClient(&redis.Options{
		Addr:          s.RedisAddr(),
		Password:      s.RedisPassword,
		DB:            s.RedisDB,
		UnstableResp3: true,
	})

	if !common.Development {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(timeout.New(timeout.WithTimeout(s.Timeout)))
	r.Use(requestid.New())
	r.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	r.Use(ginzap.RecoveryWithZap(logger, true))
	r.Use(ErrorHandler(logger))

	probe.MountProbe(r)

	cl := github.NewFetcher(logger)
	srepo := sshkeysrepository.NewRedisRepository(rdb)
	grepo := githubrepository.NewRepository(rdb)
	service := ingester.New(grepo, srepo, cl)
	search.MountV1(r.Group("/api/v1/search"), logger.Named("search"), srepo, srepo, service)
	sshkeys.MountV1(r.Group("/api/v1/sshkeys"), srepo)
	validate.MountV1(r.Group("/api/v1/validate"), srepo)

	err := srepo.EnsureIndex(context.Background(), s.RedisForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}

	err = grepo.EnsureIndex(context.Background(), s.RedisForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}

	errCh := make(chan error, 1)
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("HTTP server listening", zap.String("address", s.Addr()))
		if runErr := r.Run(s.Addr()); runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
			errCh <- runErr
		}
	}()

	select {
	case <-term:
		logger.Info("Received SIGTERM, shutting down gracefully...")
		return nil
	case serverErr := <-errCh:
		logger.Error("Server error", zap.Error(serverErr))
		return fmt.Errorf("server error: %w", serverErr)
	}
}

// ErrorHandler captures errors and returns a consistent JSON error response.
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			var httpError *apierrors.APIError
			if ok := errors.As(err, &httpError); ok {
				c.JSON(httpError.StatusCode, httpError)
				return
			}

			logger.Error("Uncatched error in request", zap.Error(err))
			c.JSON(http.StatusInternalServerError, apierrors.InternalError(c))
		}
	}
}
