package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/ssh"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/domain/stats"
	"github.com/merlindorin/sshark-api/internal/infra"
	sshkeys2 "github.com/merlindorin/sshark-api/internal/infra/sshkeys"
)

const (
	indexKey         = "sshkey"
	indexName        = "idx:sshkey"
	defaultSortField = "updated_at"

	// Stats counter keys.
	statsKeyTotalKeys   = "sshark:stats:total_keys"
	statsKeyUsernames   = "sshark:stats:usernames"
	statsKeyProviders   = "sshark:stats:providers"
	statsKeyInitialized = "sshark:stats:initialized"
)

type Repository struct {
	rdb *redis.Client

	indexKey                  string
	defaultSortIndexFieldName string
	defaultSortOrderAsc       bool
}

func NewRedisRepository(rdb *redis.Client) *Repository {
	return &Repository{
		rdb:                       rdb,
		indexKey:                  indexKey,
		defaultSortIndexFieldName: defaultSortField,
		defaultSortOrderAsc:       false,
	}
}

type QuerySearch struct {
	ID        *uuid.UUID
	Type      *string
	Source    *string
	Username  *string
	CreatedAt *time.Time
}

//nolint:nonamedreturns // existing code pattern
func (r Repository) Search(
	ctx context.Context,
	search string,
	limit, offset int,
	callback func(entity *sshkeys.Entity),
) (total int, err error) {
	searchOptions := &redis.FTSearchOptions{
		Limit:       limit,
		LimitOffset: offset,
	}

	raw, err := r.rdb.FTSearchWithArgs(ctx, fmt.Sprintf("idx:%s", r.indexKey), search, searchOptions).RawResult()
	if err != nil {
		return 0, fmt.Errorf("failed to search: %w", err)
	}

	items, total, err := infra.ParseSearchResult(raw)
	if err != nil {
		return 0, fmt.Errorf("can parse raw result: %w", err)
	}

	for _, item := range items {
		callback(&item.Entity)
	}

	return total, nil
}

func (r Repository) List(ctx context.Context, limit, offset *int, result *sshkeys.ListResult) error {
	searchOptions := &redis.FTSearchOptions{
		Limit:       *limit,
		LimitOffset: *offset,
		SortBy: []redis.FTSearchSortBy{
			{
				FieldName: r.defaultSortIndexFieldName,
				Asc:       r.defaultSortOrderAsc,
			},
		},
	}

	// As of today, it is necessary to use RawResult() for RESP3 compatibility
	raw, err := r.rdb.FTSearchWithArgs(ctx, indexName, "*", searchOptions).RawResult()
	if err != nil {
		return fmt.Errorf("cannot get raw result: %w", err)
	}

	keys, total, err := infra.ParseRawResult[sshkeys.Entity](raw)
	if err != nil {
		return fmt.Errorf("cannot parse raw result: %w", err)
	}

	*result = sshkeys.ListResult{
		Entities: keys,
		Total:    total,
	}

	return nil
}

func (r Repository) CreateFromAuthorizedKeys(
	ctx context.Context,
	authorizedKeys *sshkeys.AuthorizedKeys,
	provider string,
	entities *[]sshkeys.Entity,
) error {
	all, err := io.ReadAll(authorizedKeys.Keys)
	if err != nil {
		return fmt.Errorf("can read buffered keys: %w", err)
	}

	for len(all) > 0 {
		var key ssh.PublicKey
		var comment string
		var option []string
		var leftover []byte
		key, comment, option, leftover, err = ssh.ParseAuthorizedKey(all)
		if err != nil {
			return fmt.Errorf("failed to parse authorized key: %w", err)
		}

		if option == nil {
			option = []string{}
		}

		sshkey := sshkeys.Entity{
			Username: authorizedKeys.Username,
			Source:   authorizedKeys.Source,
			Provider: provider,
			Type:     key.Type(),
			Comment:  comment,
			Options:  option,
			Key:      key.Marshal(),
		}

		if createErr := r.Create(ctx, &sshkey); createErr != nil {
			return fmt.Errorf("failed to create key: %w", createErr)
		}

		if entities != nil {
			*entities = append(*entities, sshkey)
		}

		all = leftover
	}

	return nil
}

