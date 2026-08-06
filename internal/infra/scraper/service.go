package scraper

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
	"github.com/merlindorin/sshark-api/internal/domain/scraper"
	"github.com/merlindorin/sshark-api/internal/domain/sources"
	"github.com/merlindorin/sshark-api/internal/metrics"
)

// Service implements a continuous scraper that fetches users and their keys.
type Service struct {
	logger         *zap.Logger
	fetcher        scraper.Fetcher
	sourcesRepo    sources.Repository
	publickeysRepo publickeys.Repository
	progressRepo   scraper.ProgressRepository
	metrics        *metrics.Metrics

	// Configuration
	batchSize int
	delay     time.Duration
}

// Config holds scraper configuration.
type Config struct {
	BatchSize int           // Number of users to fetch per batch
	Delay     time.Duration // Delay between batches
}

// NewService creates a new scraper service.
func NewService(
	logger *zap.Logger,
	fetcher scraper.Fetcher,
	sourcesRepo sources.Repository,
	publickeysRepo publickeys.Repository,
	progressRepo scraper.ProgressRepository,
	cfg Config,
	m *metrics.Metrics,
) *Service {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.Delay <= 0 {
		cfg.Delay = time.Second
	}
	return &Service{
		logger:         logger.Named("scraper"),
		fetcher:        fetcher,
		sourcesRepo:    sourcesRepo,
		publickeysRepo: publickeysRepo,
		progressRepo:   progressRepo,
		metrics:        m,
		batchSize:      cfg.BatchSize,
		delay:          cfg.Delay,
	}
}

// Run starts the continuous scraping loop.
// It will run until the context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	provider := s.fetcher.Provider()
	s.logger.Info("starting scraper",
		zap.String("provider", string(provider)),
		zap.Int("batch_size", s.batchSize),
		zap.Duration("delay", s.delay),
	)

	// Load progress
	progress, err := s.progressRepo.GetProgress(ctx, provider)
	if err != nil {
		return fmt.Errorf("cannot load progress data: %w", err)
	}

	cursor := progress.LastCursor
	s.logger.Info("resuming from cursor", zap.String("cursor", cursor))

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scraper stopped")
			return ctx.Err()
		default:
		}

		// Fetch next batch of users
		page, fetchErr := s.fetcher.ListUsers(ctx, cursor, s.batchSize)
		if fetchErr != nil {
			if errors.Is(fetchErr, scraper.ErrRateLimited) {
				s.logger.Warn("rate limited, waiting...")
				if s.metrics != nil {
					s.metrics.ScrapingRateLimitHits.Add(ctx, 1,
						metrics.WithProvider(string(provider)))
				}
				s.sleep(ctx, time.Minute)
				continue
			}
			s.logger.Error("failed to list users", zap.Error(fetchErr))
			s.sleep(ctx, s.delay)
			continue
		}

		if len(page.Users) == 0 {
			s.logger.Info("no more users, waiting before restart...")
			// Reset cursor to start over
			cursor = ""
			s.sleep(ctx, time.Hour)
			continue
		}

		// Process each user
		for i := range page.Users {
			user := &page.Users[i]
			_ = s.processUser(ctx, user)
		}

		// Save progress
		cursor = page.NextCursor
		progress.LastCursor = cursor
		if saveErr := s.progressRepo.SaveProgress(ctx, progress); saveErr != nil {
			s.logger.Error("failed to save progress", zap.Error(saveErr))
		}

		s.logger.Info("batch completed",
			zap.Int("users", len(page.Users)),
			zap.String("next_cursor", cursor),
		)

		s.sleep(ctx, s.delay)
	}
}

