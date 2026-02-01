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
	"github.com/merlindorin/sshark-api/internal/domain/ingester"
	"github.com/merlindorin/sshark-api/internal/infra/github"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
)

type Scrape struct {
	RateLimit float64 `help:"Requests per second to GitHub API" default:"2.0"`
	BatchSize int     `help:"Number of users to fetch per GitHub API call" default:"100"`
}

func (s *Scrape) Run(ctx context.Context, common *cmd.Commons, redis *globals.Redis) error { //nolint:funlen
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
	service := ingester.New(grepo, srepo, fetcher)
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

			ingestErr := service.Ingest(ctx, user.Login)
			success := ingestErr == nil

			if updateErr := grepo.UpdateScrapeMetadata(ctx, githubdomain.Username(user.Login), success); updateErr != nil {
				logger.Warn("Failed to update scrape metadata",
					zap.String("username", user.Login),
					zap.Error(updateErr),
				)
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
