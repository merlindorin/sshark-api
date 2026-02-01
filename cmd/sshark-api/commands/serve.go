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

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-contrib/requestid"
	"github.com/gin-contrib/timeout"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/merlindorin/go-shared/pkg/cmd"
	"github.com/merlindorin/sshark-api/api/authenticated"
	v2 "github.com/merlindorin/sshark-api/api/authenticated/v1"
	"github.com/merlindorin/sshark-api/api/private"
	v3 "github.com/merlindorin/sshark-api/api/private/v1"
	"github.com/merlindorin/sshark-api/api/public"
	v1 "github.com/merlindorin/sshark-api/api/public/v1"
	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
	"github.com/merlindorin/sshark-api/internal/middleware"
	"go.uber.org/zap"
)

type Serve struct {
	Host string `env:"HOST" help:"Host to bind the server to" default:"0.0.0.0"`
	Port int    `env:"PORT" help:"Port to bind the server to" default:"8080"`

	ClerkToken string `env:"CLERK_TOKEN" help:"Clerk to use for auth"`

	Timeout time.Duration `default:"5s" help:"HTTP request timeout"`
}

func (s *Serve) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s *Serve) Run(_ context.Context, common *cmd.Commons, redis *globals.Redis) error {
	logger := common.MustLogger().Named("server")

	logger.Info(
		"Starting server...",
		zap.String("name", common.Version.Name()),
		zap.String("version", common.Version.Version()),
		zap.String("address", s.Addr()),
		zap.String("redis.address", redis.Addr()),
		zap.Int("redis.db", redis.DB),
		zap.String("gin.version", gin.Version),
	)

	redisClient := redis.Client()
	clerk.SetKey(s.ClerkToken)

	if !common.Development {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(timeout.New(timeout.WithTimeout(s.Timeout)))
	r.Use(requestid.New())
	r.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	r.Use(ginzap.RecoveryWithZap(logger, true))
	r.Use(middleware.ErrorHandler(logger))

	srepo := sshkeysrepository.NewRedisRepository(redisClient)

	requireAuthMiddleware := middleware.RequireAuth()

	private.RegisterHandlers(r, v3.NewServer(logger.Named("internal api")))
	public.RegisterHandlers(r.Group("/api/v1"), v1.NewServer(logger.Named("public api"), srepo))
	authenticated.RegisterHandlers(
		r.Group("/api/v1", requireAuthMiddleware),
		v2.NewServer(logger.Named("authenticated api")),
	)

	errCh := make(chan error, 1)
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("HTTP server listening", zap.String("address", s.Addr()))

		server := &http.Server{
			Handler:           r,
			Addr:              s.Addr(),
			ReadHeaderTimeout: 5 * time.Second,
		}

		if runErr := server.ListenAndServe(); runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
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
