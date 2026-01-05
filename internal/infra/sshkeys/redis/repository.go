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

func (r Repository) Search(ctx context.Context, search string, limit, offset int, result *sshkeys.SearchResult) error {
	searchOptions := &redis.FTSearchOptions{
		Limit:       limit,
		LimitOffset: offset,
		WithScores:  true,
	}

	raw, err := r.rdb.FTSearchWithArgs(ctx, fmt.Sprintf("idx:%s", r.indexKey), search, searchOptions).RawResult()
	if err != nil {
		return fmt.Errorf("failed to search: %w", err)
	}

	items, total, err := infra.ParseSearchResult(raw)
	if err != nil {
		return fmt.Errorf("can parse raw result: %w", err)
	}

	*result = sshkeys.SearchResult{
		Entities: items.ToSearchEntities(),
		Total:    total,
	}

	return nil
}

func (r Repository) List(ctx context.Context, limit, offset int, result *sshkeys.ListResult) error {
	searchOptions := &redis.FTSearchOptions{
		Limit:       limit,
		LimitOffset: offset,
		SortBy: []redis.FTSearchSortBy{
			{
				FieldName: r.defaultSortIndexFieldName,
				Asc:       r.defaultSortOrderAsc,
			},
		},
	}

	// As of today, it is necessary to use RawResult() for RESP3 compatibility
	raw, err := r.rdb.FTSearchWithArgs(ctx, "idx:sshkeys", "*", searchOptions).RawResult()
	if err != nil {
		return fmt.Errorf("can get raw result: %w", err)
	}

	keys, total, err := infra.ParseRawResult[sshkeys.Entity](raw)
	if err != nil {
		return fmt.Errorf("can parse raw result: %w", err)
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

		sshkey := SSHKey{
			ID:        uuid.New(),
			Username:  authorizedKeys.Username.String(),
			Source:    authorizedKeys.Source,
			Provider:  "github",
			Type:      key.Type(),
			Comment:   comment,
			Options:   option,
			Key:       key.Marshal(),
			Raw:       all,
			Rest:      leftover,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		status := r.rdb.JSONSet(ctx, fmt.Sprintf("%s:%s", r.indexKey, sshkey.ID), "$", sshkey)
		if status.Err() != nil {
			return fmt.Errorf("failed to save ssh key: %w", status.Err())
		}

		if entities != nil {
			*entities = append(*entities, sshkey.GetEntity())
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

// DropIndex removes the RediSearch index.
// Use this to force index recreation with a new schema.
func (r Repository) DropIndex(ctx context.Context) error {
	_, err := r.rdb.FTDropIndex(ctx, indexName).Result()
	return err
}

// ExplainQuery validates a query and returns its execution plan without executing it.
// Note: FT.EXPLAIN is not supported by Dragonfly, so this is a no-op.
func (r Repository) ExplainQuery(_ context.Context, _ string) (string, error) {
	return "", nil
}

// EnsureIndex creates the RediSearch index if it doesn't exist.
// If forceReindex is true, the existing index will be dropped and recreated.
// Index fields:
//   - id           (TAG)
//   - username     (TEXT, weight 3.0) - for full-text search
//   - username     (TAG) as username_exact - for exact match
//   - source       (TAG) - full URL of the key source
//   - provider     (TAG) - github, gitlab, etc.
//   - type         (TAG) - ssh-rsa, ssh-ed25519, etc.
//   - key          (TAG) - for reverse lookup by key content
//   - comment      (TEXT, weight 1.0)
//   - created_at   (TEXT, sortable)
//   - updated_at   (TEXT, sortable)
func (r Repository) EnsureIndex(ctx context.Context, forceReindex bool) error {
	if forceReindex {
		_ = r.DropIndex(ctx) // ignore error if index doesn't exist
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
		&redis.FieldSchema{FieldName: "$.username", As: "username", FieldType: redis.SearchFieldTypeText, Weight: 3.0},
		&redis.FieldSchema{FieldName: "$.username", As: "username_exact", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.source", As: "source", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.provider", As: "provider", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.type", As: "type", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.key", As: "key", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.comment", As: "comment", FieldType: redis.SearchFieldTypeText, Weight: 1.0},
		&redis.FieldSchema{FieldName: "$.created_at", As: "created_at", FieldType: redis.SearchFieldTypeText, Sortable: true},
		&redis.FieldSchema{FieldName: "$.updated_at", As: "updated_at", FieldType: redis.SearchFieldTypeText, Sortable: true},
	).Result()

	return err
}

// GetStats retrieves aggregated statistics about SSH keys.
func (r Repository) GetStats(ctx context.Context, result *stats.Stats) error {
	keysResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "0",
		"REDUCE", "COUNT", "0", "AS", "total",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get total keys: %w", err)
	}

	result.TotalKeys = parseAggregateTotal(keysResult)

	usernameResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "1", "@username_exact",
		"REDUCE", "COUNT", "0",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get unique usernames: %w", err)
	}

	result.TotalUsernames = parseAggregateCount(usernameResult)

	providerResult, err := r.rdb.Do(ctx,
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", "1", "@provider",
		"REDUCE", "COUNT", "0",
	).Result()
	if err != nil {
		return fmt.Errorf("failed to get unique providers: %w", err)
	}

	result.TotalProviders = parseAggregateCount(providerResult)

	return nil
}

// parseAggregateCount extracts the count of groups from FT.AGGREGATE RESP3 result.
func parseAggregateCount(result interface{}) int {
	m, ok := result.(map[interface{}]interface{})
	if !ok {
		return 0
	}
	results, ok := m["results"].([]interface{})
	if !ok {
		return 0
	}
	return len(results)
}

// parseAggregateTotal extracts the "total" field from FT.AGGREGATE RESP3 result with GROUPBY 0.
func parseAggregateTotal(result interface{}) int {
	m, ok := result.(map[interface{}]interface{})
	if !ok {
		return 0
	}
	results, ok := m["results"].([]interface{})
	if !ok || len(results) == 0 {
		return 0
	}
	row, ok := results[0].(map[interface{}]interface{})
	if !ok {
		return 0
	}
	extra, ok := row["extra_attributes"].(map[interface{}]interface{})
	if !ok {
		return 0
	}
	total, isStr := extra["total"].(string)
	if !isStr {
		return 0
	}
	var count int
	_, _ = fmt.Sscanf(total, "%d", &count)
	return count
}
