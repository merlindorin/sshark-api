package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-contrib/requestid"
	"github.com/gin-contrib/timeout"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	apiAuthenticatedV1 "github.com/merlindorin/sshark-api/api/authenticated/v1"
	apiPrivateV1 "github.com/merlindorin/sshark-api/api/private/v1"
	apiPublicV1 "github.com/merlindorin/sshark-api/api/public/v1"

	apiAuthenticated "github.com/merlindorin/sshark-api/api/authenticated"
	apiPrivate "github.com/merlindorin/sshark-api/api/private"
	apiPublic "github.com/merlindorin/sshark-api/api/public"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	"github.com/merlindorin/sshark-api/internal/app/keyops"
	"github.com/merlindorin/sshark-api/internal/domain/profiles"
	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/domain/tasks"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/github"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/gitlab"
	"github.com/merlindorin/sshark-api/internal/infra/identity"
	"github.com/merlindorin/sshark-api/internal/infra/jobs"
	profilesrepo "github.com/merlindorin/sshark-api/internal/infra/profiles/postgres"
	publickeysrepo "github.com/merlindorin/sshark-api/internal/infra/publickeys/postgres"
	infrascraper "github.com/merlindorin/sshark-api/internal/infra/scraper"
	scraperrepo "github.com/merlindorin/sshark-api/internal/infra/scraper/postgres"
	sourcesrepo "github.com/merlindorin/sshark-api/internal/infra/sources/postgres"
	tasksrepo "github.com/merlindorin/sshark-api/internal/infra/tasks/postgres"
	"github.com/merlindorin/sshark-api/internal/middleware"
	"github.com/merlindorin/sshark-api/internal/otel"
)

const (
	apiPath = "/api/v1"

	// queueStopTimeout is how long jobs in flight get to finish on shutdown.
	queueStopTimeout = 30 * time.Second
)

type Serve struct {
	ClerkToken  string        `env:"CLERK_TOKEN" help:"Clerk to use for auth"`
	GithubToken string        `env:"GITHUB_TOKEN" help:"GitHub API token used to refresh a user's keys on demand"`
	GitlabToken string        `env:"GITLAB_TOKEN" help:"GitLab API token used to refresh a user's keys on demand"`
	Timeout     time.Duration `default:"5s" help:"HTTPServer request timeout"`
}

// buildScrapers wires one on-demand scraper per provider, so a signed-in user can pull their
// own keys straight away instead of waiting for the background crawler to come around.
func (s *Serve) buildScrapers(
	logger *zap.Logger,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	progressRepo scraper.ProgressRepository,
) map[scraper.Provider]scraper.Service {
	cfg := infrascraper.Config{}

	return map[scraper.Provider]scraper.Service{
		scraper.ProviderGitHub: infrascraper.NewService(
			logger, github.NewFetcher(github.WithToken(s.GithubToken)),
			sourcesRepo, publickeysRepo, progressRepo, cfg,
		),
		scraper.ProviderGitLab: infrascraper.NewService(
			logger, gitlab.NewFetcher(gitlab.WithToken(s.GitlabToken)),
			sourcesRepo, publickeysRepo, progressRepo, cfg,
		),
	}
}

