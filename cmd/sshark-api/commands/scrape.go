package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	githubdomain "github.com/merlindorin/sshark-api/internal/domain/github"
	gitlabdomain "github.com/merlindorin/sshark-api/internal/domain/gitlab"
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/infra/github"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	"github.com/merlindorin/sshark-api/internal/infra/gitlab"
	gitlabrepository "github.com/merlindorin/sshark-api/internal/infra/gitlab/redis"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
	internalmetrics "github.com/merlindorin/sshark-api/internal/metrics"
	"github.com/merlindorin/sshark-api/internal/otel"
)

type Scrape struct {
	Provider    string  `help:"Provider (github, gitlab)" default:"github" enum:"github,gitlab"`
	RateLimit   float64 `help:"Requests per second" default:"2.0"`
	BatchSize   int     `help:"Users per batch" default:"100"`
	GitLabToken string  `env:"GITLAB_TOKEN" help:"GitLab API token (required for gitlab)"`
}

func (s *Scrape) Run(ctx context.Context, common *cmd.Commons, redis *globals.Redis) error {
	if s.Provider == "gitlab" && s.GitLabToken == "" {
		return fmt.Errorf("GITLAB_TOKEN required for gitlab provider")
	}

	logger := common.MustLogger()

	meterProvider, err := otel.InitMeterProvider("sshark-api-scraper", common.Version.Version())
	if err != nil {
		return fmt.Errorf("failed to initialize meter provider: %w", err)
	}
	defer func() {
		if shutdownErr := meterProvider.Shutdown(context.Background()); shutdownErr != nil {
			logger.Error("failed to shutdown meter provider", zap.Error(shutdownErr))
		}
	}()

	if err = internalmetrics.InitMetrics(); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	switch s.Provider {
	case "github":
		return s.runGitHub(ctx, common, redis)
	case "gitlab":
		return s.runGitLab(ctx, common, redis)
	default:
		return fmt.Errorf("unsupported provider: %s", s.Provider)
	}
}

func (s *Scrape) runGitHub(ctx context.Context, common *cmd.Commons, redis *globals.Redis) error { //nolint:funlen
	logger := common.MustLogger().Named("scraper")

	logger.Info(
		"Starting GitHub user scraper...",
		zap.String("name", common.Version.Name()),
		zap.String("version", common.Version.Version()),
		zap.Float64("rate_limit", s.RateLimit),
		zap.Int("batch_size", s.BatchSize),
	)

	scraperMetrics, err := internalmetrics.NewScraperMetrics("github")
	if err != nil {
		return fmt.Errorf("failed to initialize scraper metrics: %w", err)
	}

	redisClient := redis.Client()

	fetcher := github.NewFetcher(logger)
	usersFetcher := github.NewUsersFetcher(logger)
	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	grepo := githubrepository.NewRepository(redisClient)
	service := ingester.NewGitHub(grepo, srepo, fetcher)
	progressTracker := github.NewProgressTracker(redisClient)

	limiter := rate.NewLimiter(rate.Limit(s.RateLimit), 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-term
		logger.Info("Received shutdown signal, stopping gracefully...")
		cancel()
	}()

	lastID, err := progressTracker.GetLastUserID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last user ID: %w", err)
	}

	if lastID > 0 {
		logger.Info("Resuming from last position", zap.Int64("last_user_id", lastID))
	}

	currentID := lastID
	if err = scraperMetrics.RegisterPositionGauge(ctx, "github", func() int64 {
		return currentID
	}); err != nil {
		logger.Warn("Failed to register position gauge", zap.Error(err))
	}

	totalProcessed := 0
	totalIngested := 0
	startTime := time.Now()

	providerAttr := attribute.String("provider", "github")

	processor := &githubUserProcessor{
		service:         service,
		grepo:           grepo,
		scraperMetrics:  scraperMetrics,
		providerAttr:    providerAttr,
		logger:          logger,
		progressTracker: progressTracker,
	}

	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			logger.Info(
				"Scraper stopped",
				zap.Int("total_processed", totalProcessed),
				zap.Int("total_ingested", totalIngested),
				zap.Duration("duration", time.Since(startTime)),
			)
			return nil
		}

		waitStart := time.Now()
		if waitErr := limiter.Wait(ctx); waitErr != nil {
			return fmt.Errorf("rate limiter error: %w", waitErr)
		}
		scraperMetrics.RateLimitWait.Record(ctx, time.Since(waitStart).Seconds(), metric.WithAttributes(providerAttr))

		users, fetchErr := usersFetcher.FetchUsers(ctx, lastID, s.BatchSize)
		if fetchErr != nil {
			logger.Error("Failed to fetch users", zap.Error(fetchErr))
			scraperMetrics.FetchErrors.Add(ctx, 1, metric.WithAttributes(providerAttr))
			continue
		}

		scraperMetrics.BatchSize.Record(ctx, int64(len(users)), metric.WithAttributes(providerAttr))

		if len(users) == 0 {
			logger.Info(
				"Reached end of users list",
				zap.Int("total_processed", totalProcessed),
				zap.Int("total_ingested", totalIngested),
				zap.Duration("duration", time.Since(startTime)),
			)
			return nil
		}

		for _, user := range users {
			if ctxErr := ctx.Err(); ctxErr != nil {
				break
			}

			totalProcessed++
			scraperMetrics.UsersProcessed.Add(ctx, 1, metric.WithAttributes(providerAttr))

			newID, ingested, processErr := processor.process(ctx, user, limiter)
			if processErr != nil {
				return processErr
			}

			lastID = newID
			currentID = lastID

			if ingested {
				totalIngested++
			}

			if totalProcessed%100 == 0 {
				logger.Info(
					"Progress",
					zap.Int("processed", totalProcessed),
					zap.Int("ingested", totalIngested),
					zap.Int64("last_id", lastID),
					zap.String("last_user", user.Login),
				)
			}

			if saveErr := progressTracker.SetLastUserID(ctx, lastID); saveErr != nil {
				logger.Error("Failed to save progress", zap.Error(saveErr))
			}
		}
	}
}

