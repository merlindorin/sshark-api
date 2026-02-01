package commands

import (
	"context"
	"fmt"

	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	gitlabrepository "github.com/merlindorin/sshark-api/internal/infra/gitlab/redis"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
)

type Migrate struct {
	ForceReindex bool `help:"Force recreation of index" default:"true"`
}

func (s *Migrate) Run(ctx context.Context, common *cmd.Commons, redis *globals.Redis) error {
	logger := common.MustLogger().Named("server")

	logger.Info(
		"Migrating...",
		zap.String("name", fmt.Sprintf("%s-migration", common.Version.Name())),
		zap.String("version", common.Version.Version()),
		zap.String("redis.address", redis.Addr()),
		zap.Int("redis.db", redis.DB),
	)

	redisClient := redis.Client()

	srepo := sshkeysrepository.NewRedisRepository(redisClient)
	githubRepo := githubrepository.NewRepository(redisClient)
	gitlabRepo := gitlabrepository.NewRepository(redisClient)

	err := srepo.EnsureIndex(ctx, s.ForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure SSH keys index: %w", err)
	}

	err = githubRepo.EnsureIndex(ctx, s.ForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure GitHub users index: %w", err)
	}

	err = gitlabRepo.EnsureIndex(ctx, s.ForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure GitLab users index: %w", err)
	}

	logger.Info("Initializing stats counters...")
	err = srepo.InitializeStatsCounters(ctx)
	if err != nil {
		logger.Warn("Failed to initialize stats counters", zap.Error(err))
		// Don't fail migration if stats initialization fails
	} else {
		logger.Info("Stats counters initialized successfully")
	}

	return nil
}