// ScrapeUser scrapes keys for a single user from the provider this service is bound to.
// It is the on-demand counterpart of Run: it refreshes one account immediately, picking up
// keys added since the last crawl and dropping keys removed upstream.
func (s *Service) ScrapeUser(
	ctx context.Context,
	provider scraper.Provider,
	username string,
) (*scraper.ScrapeResult, error) {
	start := time.Now()
	providerStr := string(provider)

	if provider != s.fetcher.Provider() {
		return nil, fmt.Errorf("%w: %s", scraper.ErrProviderUnavailable, provider)
	}

	user, err := s.fetcher.FetchUser(ctx, username)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ScrapingRequestsTotal.Add(ctx, 1,
				metrics.WithProvider(providerStr),
				metrics.WithStatus("failure"))
			s.metrics.ScrapingDuration.Record(ctx, time.Since(start).Seconds(),
				metrics.WithProvider(providerStr),
				metrics.WithStatus("failure"))
			s.metrics.ScrapingErrors.Add(ctx, 1,
				metrics.WithProvider(providerStr),
				metrics.WithErrorType(categorizeScraperError(err)))
		}
		return nil, err
	}

	result := s.processUser(ctx, user)

	if s.metrics != nil {
		status := "success"
		if result.Error != nil {
			status = "failure"
		}
		s.metrics.ScrapingRequestsTotal.Add(ctx, 1,
			metrics.WithProvider(providerStr),
			metrics.WithStatus(status))
		s.metrics.ScrapingDuration.Record(ctx, time.Since(start).Seconds(),
			metrics.WithProvider(providerStr),
			metrics.WithStatus(status))

		if result.Error == nil {
			keysDiscovered := int64(result.KeysAdded + result.KeysUpdated)
			s.metrics.ScrapingKeysDiscovered.Add(ctx, keysDiscovered,
				metrics.WithProvider(providerStr))
		} else {
			s.metrics.ScrapingErrors.Add(ctx, 1,
				metrics.WithProvider(providerStr),
				metrics.WithErrorType(categorizeScraperError(result.Error)))
		}
	}

	return &result, nil
}

