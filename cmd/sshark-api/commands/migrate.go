package commands

import (
	"context"
	"fmt"

	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
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
	grepo := githubrepository.NewRepository(redisClient)

	err := srepo.EnsureIndex(ctx, s.ForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}

	err = grepo.EnsureIndex(context.Background(), s.ForceReindex)
	if err != nil {
		return fmt.Errorf("failed to ensure index: %w", err)
	}
	return nil
}
