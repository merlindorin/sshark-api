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

	"github.com/merlindorin/sshark-api/internal/domain/github"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/domain/stats"
	"github.com/merlindorin/sshark-api/internal/infra"
	sshkeys2 "github.com/merlindorin/sshark-api/internal/infra/sshkeys"
)

const (
	indexKey         = "sshkey"
	indexName        = "idx:sshkey"
	defaultSortField = "updated_at"
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
	limit, offset *int,
	callback func(entity *sshkeys.Entity),
) (total int, err error) {
	searchOptions := &redis.FTSearchOptions{
		Limit:       *limit,
		LimitOffset: *offset,
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
	authorizedKeys *github.AuthorizedKeys,
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
			Username: authorizedKeys.Username.String(),
			Source:   authorizedKeys.Source,
			Provider: "github",
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

// GetStats retrieves aggregated statistics about SSH keys.
func (r Repository) GetStats(ctx context.Context, result *stats.Stats) error {
	keysResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*", "GROUPBY", "1", "@key", "GROUPBY", "0", "REDUCE", "COUNT", "0", "as", "count",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get total keys: %w", err)
	}

	result.TotalKeys = parseAggregateTotal(keysResult)

	usernameResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*", "GROUPBY", "1", "@username", "GROUPBY", "0", "REDUCE", "COUNT", "0", "as", "count",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get unique usernames: %w", err)
	}

	result.TotalUsernames = parseAggregateTotal(usernameResult)

	providerResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*", "GROUPBY", "1", "@provider", "GROUPBY", "0", "REDUCE", "COUNT", "0", "as", "count",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get unique providers: %w", err)
	}

	result.TotalProviders = parseAggregateTotal(providerResult)

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
