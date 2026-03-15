package fetch

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/scraper"
)

type Fetch struct {
	Cursor    string `help:"GitHub cursor position" default:""`
	BatchSize int    `help:"Number of users to fetch per batch" default:"10"`

	Github Github `cmd:"" help:"Scrape SSH keys from providers"`
	Gitlab Gitlab `cmd:"" help:"Scrape SSH keys from providers"`
}

func process(ctx context.Context, fetcher scraper.Fetcher, cursor string, size int, logger *zap.Logger) error {
	logger.Info("starting fetch",
		zap.String("provider", string(fetcher.Provider())),
		zap.String("cursor", cursor),
		zap.Int("batch_size", size),
	)

	users, err := fetcher.ListUsers(ctx, cursor, size)
	if err != nil {
		return fmt.Errorf("cannot list users: %w", err)
	}

	logger.Info("users found", zap.Int("count", len(users.Users)))

	data := [][]string{}

	for i := range users.Users {
		user := &users.Users[i]
		logger.Info("fetching keys for user",
			zap.String("userid", user.UserID),
			zap.String("username", user.Username))

		if fetchErr := fetcher.FetchUserKeys(ctx, user); fetchErr != nil {
			return fetchErr
		}

		if len(user.Keys) == 0 {
			data = append(data, []string{
				user.UserID,
				user.Username,
				user.URI,
				"",
				"",
			})
		}

		for _, key := range user.Keys {
			data = append(data, []string{
				user.UserID,
				user.Username,
				user.URI,
				key.KeyID,
				key.Fingerprint,
			})
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"UserID", "Username", "URI", "KeyID", "FingerPrint"})
	table.AppendBulk(data)
	table.SetAutoMergeCells(true)
	table.Render()

	return nil
}