func (s *Scrape) runGitLab(ctx context.Context, common *cmd.Commons, redis *globals.Redis) error { //nolint:funlen
	logger := common.MustLogger().Named("scraper")

	logger.Info(
		"Starting GitLab user scraper...",
		zap.String("name", common.Version.Name()),
		zap.String("version", common.Version.Version()),
		zap.Float64("rate_limit", s.RateLimit),
		zap.Int("batch_size", s.BatchSize),
	)

	scraperMetrics, err := internalmetrics.NewScraperMetrics("gitlab")
	if err != nil {
		return fmt.Errorf("failed to initialize scraper metrics: %w", err)
	}

	redisClient := redis.Client()

	fetcher := gitlab.NewFetcher(logger, s.GitLabToken)
	usersFetcher := gitlab.NewUsersFetcher(logger, s.GitLabToken)
	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	grepo := gitlabrepository.NewRepository(redisClient)
	service := ingester.NewGitLab(grepo, srepo, fetcher)
	progressTracker := gitlab.NewProgressTracker(redisClient)

	limiter := rate.NewLimiter(rate.Limit(s.RateLimit), 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-term
		logger.Info("Received shutdown signal, stopping gracefully...")
		cancel()
	}()

	page, err := progressTracker.GetLastPage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last page: %w", err)
	}

	if page > 1 {
		logger.Info("Resuming from last position", zap.Int("last_page", page))
	}

	currentPage := int64(page)
	if err = scraperMetrics.RegisterPositionGauge(ctx, "gitlab", func() int64 {
		return currentPage
	}); err != nil {
		logger.Warn("Failed to register position gauge", zap.Error(err))
	}

	totalProcessed := 0
	totalIngested := 0
	startTime := time.Now()

	providerAttr := attribute.String("provider", "gitlab")

	processor := &gitlabUserProcessor{
		service:         service,
		grepo:           grepo,
		scraperMetrics:  scraperMetrics,
		providerAttr:    providerAttr,
		logger:          logger,
		progressTracker: progressTracker,
	}

	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			logger.Info(
				"Scraper stopped",
				zap.Int("total_processed", totalProcessed),
				zap.Int("total_ingested", totalIngested),
				zap.Duration("duration", time.Since(startTime)),
			)
			return nil
		}

		waitStart := time.Now()
		if waitErr := limiter.Wait(ctx); waitErr != nil {
			return fmt.Errorf("rate limiter error: %w", waitErr)
		}
		scraperMetrics.RateLimitWait.Record(ctx, time.Since(waitStart).Seconds(), metric.WithAttributes(providerAttr))

		users, fetchErr := usersFetcher.FetchUsers(ctx, page, s.BatchSize)
		if fetchErr != nil {
			logger.Error("Failed to fetch users", zap.Error(fetchErr))
			scraperMetrics.FetchErrors.Add(ctx, 1, metric.WithAttributes(providerAttr))
			continue
		}

		scraperMetrics.BatchSize.Record(ctx, int64(len(users)), metric.WithAttributes(providerAttr))

		if len(users) == 0 {
			logger.Info(
				"Reached end of users list",
				zap.Int("total_processed", totalProcessed),
				zap.Int("total_ingested", totalIngested),
				zap.Duration("duration", time.Since(startTime)),
			)
			return nil
		}

		for _, user := range users {
			if ctxErr := ctx.Err(); ctxErr != nil {
				break
			}

			totalProcessed++
			scraperMetrics.UsersProcessed.Add(ctx, 1, metric.WithAttributes(providerAttr))

			ingested, processErr := processor.process(ctx, user, limiter)
			if processErr != nil {
				return processErr
			}

			if ingested {
				totalIngested++
			}

			if totalProcessed%100 == 0 {
				logger.Info(
					"Progress",
					zap.Int("processed", totalProcessed),
					zap.Int("ingested", totalIngested),
					zap.Int("page", page),
					zap.String("last_user", user.Username),
				)
			}
		}

		page++
		currentPage = int64(page)
		if saveErr := progressTracker.SetLastPage(ctx, page); saveErr != nil {
			logger.Error("Failed to save progress", zap.Error(saveErr))
		}
	}
}

