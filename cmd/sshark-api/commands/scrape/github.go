package scrape

import (
	"context"

	"github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/github"
)

type Github struct {
	GithubToken string `env:"GITHUB_TOKEN" help:"GitHub API token"`
}

func (s *Github) Run(ctx context.Context, common *cmd.Commons, scrape *Scrape, postgres *globals.Postgres) error {
	return process(
		ctx,
		postgres,
		github.NewFetcher(github.WithToken(s.GithubToken)),
		scrape.BatchSize,
		scrape.Delay,
		common.MustLogger(),
	)
}
