package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/merlindorin/go-shared/pkg/cmd"
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

	redisClient := redis.Client()

	fetcher := github.NewFetcher(logger)
	usersFetcher := github.NewUsersFetcher(logger)
	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	grepo := githubrepository.NewRepository(redisClient)
	service := ingester.NewGitHub(grepo, srepo, fetcher)
	progressTracker := github.NewProgressTracker(redisClient)

	if ensureErr := grepo.EnsureIndex(ctx, false); ensureErr != nil {
		return fmt.Errorf("failed to ensure GitHub users index: %w", ensureErr)
	}

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

	totalProcessed := 0
	totalIngested := 0
	startTime := time.Now()

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

		if waitErr := limiter.Wait(ctx); waitErr != nil {
			return fmt.Errorf("rate limiter error: %w", waitErr)
		}

		users, fetchErr := usersFetcher.FetchUsers(ctx, lastID, s.BatchSize)
		if fetchErr != nil {
			logger.Error("Failed to fetch users", zap.Error(fetchErr))
			continue
		}

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

			if waitErr := limiter.Wait(ctx); waitErr != nil {
				return fmt.Errorf("rate limiter error: %w", waitErr)
			}

			username := githubdomain.Username(user.Login)

			ingestErr := service.Ingest(ctx, user.Login)
			success := ingestErr == nil

			userExists, existErr := grepo.Exist(ctx, username)
			if existErr != nil {
				logger.Warn("Failed to check user existence",
					zap.String("username", user.Login),
					zap.Error(existErr),
				)
			} else if userExists {
				if updateErr := grepo.UpdateScrapeMetadata(ctx, username, success); updateErr != nil {
					logger.Warn("Failed to update scrape metadata",
						zap.String("username", user.Login),
						zap.Error(updateErr),
					)
				}
			}

			if ingestErr != nil {
				logger.Warn("Failed to ingest user",
					zap.String("username", user.Login),
					zap.Error(ingestErr),
				)
				lastID = user.ID
				continue
			}

			totalIngested++
			lastID = user.ID

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

	redisClient := redis.Client()

	fetcher := gitlab.NewFetcher(logger, s.GitLabToken)
	usersFetcher := gitlab.NewUsersFetcher(logger, s.GitLabToken)
	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	grepo := gitlabrepository.NewRepository(redisClient)
	service := ingester.NewGitLab(grepo, srepo, fetcher)
	progressTracker := gitlab.NewProgressTracker(redisClient)

	if ensureErr := grepo.EnsureIndex(ctx, false); ensureErr != nil {
		return fmt.Errorf("failed to ensure GitLab users index: %w", ensureErr)
	}

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

	totalProcessed := 0
	totalIngested := 0
	startTime := time.Now()

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

		if waitErr := limiter.Wait(ctx); waitErr != nil {
			return fmt.Errorf("rate limiter error: %w", waitErr)
		}

		users, fetchErr := usersFetcher.FetchUsers(ctx, page, s.BatchSize)
		if fetchErr != nil {
			logger.Error("Failed to fetch users", zap.Error(fetchErr))
			continue
		}

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

			if waitErr := limiter.Wait(ctx); waitErr != nil {
				return fmt.Errorf("rate limiter error: %w", waitErr)
			}

			username := gitlabdomain.Username(user.Username)

			ingestErr := service.Ingest(ctx, user.Username)
			success := ingestErr == nil

			userExists, existErr := grepo.Exist(ctx, username)
			if existErr != nil {
				logger.Warn("Failed to check user existence",
					zap.String("username", user.Username),
					zap.Error(existErr),
				)
			} else if userExists {
				if updateErr := grepo.UpdateScrapeMetadata(ctx, username, success); updateErr != nil {
					logger.Warn("Failed to update scrape metadata",
						zap.String("username", user.Username),
						zap.Error(updateErr),
					)
				}
			}

			if ingestErr != nil {
				logger.Warn("Failed to ingest user",
					zap.String("username", user.Username),
					zap.Error(ingestErr),
				)
				continue
			}

			totalIngested++

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
		if saveErr := progressTracker.SetLastPage(ctx, page); saveErr != nil {
			logger.Error("Failed to save progress", zap.Error(saveErr))
		}
	}
}