type githubUserProcessor struct {
	service         *ingester.GitHubService
	grepo           *githubrepository.Repository
	scraperMetrics  *internalmetrics.ScraperMetrics
	providerAttr    attribute.KeyValue
	logger          *zap.Logger
	progressTracker *github.ProgressTracker
}

func (p *githubUserProcessor) process(
	ctx context.Context, user github.User, limiter *rate.Limiter,
) (int64, bool, error) {
	waitStart := time.Now()
	if waitErr := limiter.Wait(ctx); waitErr != nil {
		return 0, false, fmt.Errorf("rate limiter error: %w", waitErr)
	}
	p.scraperMetrics.RateLimitWait.Record(ctx, time.Since(waitStart).Seconds(), metric.WithAttributes(p.providerAttr))

	username := githubdomain.Username(user.Login)

	ingestStart := time.Now()
	ingestErr := p.service.Ingest(ctx, user.Login)
	p.scraperMetrics.ScrapeDuration.Record(ctx, time.Since(ingestStart).Seconds(), metric.WithAttributes(p.providerAttr))
	success := ingestErr == nil

	userExists, existErr := p.grepo.Exist(ctx, username)
	if existErr != nil {
		p.logger.Warn("Failed to check user existence",
			zap.String("username", user.Login),
			zap.Error(existErr),
		)
	} else if userExists {
		if updateErr := p.grepo.UpdateScrapeMetadata(ctx, username, success); updateErr != nil {
			p.logger.Warn("Failed to update scrape metadata",
				zap.String("username", user.Login),
				zap.Error(updateErr),
			)
		}
	}

	if ingestErr != nil {
		p.logger.Warn("Failed to ingest user",
			zap.String("username", user.Login),
			zap.Error(ingestErr),
		)
		p.scraperMetrics.IngestErrors.Add(ctx, 1, metric.WithAttributes(p.providerAttr))
		return user.ID, false, nil
	}

	p.scraperMetrics.UsersIngested.Add(ctx, 1, metric.WithAttributes(p.providerAttr))
	return user.ID, true, nil
}

type gitlabUserProcessor struct {
	service         *ingester.GitLabService
	grepo           *gitlabrepository.Repository
	scraperMetrics  *internalmetrics.ScraperMetrics
	providerAttr    attribute.KeyValue
	logger          *zap.Logger
	progressTracker *gitlab.ProgressTracker
}

func (p *gitlabUserProcessor) process(ctx context.Context, user gitlab.User, limiter *rate.Limiter) (bool, error) {
	waitStart := time.Now()
	if waitErr := limiter.Wait(ctx); waitErr != nil {
		return false, fmt.Errorf("rate limiter error: %w", waitErr)
	}
	p.scraperMetrics.RateLimitWait.Record(ctx, time.Since(waitStart).Seconds(), metric.WithAttributes(p.providerAttr))

	username := gitlabdomain.Username(user.Username)

	ingestStart := time.Now()
	ingestErr := p.service.Ingest(ctx, user.Username)
	p.scraperMetrics.ScrapeDuration.Record(ctx, time.Since(ingestStart).Seconds(), metric.WithAttributes(p.providerAttr))
	success := ingestErr == nil

	userExists, existErr := p.grepo.Exist(ctx, username)
	if existErr != nil {
		p.logger.Warn("Failed to check user existence",
			zap.String("username", user.Username),
			zap.Error(existErr),
		)
	} else if userExists {
		if updateErr := p.grepo.UpdateScrapeMetadata(ctx, username, success); updateErr != nil {
			p.logger.Warn("Failed to update scrape metadata",
				zap.String("username", user.Username),
				zap.Error(updateErr),
			)
		}
	}

	if ingestErr != nil {
		p.logger.Warn("Failed to ingest user",
			zap.String("username", user.Username),
			zap.Error(ingestErr),
		)
		p.scraperMetrics.IngestErrors.Add(ctx, 1, metric.WithAttributes(p.providerAttr))
		return false, nil
	}

	p.scraperMetrics.UsersIngested.Add(ctx, 1, metric.WithAttributes(p.providerAttr))
	return true, nil
}
