// Package ingester provides services for ingesting SSH keys from external sources.
package ingester

import (
	"context"
	"errors"
	"fmt"

	"github.com/merlindorin/sshark-api/internal/domain/github"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

// Service handles the ingestion of SSH keys from GitHub users.
type Service struct {
	githubRepository  github.Repository
	sshkeysRepository sshkeys.Repository
	fetcher           github.KeyFetcher
}

// New creates a new ingester Service with the given dependencies.
func New(githubRepository github.Repository, sshkeysRepository sshkeys.Repository, fetcher github.KeyFetcher) *Service {
	return &Service{
		githubRepository:  githubRepository,
		sshkeysRepository: sshkeysRepository,
		fetcher:           fetcher,
	}
}

// Ingest fetches and stores SSH keys for a GitHub username.
// It validates the username, creates a GitHub user record if new,
// then fetches and stores the user's public SSH keys.
func (s *Service) Ingest(ctx context.Context, username string) error {
	if !github.IsValid(username) {
		return nil
	}

	githubUsername := github.Username(username)

	err := s.githubRepository.Create(ctx, githubUsername, &github.User{})
	if errors.Is(err, github.ErrUserAlreadyExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("cannot create github user: %w", err)
	}

	authorizedKeys := github.NewAuthorizedKeys()

	err = s.fetcher.FetchAuthorizedKeys(ctx, githubUsername, authorizedKeys)
	if err != nil {
		return fmt.Errorf("cannot fetch github keys: %w", err)
	}

	err = s.sshkeysRepository.CreateFromAuthorizedKeys(ctx, authorizedKeys, nil)
	if err != nil {
		return fmt.Errorf("cannot create sshkeys: %w", err)
	}

	return nil
}