// ScrapeUsers scrapes keys for multiple users from the provider this service is bound to.
func (s *Service) ScrapeUsers(
	ctx context.Context,
	provider scraper.Provider,
	usernames []string,
) ([]scraper.ScrapeResult, error) {
	results := make([]scraper.ScrapeResult, 0, len(usernames))

	for _, username := range usernames {
		result, err := s.ScrapeUser(ctx, provider, username)
		if err != nil {
			results = append(results, scraper.ScrapeResult{
				Provider: provider,
				Username: username,
				Error:    err,
			})
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

func (s *Service) processUser(ctx context.Context, user *scraper.FetchedUser) scraper.ScrapeResult {
	result := scraper.ScrapeResult{
		Provider: s.fetcher.Provider(),
		Username: user.Username,
	}

	// Fetch SSH keys for the user
	if err := s.fetcher.FetchUserKeys(ctx, user); err != nil {
		if errors.Is(err, scraper.ErrRateLimited) {
			s.logger.Warn("rate limited fetching SSH keys", zap.String("username", user.Username))
			result.Error = err
			return result
		}
		s.logger.Warn("failed to fetch SSH keys",
			zap.String("username", user.Username),
			zap.Error(err),
		)
		result.Error = err
	}

	// Fetch GPG keys for the user
	if err := s.fetcher.FetchUserGPGKeys(ctx, user); err != nil {
		if errors.Is(err, scraper.ErrRateLimited) {
			s.logger.Warn("rate limited fetching GPG keys", zap.String("username", user.Username))
		} else {
			s.logger.Warn("failed to fetch GPG keys",
				zap.String("username", user.Username),
				zap.Error(err),
			)
		}
		// Continue - GPG keys are optional
	}

	// Skip users with no keys at all
	if len(user.Keys) == 0 && len(user.GPGKeys) == 0 {
		return result
	}

	// Get or create source
	source, err := s.getOrCreateSource(ctx, user)
	if err != nil {
		s.logger.Error("failed to get/create source",
			zap.String("username", user.Username),
			zap.Error(err),
		)
		result.Error = err
		return result
	}

	// Sync SSH keys
	if len(user.Keys) > 0 {
		s.syncKeys(ctx, source.ID, user.Keys, publickeys.KeyTypeSSH, &result)
	}

	// Sync GPG keys
	if len(user.GPGKeys) > 0 {
		s.syncKeys(ctx, source.ID, user.GPGKeys, publickeys.KeyTypeGPG, &result)
	}

	return result
}

func (s *Service) getOrCreateSource(
	ctx context.Context,
	user *scraper.FetchedUser,
) (*sources.Entity, error) {
	provider := string(s.fetcher.Provider())

	// Try to find existing source
	source, err := s.sourcesRepo.GetByProviderAndUserID(ctx, provider, user.UserID)
	if err == nil {
		// Update if username or URI changed
		if source.Username != user.Username || source.URI != user.URI {
			source.Username = user.Username
			source.URI = user.URI
			if updateErr := s.sourcesRepo.Update(ctx, source); updateErr != nil {
				s.logger.Warn("failed to update source", zap.Error(updateErr))
			}
		}
		return source, nil
	}

	if !errors.Is(err, sources.ErrSourceNotFound) {
		return nil, err
	}

	// Create new source
	source = &sources.Entity{
		ID:       uuid.New(),
		Provider: provider,
		UserID:   user.UserID,
		Username: user.Username,
		URI:      user.URI,
	}
	if createErr := s.sourcesRepo.Create(ctx, source); createErr != nil {
		return nil, createErr
	}

	s.logger.Debug("created source",
		zap.String("username", user.Username),
		zap.String("user_id", user.UserID),
	)

	return source, nil
}

func (s *Service) syncKeys(
	ctx context.Context,
	sourceID uuid.UUID,
	fetchedKeys []scraper.FetchedKey,
	keyType publickeys.KeyType,
	result *scraper.ScrapeResult,
) {
	// Get existing keys for this source and type
	existingKeys, err := s.publickeysRepo.Search(ctx, publickeys.SearchFilter{
		SourceID: &sourceID,
		KeyType:  &keyType,
	}, 1000, 0)
	if err != nil {
		s.logger.Error("failed to get existing keys", zap.Error(err))
		return
	}

	// Build fingerprint map of existing keys
	existingByFingerprint := make(map[string]*publickeys.Entity)
	for i := range existingKeys.Entities {
		key := &existingKeys.Entities[i]
		if key.Fingerprint != "" {
			existingByFingerprint[key.Fingerprint] = key
		}
	}

	// Process fetched keys
	seenFingerprints := make(map[string]bool)
	for _, fetchedKey := range fetchedKeys {
		if fetchedKey.Fingerprint == "" {
			continue
		}
		seenFingerprints[fetchedKey.Fingerprint] = true

		existing, exists := existingByFingerprint[fetchedKey.Fingerprint]
		if exists {
			if s.updateExistingKey(ctx, existing, fetchedKey, keyType) {
				result.KeysUpdated++
			}
		} else if s.createNewKey(ctx, sourceID, fetchedKey, keyType) {
			result.KeysAdded++
		}
	}

	// Remove keys that no longer exist
	for fingerprint, existing := range existingByFingerprint {
		if seenFingerprints[fingerprint] {
			continue
		}
		if deleteErr := s.publickeysRepo.Delete(ctx, existing.ID); deleteErr != nil {
			s.logger.Warn("failed to delete stale key", zap.Error(deleteErr))
			continue
		}
		result.KeysRemoved++
	}
}

// updateExistingKey refreshes a stored key from the provider payload.
// It reports whether the stored key actually changed.
func (s *Service) updateExistingKey(
	ctx context.Context,
	existing *publickeys.Entity,
	fetchedKey scraper.FetchedKey,
	keyType publickeys.KeyType,
) bool {
	if !s.keyNeedsUpdate(existing, &fetchedKey, keyType) {
		s.recordScrapeHistory(ctx, existing.ID, true, false)
		return false
	}

	existing.KeyData = fetchedKey.KeyData
	existing.ProviderKeyID = providerKeyID(fetchedKey)

	switch keyType {
	case publickeys.KeyTypeSSH:
		existing.SSHMetadata = &publickeys.SSHMetadata{
			Algorithm: fetchedKey.Algorithm,
			Comment:   fetchedKey.Comment,
			KeyBits:   fetchedKey.KeyBits,
		}
	case publickeys.KeyTypeGPG:
		existing.GPGMetadata = &publickeys.GPGMetadata{
			Algorithm:    fetchedKey.Algorithm,
			KeyBits:      fetchedKey.KeyBits,
			ExpiresAt:    fetchedKey.ExpiresAt,
			UserIDs:      fetchedKey.UserIDs,
			Capabilities: fetchedKey.Capabilities,
		}
	}

	if updateErr := s.publickeysRepo.Update(ctx, existing); updateErr != nil {
		s.logger.Warn("failed to update key", zap.Error(updateErr))
		return false
	}

	s.recordScrapeHistory(ctx, existing.ID, true, true)

	return true
}

// createNewKey stores a key discovered on the provider.
// It reports whether the key was persisted.
func (s *Service) createNewKey(
	ctx context.Context,
	sourceID uuid.UUID,
	fetchedKey scraper.FetchedKey,
	keyType publickeys.KeyType,
) bool {
	newKey := &publickeys.Entity{
		ID:            uuid.New(),
		SourceID:      sourceID,
		KeyType:       keyType,
		KeyData:       fetchedKey.KeyData,
		ProviderKeyID: providerKeyID(fetchedKey),
		Fingerprint:   fetchedKey.Fingerprint,
	}

	switch keyType {
	case publickeys.KeyTypeSSH:
		newKey.SSHMetadata = &publickeys.SSHMetadata{
			Algorithm: fetchedKey.Algorithm,
			Comment:   fetchedKey.Comment,
			KeyBits:   fetchedKey.KeyBits,
		}
	case publickeys.KeyTypeGPG:
		newKey.GPGMetadata = &publickeys.GPGMetadata{
			Algorithm:    fetchedKey.Algorithm,
			KeyBits:      fetchedKey.KeyBits,
			ExpiresAt:    fetchedKey.ExpiresAt,
			UserIDs:      fetchedKey.UserIDs,
			Capabilities: fetchedKey.Capabilities,
		}
	}

	if createErr := s.publickeysRepo.Create(ctx, newKey); createErr != nil {
		s.logger.Warn("failed to create key", zap.Error(createErr))
		return false
	}

	s.recordScrapeHistory(ctx, newKey.ID, true, true)

	return true
}

func providerKeyID(fetchedKey scraper.FetchedKey) *string {
	if fetchedKey.KeyID == "" {
		return nil
	}

	return &fetchedKey.KeyID
}

func (s *Service) keyNeedsUpdate(
	existing *publickeys.Entity,
	fetched *scraper.FetchedKey,
	keyType publickeys.KeyType,
) bool {
	if string(existing.KeyData) != string(fetched.KeyData) {
		return true
	}

	switch keyType {
	case publickeys.KeyTypeSSH:
		return s.sshMetadataChanged(existing.SSHMetadata, fetched)
	case publickeys.KeyTypeGPG:
		return s.gpgMetadataChanged(existing.GPGMetadata, fetched)
	}

	return false
}

func (s *Service) sshMetadataChanged(meta *publickeys.SSHMetadata, fetched *scraper.FetchedKey) bool {
	if meta == nil {
		return false
	}
	if meta.Algorithm != fetched.Algorithm {
		return true
	}
	if meta.Comment != fetched.Comment {
		return true
	}
	return false
}

func (s *Service) gpgMetadataChanged(meta *publickeys.GPGMetadata, fetched *scraper.FetchedKey) bool {
	if meta == nil {
		return false
	}
	if meta.Algorithm != fetched.Algorithm {
		return true
	}
	// Check if expiration changed
	if (meta.ExpiresAt == nil) != (fetched.ExpiresAt == nil) {
		return true
	}
	if meta.ExpiresAt != nil && fetched.ExpiresAt != nil && !meta.ExpiresAt.Equal(*fetched.ExpiresAt) {
		return true
	}
	return false
}

func (s *Service) recordScrapeHistory(
	ctx context.Context,
	keyID uuid.UUID,
	success bool,
	keyChanged bool,
) {
	history := &publickeys.ScrapeHistory{
		ID:         uuid.New(),
		KeyID:      keyID,
		ScrapedAt:  time.Now(),
		Success:    success,
		KeyChanged: keyChanged,
	}
	if histErr := s.publickeysRepo.AddScrapeHistory(ctx, history); histErr != nil {
		s.logger.Warn("failed to record scrape history", zap.Error(histErr))
	}
}

func (s *Service) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// categorizeScraperError categorizes errors for metrics labeling.
func categorizeScraperError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, scraper.ErrRateLimited) {
		return "api_limit"
	}
	if errors.Is(err, scraper.ErrProviderUnavailable) {
		return "unavailable"
	}
	if errors.Is(err, scraper.ErrUserNotFound) {
		return "not_found"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	// Default to network for other errors
	return "network"
}