func (r Repository) Get(ctx context.Context, id uuid.UUID, key *sshkeys.Entity) error {
	val, err := r.rdb.JSONGet(ctx, fmt.Sprintf("%s:%s", r.indexKey, id)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return sshkeys.ErrSSHKeyNotFound
		}

		return fmt.Errorf("failed to find sshkey: %w", err)
	}

	err = json.Unmarshal([]byte(val), &key)
	if err != nil {
		return fmt.Errorf("failed to parse sshkey: %w", err)
	}

	return nil
}

func (r Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.rdb.Del(ctx, fmt.Sprintf("%s:%s", r.indexKey, id)).Result()
	if err != nil {
		return fmt.Errorf("failed to delete sshkey: %w", err)
	}

	if result == 0 {
		return sshkeys.ErrSSHKeyNotFound
	}

	// Update stats counter (decrement total keys)
	// Note: We keep username and provider sets as they may still have other keys
	_ = r.rdb.Decr(ctx, statsKeyTotalKeys).Err() // Ignore error to not fail deletion

	return nil
}

// DropIndex removes the index.
// Use this to force index recreation with a new schema.
func (r Repository) DropIndex(ctx context.Context) error {
	_, err := r.rdb.FTDropIndex(ctx, indexName).Result()
	return err
}

// ExplainQuery validates a query by running it with LIMIT 0 0.
func (r Repository) ValidateQuery(ctx context.Context, query string) (string, error) {
	_, err := r.rdb.Do(ctx, "FT.SEARCH", indexName, query, "LIMIT", "0", "0").Result()
	if err != nil {
		return "", fmt.Errorf("failed to validate query: %w", err)
	}
	return "OK", nil
}

// EnsureIndex creates an index if it doesn't exist.
// If forceReindex is true, the existing index will be dropped and recreated.
func (r Repository) EnsureIndex(ctx context.Context, forceReindex bool) error {
	if forceReindex {
		_ = r.DropIndex(ctx)
	}

	_, err := r.rdb.FTInfo(ctx, indexName).Result()
	if err == nil {
		return nil
	}

	_, err = r.rdb.FTCreate(ctx, indexName,
		&redis.FTCreateOptions{
			OnJSON: true,
			Prefix: []interface{}{fmt.Sprintf("%s:", r.indexKey)},
		},
		&redis.FieldSchema{FieldName: "$.id", As: "id", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.username", As: "username", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.source", As: "source", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.provider", As: "provider", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.type", As: "type", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.key", As: "key", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.comment", As: "comment", FieldType: redis.SearchFieldTypeTag},
	).Result()

	return err
}

func (r Repository) SSHKeyCount(ctx context.Context) (int, error) {
	keysResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "0",
		"REDUCE", "COUNT", "0", "AS", "total",
	).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get total keys: %w", err)
	}

	return parseAggregateTotal(keysResult), nil
}

// GetStats retrieves aggregated statistics about SSH keys using fast counter reads.
// This method reads from pre-computed counters maintained by Create/Delete operations.
func (r Repository) GetStats(ctx context.Context, result *stats.Stats) error {
	// Use pipeline for parallel reads
	pipe := r.rdb.Pipeline()
	totalKeysCmd := pipe.Get(ctx, statsKeyTotalKeys)
	usernamesCmd := pipe.SCard(ctx, statsKeyUsernames)
	providersCmd := pipe.SCard(ctx, statsKeyProviders)

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Parse total keys (default to 0 if not found)
	totalKeys, _ := totalKeysCmd.Int()
	result.TotalKeys = totalKeys

	// Parse unique usernames count
	result.TotalUsernames = int(usernamesCmd.Val())

	// Parse unique providers count
	result.TotalProviders = int(providersCmd.Val())

	return nil
}