func (s *Serve) Run(
	ctx context.Context,
	common *cmd.Commons,
	postgres *globals.Postgres,
	httpServer *globals.HTTPServer,
	gotel *globals.MetricServer,
) error {
	name := common.Version.Name()
	version := common.Version.Version()
	namedLogger := common.MustLogger().Named(name)
	serverAddr := httpServer.Addr()
	development := common.Development

	namedLogger.Info(
		"Starting server...",
		zap.String("name", name),
		zap.String("version", version),
		zap.String("address", serverAddr),
		zap.Bool("development", development),
		zap.String("postgres.host", postgres.Host),
		zap.String("postgres.database", postgres.Database),
		zap.String("gin.version", gin.Version),
	)

	pool, err := postgres.Pool(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer pool.Close()

	clerk.SetKey(s.ClerkToken)
	clerkConfigured := s.ClerkToken != ""

	// Worth shouting about: the server still starts and serves public search, so nothing looks
	// broken until someone signs in and every authenticated call comes back 401.
	if !clerkConfigured {
		namedLogger.Error(
			"CLERK_TOKEN is not set — every authenticated request will be rejected. " +
				"Public search keeps working; /me, key management and profiles do not.",
		)
	}

	gin.SetMode(getGinMode(development))

	sourcesRepo := sourcesrepo.NewRepository(pool)
	tasksRepo := tasksrepo.NewRepository(pool)
	publickeysRepo := publickeysrepo.NewRepository(pool)
	profilesRepo := profilesrepo.NewRepository(pool)
	identities := identity.NewResolver()

	router := gin.New()
	router.Use(otelgin.Middleware(name))
	router.Use(timeout.New(timeout.WithTimeout(s.Timeout)))
	router.Use(requestid.New())
	router.Use(ginzap.Ginzap(namedLogger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(namedLogger, true))
	router.Use(middleware.ErrorHandler(namedLogger))

	gotel.Mount(router)

	apiPrivateV1Handler := apiPrivateV1.NewServer(namedLogger)
	apiPrivate.RegisterHandlers(router, apiPrivateV1Handler)

	apiPublicV1Handler := apiPublicV1.NewServer(namedLogger, sourcesRepo, publickeysRepo, profilesRepo, identities)
	apiPublic.RegisterHandlers(router.Group(apiPath), apiPublicV1Handler)

	keyServices, profileServices, taskServices, queue, err := s.buildAuthenticatedServices(
		namedLogger, pool, sourcesRepo, publickeysRepo, profilesRepo, tasksRepo, identities)
	if err != nil {
		return err
	}

	apiAuthenticatedV1Handler := apiAuthenticatedV1.NewServer(
		namedLogger, keyServices, profileServices, taskServices)
	authenticated := router.Group(apiPath, middleware.RequireAuth(clerkConfigured))
	apiAuthenticated.RegisterHandlers(authenticated, apiAuthenticatedV1Handler)

	errs, ctx := errgroup.WithContext(ctx)
	errs.Go(runQueue(ctx, queue, namedLogger))
	errs.Go(waitForShutdownSignal(ctx))
	errs.Go(runMeterProvider(ctx, name, version, namedLogger))
	errs.Go(httpServer.Start(ctx, namedLogger, router))

	if waitErr := errs.Wait(); waitErr != nil {
		namedLogger.Info("Shutting down", zap.Error(waitErr))
	}

	return nil
}

// runQueue processes background jobs alongside the HTTP server, and drains what is in flight
// when the process is asked to stop.
// buildAuthenticatedServices assembles what the signed-in endpoints need, including the queue
// that carries out anything too slow to hold a request open for.
func (s *Serve) buildAuthenticatedServices(
	logger *zap.Logger,
	pool *pgxpool.Pool,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	profilesRepo profiles.Repository,
	tasksRepo tasks.Repository,
	identities *identity.Resolver,
) (
	apiAuthenticatedV1.KeyServices,
	apiAuthenticatedV1.ProfileServices,
	apiAuthenticatedV1.TaskServices,
	*jobs.Queue,
	error,
) {
	keyOps := &keyops.Service{
		Logger:     logger,
		Profiles:   profilesRepo,
		Sources:    sourcesRepo,
		PublicKeys: publickeysRepo,
		Identities: identities,
		Scrapers: s.buildScrapers(
			logger, sourcesRepo, publickeysRepo, scraperrepo.NewProgressRepository(pool)),
	}

	queue, err := jobs.NewQueue(logger, pool, tasksRepo, keyOps)
	if err != nil {
		return apiAuthenticatedV1.KeyServices{}, apiAuthenticatedV1.ProfileServices{},
			apiAuthenticatedV1.TaskServices{}, nil, fmt.Errorf("failed to create the job queue: %w", err)
	}

	profileServices := apiAuthenticatedV1.ProfileServices{
		Profiles:   profilesRepo,
		Sources:    sourcesRepo,
		Identities: identities,
	}

	keyServices := apiAuthenticatedV1.KeyServices{
		Sources:    sourcesRepo,
		PublicKeys: publickeysRepo,
		Profiles:   profileServices,
		Identities: identities,
		Queue:      queue,
	}

	return keyServices, profileServices, apiAuthenticatedV1.TaskServices{Tasks: tasksRepo, Queue: queue}, queue, nil
}

func runQueue(ctx context.Context, queue *jobs.Queue, logger *zap.Logger) func() error {
	return func() error {
		if err := queue.Start(ctx); err != nil {
			return fmt.Errorf("failed to start the job queue: %w", err)
		}

		<-ctx.Done()

		stopCtx, cancel := context.WithTimeout(context.Background(), queueStopTimeout)
		defer cancel()

		if err := queue.Stop(stopCtx); err != nil {
			logger.Warn("job queue did not stop cleanly", zap.Error(err))
		}

		return nil
	}
}

func runMeterProvider(ctx context.Context, name, version string, logger *zap.Logger) func() error {
	return func() error {
		meterProvider, err := otel.InitMeterProvider(name, version)
		if err != nil {
			return fmt.Errorf("failed to initialize meter provider: %w", err)
		}

		<-ctx.Done()
		if err = meterProvider.Shutdown(context.Background()); err != nil {
			logger.Error("failed to shutdown meter provider", zap.Error(err))
			return err
		}

		return nil
	}
}

func waitForShutdownSignal(ctx context.Context) func() error {
	return func() error {
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		select {
		case <-ctx.Done():
			return nil
		case <-term:
			return errors.New("received shutdown signal")
		}
	}
}

func getGinMode(development bool) string {
	if development {
		return gin.DebugMode
	}

	return gin.ReleaseMode
}
