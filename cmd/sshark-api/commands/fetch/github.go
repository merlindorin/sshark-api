package fetch

import (
	"context"

	"github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/internal/infra/fetchers/github"
)

type Github struct {
	GithubToken string `env:"GITHUB_TOKEN" help:"GitHub API token"`
}

func (s *Github) Run(ctx context.Context, common *cmd.Commons, f *Fetch) error {
	return process(
		ctx,
		github.NewFetcher(github.WithToken(s.GithubToken)),
		f.Cursor,
		f.BatchSize,
		common.MustLogger().Named("fetch"),
	)
}
