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

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	"github.com/merlindorin/sshark-api/internal/api/me"
	"github.com/merlindorin/sshark-api/internal/api/probe"
	"github.com/merlindorin/sshark-api/internal/api/search"
	"github.com/merlindorin/sshark-api/internal/api/sshkeys"
	"github.com/merlindorin/sshark-api/internal/api/stats"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/infra/github"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
	"github.com/merlindorin/sshark-api/internal/middleware"

	"github.com/gin-contrib/timeout"
	ginzap "github.com/gin-contrib/zap"

	"github.com/gin-gonic/gin"

	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.uber.org/zap"

	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
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

	probe.MountProbe(r)

	cl := github.NewFetcher(logger)
	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	grepo := githubrepository.NewRepository(redisClient)
	service := ingester.New(grepo, srepo, cl)

	api := r.Group("/api/v1")
	protected := middleware.AdaptClerk(clerkhttp.RequireHeaderAuthorization())

	search.MountV1(api.Group("/search"), logger.Named("search"), srepo, srepo, service)
	sshkeys.MountV1(api.Group("/sshkeys"), logger.Named("sshkeys"), srepo)
	stats.MountV1(api.Group("/stats"), logger.Named("stats"), srepo)

	// protected
	me.MountV1(api.Group("/me", protected), logger.Named("me"))

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