// InitializeStatsCounters performs a one-time initialization of stats counters
// by scanning existing SSH keys. This is idempotent and safe to run multiple times.
func (r Repository) InitializeStatsCounters(ctx context.Context) error {
	// Check if already initialized
	exists, err := r.rdb.Exists(ctx, statsKeyInitialized).Result()
	if err != nil {
		return fmt.Errorf("failed to check initialization status: %w", err)
	}
	if exists > 0 {
		return nil // Already initialized, skip
	}

	// Use FT.AGGREGATE to get total count (faster than scanning)
	totalResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "0",
		"REDUCE", "COUNT", "0", "AS", "total",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to count total keys: %w", err)
	}

	totalKeys := parseAggregateTotal(totalResult)

	// Get distinct username values using FT.AGGREGATE
	usernamesRaw, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "1", "@username",
		"LIMIT", "0", "10000", // Limit to first 10k usernames
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get usernames: %w", err)
	}

	providersRaw, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "1", "@provider",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get providers: %w", err)
	}

	usernames := parseAggregateGroupedValues(usernamesRaw, "username")
	providers := parseAggregateGroupedValues(providersRaw, "provider")

	// Store counters atomically using pipeline
	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, statsKeyTotalKeys, totalKeys, 0)

	// Add all usernames to set
	if len(usernames) > 0 {
		usernameSlice := make([]interface{}, len(usernames))
		for i, username := range usernames {
			usernameSlice[i] = username
		}
		pipe.SAdd(ctx, statsKeyUsernames, usernameSlice...)
	}

	// Add all providers to set
	if len(providers) > 0 {
		providerSlice := make([]interface{}, len(providers))
		for i, provider := range providers {
			providerSlice[i] = provider
		}
		pipe.SAdd(ctx, statsKeyProviders, providerSlice...)
	}

	// Mark as initialized
	pipe.Set(ctx, statsKeyInitialized, "1", 0)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store stats counters: %w", err)
	}

	return nil
}

func (r Repository) Create(ctx context.Context, sshkey *sshkeys.Entity) error {
	// we do not need the monotonic component
	now := time.Now().Truncate(0)

	model := sshkeys2.SSHKey{
		ID:        uuid.New(),
		Username:  sshkey.Username,
		Source:    sshkey.Source,
		Provider:  sshkey.Provider,
		Type:      sshkey.Type,
		Comment:   sshkey.Comment,
		Options:   sshkey.Options,
		Key:       sshkey.Key,
		CreatedAt: now,
		UpdatedAt: now,
	}

	status := r.rdb.JSONSet(ctx, fmt.Sprintf("%s:%s", r.indexKey, model.ID), "$", model)
	if status.Err() != nil {
		return fmt.Errorf("failed to save ssh key: %w", status.Err())
	}

	// Update stats counters (non-blocking, best effort)
	pipe := r.rdb.Pipeline()
	pipe.Incr(ctx, statsKeyTotalKeys)
	pipe.SAdd(ctx, statsKeyUsernames, model.Username)
	pipe.SAdd(ctx, statsKeyProviders, model.Provider)
	_, _ = pipe.Exec(ctx) // Ignore errors to not fail key creation

	*sshkey = model.ToEntity()

	return nil
}

// parseAggregateTotal extracts the "total" field from FT.AGGREGATE result with GROUPBY 0.
func parseAggregateTotal(result interface{}) int {
	m, ok := result.([]interface{})
	if !ok || len(m) != 2 {
		return 0
	}

	results, ok := m[1].([]interface{})
	if !ok || len(results) != 2 {
		return 0
	}

	total, ok := results[1].(float64)
	if !ok {
		return 0
	}

	return int(total)
}

// parseAggregateGroupedValues extracts field values from FT.AGGREGATE GROUPBY results.
func parseAggregateGroupedValues(result interface{}, fieldName string) []string {
	m, ok := result.([]interface{})
	if !ok || len(m) < 2 {
		return nil
	}

	totalCount, ok := m[0].(int64)
	if !ok {
		return nil
	}

	values := make([]string, 0, totalCount)
	results, ok := m[1].([]interface{})
	if !ok {
		return nil
	}

	// Each result is a map with the grouped field
	for _, item := range results {
		fields, fieldsOk := item.([]interface{})
		if !fieldsOk || len(fields) < 2 {
			continue
		}

		// Find the field value (format: [field_name, value, ...])
		for i := 0; i < len(fields)-1; i += 2 {
			if key, keyOk := fields[i].(string); keyOk && key == fieldName {
				if value, valueOk := fields[i+1].(string); valueOk {
					values = append(values, value)
				}
			}
		}
	}

	return values
}
