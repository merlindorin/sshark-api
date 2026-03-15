package scrape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	publickeyspostgres "github.com/merlindorin/sshark-api/internal/infra/publickeys/postgres"
	scraperservice "github.com/merlindorin/sshark-api/internal/infra/scraper"
	scraperpostgres "github.com/merlindorin/sshark-api/internal/infra/scraper/postgres"
	sourcespostgres "github.com/merlindorin/sshark-api/internal/infra/sources/postgres"
)

type Scrape struct {
	BatchSize int           `help:"Number of users to fetch per batch" default:"100"`
	Delay     time.Duration `help:"Delay between batches" default:"1s"`

	Github Github `cmd:"" help:"Scrape SSH keys from providers"`
	Gitlab Gitlab `cmd:"" help:"Scrape SSH keys from providers"`
}

func process(
	ctx context.Context,
	postgres *globals.Postgres,
	fetcher scraper.Fetcher,
	size int,
	delay time.Duration,
	logger *zap.Logger,
) error {
	logger = logger.Named("scrape")

	logger.Info("starting scraper",
		zap.String("provider", string(fetcher.Provider())),
		zap.Int("batch_size", size),
		zap.Duration("delay", delay),
	)

	// Connect to PostgreSQL
	pool, err := postgres.Pool(ctx)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()

	// Create repositories
	sourcesRepo := sourcespostgres.NewRepository(pool)
	publickeysRepo := publickeyspostgres.NewRepository(pool)
	progressRepo := scraperpostgres.NewProgressRepository(pool)

	// Create scraper service
	svc := scraperservice.NewService(
		logger,
		fetcher,
		sourcesRepo,
		publickeysRepo,
		progressRepo,
		scraperservice.Config{
			BatchSize: size,
			Delay:     delay,
		},
	)

	// Setup signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", zap.String("signal", sig.String()))
		cancel()
	}()

	// Run the scraper
	runErr := svc.Run(ctx)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}

	logger.Info("scraper stopped gracefully")
	return nil
}
