// Package ingester provides services for ingesting SSH keys from external sources.
package ingester

import (
	"context"
	"errors"
	"fmt"

	"github.com/merlindorin/sshark-api/internal/domain/github"
	"github.com/merlindorin/sshark-api/internal/domain/gitlab"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

// GitHubService handles the ingestion of SSH keys from GitHub users.
type GitHubService struct {
	githubRepository  github.Repository
	sshkeysRepository sshkeys.Repository
	fetcher           github.KeyFetcher
}

// NewGitHub creates a new GitHub ingester Service with the given dependencies.
func NewGitHub(
	githubRepository github.Repository,
	sshkeysRepository sshkeys.Repository,
	fetcher github.KeyFetcher,
) *GitHubService {
	return &GitHubService{
		githubRepository:  githubRepository,
		sshkeysRepository: sshkeysRepository,
		fetcher:           fetcher,
	}
}

// Ingest fetches and stores SSH keys for a GitHub username.
// It validates the username, creates a GitHub user record if new,
// then fetches and stores the user's public SSH keys.
//
//nolint:dupl // Intentional duplication for different provider types
func (s *GitHubService) Ingest(ctx context.Context, username string) error {
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

	genericKeys := &sshkeys.AuthorizedKeys{
		Username: authorizedKeys.Username.String(),
		Source:   authorizedKeys.Source,
		Keys:     authorizedKeys.Keys,
	}

	err = s.sshkeysRepository.CreateFromAuthorizedKeys(ctx, genericKeys, "github", nil)
	if err != nil {
		return fmt.Errorf("cannot create sshkeys: %w", err)
	}

	return nil
}

// GitLabService handles the ingestion of SSH keys from GitLab users.
type GitLabService struct {
	gitlabRepository  gitlab.Repository
	sshkeysRepository sshkeys.Repository
	fetcher           gitlab.KeyFetcher
}

// NewGitLab creates a new GitLab ingester Service with the given dependencies.
func NewGitLab(
	gitlabRepository gitlab.Repository,
	sshkeysRepository sshkeys.Repository,
	fetcher gitlab.KeyFetcher,
) *GitLabService {
	return &GitLabService{
		gitlabRepository:  gitlabRepository,
		sshkeysRepository: sshkeysRepository,
		fetcher:           fetcher,
	}
}

// Ingest fetches and stores SSH keys for a GitLab username.
// It validates the username, creates a GitLab user record if new,
// then fetches and stores the user's public SSH keys.
//
//nolint:dupl // Intentional duplication for different provider types
func (s *GitLabService) Ingest(ctx context.Context, username string) error {
	if !gitlab.IsValid(username) {
		return nil
	}

	gitlabUsername := gitlab.Username(username)

	err := s.gitlabRepository.Create(ctx, gitlabUsername, &gitlab.User{})
	if errors.Is(err, gitlab.ErrUserAlreadyExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("cannot create gitlab user: %w", err)
	}

	authorizedKeys := gitlab.NewAuthorizedKeys()

	err = s.fetcher.FetchAuthorizedKeys(ctx, gitlabUsername, authorizedKeys)
	if err != nil {
		return fmt.Errorf("cannot fetch gitlab keys: %w", err)
	}

	genericKeys := &sshkeys.AuthorizedKeys{
		Username: authorizedKeys.Username.String(),
		Source:   authorizedKeys.Source,
		Keys:     authorizedKeys.Keys,
	}

	err = s.sshkeysRepository.CreateFromAuthorizedKeys(ctx, genericKeys, "gitlab", nil)
	if err != nil {
		return fmt.Errorf("cannot create sshkeys: %w", err)
	}

	return nil
}
