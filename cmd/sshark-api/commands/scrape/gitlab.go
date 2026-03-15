package scrape

import (
	"context"

	"github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
	"github.com/merlindorin/sshark-api/internal/infra/fetchers/gitlab"
)

type Gitlab struct {
	GitlabToken string `env:"GITLAB_TOKEN" help:"Gitlab API token"`
}

func (s *Gitlab) Run(ctx context.Context, common *cmd.Commons, scrape *Scrape, postgres *globals.Postgres) error {
	return process(
		ctx,
		postgres,
		gitlab.NewFetcher(gitlab.WithToken(s.GitlabToken)),
		scrape.BatchSize,
		scrape.Delay,
		common.MustLogger(),
	)
}
