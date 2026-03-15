package fetch

import (
	"context"

	"github.com/merlindorin/go-shared/pkg/cmd"

	"github.com/merlindorin/sshark-api/internal/infra/fetchers/gitlab"
)

type Gitlab struct {
	GitlabToken string `env:"GITLAB_TOKEN" help:"GitLab API token"`
}

func (s *Gitlab) Run(ctx context.Context, common *cmd.Commons, f *Fetch) error {
	return process(
		ctx,
		gitlab.NewFetcher(gitlab.WithToken(s.GitlabToken)),
		f.Cursor,
		f.BatchSize,
		common.MustLogger().Named("fetch"),
	)
}
