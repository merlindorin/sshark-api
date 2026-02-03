package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/merlindorin/sshark-api/internal/domain/gitlab"
	"github.com/merlindorin/sshark-api/internal/infra"
	"github.com/merlindorin/sshark-api/internal/redisquery"
)

const (
	// Documents are stored as: gitlabusers:{username}.
	indexKey         = "gitlabusers"
	indexName        = "idx:gitlabusers"
	defaultSortField = "updated_at"
)

// Repository provides Redis-based persistence for GitLab users using RedisJSON and RediSearch.
// It stores users as JSON documents and provides full-text search capabilities.
type Repository struct {
	rdb *redis.Client

	indexKey                  string
	defaultSortIndexFieldName string
	defaultSortOrderAsc       bool
}

// NewRepository creates a new Repository with the given Redis client.
func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{
		rdb:                       rdb,
		indexKey:                  indexKey,
		defaultSortIndexFieldName: defaultSortField,
	}
}

// Exist checks if a user with the given username exists in the repository.
func (r Repository) Exist(ctx context.Context, username gitlab.Username) (bool, error) {
	builder := redisquery.
		NewBuilder().
		Field("username").
		Tag(string(username))

	res := gitlab.ListResult{}

	if err := r.Query(ctx, 1, 0, builder, &res); err != nil {
		return false, fmt.Errorf("failed to execute query: %w", err)
	}

	return res.Count > 0, nil
}

// List returns all users with pagination support.
func (r Repository) List(ctx context.Context, limit, offset int, result *gitlab.ListResult) error {
	return r.Query(ctx, limit, offset, redisquery.NewBuilder(), result)
}

// Query executes a RediSearch query and returns matching users.
// Use redisquery.Builder to construct the query (e.g., Field("username").Tag("merlin")).
func (r Repository) Query(
	ctx context.Context,
	limit, offset int,
	query *redisquery.Builder,
	result *gitlab.ListResult,
) error {
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

	raw, err := r.rdb.FTSearchWithArgs(ctx, infra.FullIndexKey(indexKey), query.Build(), searchOptions).RawResult()
	if err != nil {
		return fmt.Errorf("cannot get raw result: %w", err)
	}

	users, total, err := infra.ParseRawResult[Model](raw)
	if err != nil {
		return fmt.Errorf("can parse raw result: %w", err)
	}

	*result = gitlab.ListResult{
		Entities: []gitlab.User{},
		Limit:    limit,
		Offset:   offset,
		Count:    len(users),
		Total:    total,
	}

	for _, user := range users {
		result.Entities = append(result.Entities, user.GetGitlabUser())
	}

	return nil
}

// Create creates a new user with the given username.
// Returns gitlab.ErrUserAlreadyExist if the user already exists.
func (r Repository) Create(ctx context.Context, username gitlab.Username, user *gitlab.User) error {
	userModel := NewGitlabUserModel(username)

	exist, err := r.Exist(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	if exist {
		return gitlab.ErrUserAlreadyExist
	}

	status := r.rdb.JSONSet(ctx, r.userKey(username), "$", userModel)
	if status.Err() != nil {
		return fmt.Errorf("failed to save userModel: %w", status.Err())
	}

	userModel.ToGitlabUser(user)

	return nil
}

// Get retrieves a user by username.
// Returns gitlab.ErrUserNotFound if the user does not exist.
func (r Repository) Get(ctx context.Context, username gitlab.Username, u *gitlab.User) error {
	val, err := r.rdb.JSONGet(ctx, r.userKey(username)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return gitlab.ErrUserNotFound
		}

		return fmt.Errorf("failed to find user: %w", err)
	}

	userModel := Model{}
	err = json.Unmarshal([]byte(val), &userModel)
	if err != nil {
		return fmt.Errorf("failed to parse user: %w", err)
	}

	userModel.ToGitlabUser(u)

	return nil
}

// Delete removes a user by username.
// Returns gitlab.ErrUserNotFound if the user does not exist.
func (r Repository) Delete(ctx context.Context, username gitlab.Username) error {
	result, err := r.rdb.Del(ctx, r.userKey(username)).Result()
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result == 0 {
		return gitlab.ErrUserNotFound
	}

	return nil
}

// DropIndex removes the RediSearch index.
// Use this to force index recreation with a new schema.
func (r Repository) DropIndex(ctx context.Context) error {
	_, err := r.rdb.FTDropIndex(ctx, indexName).Result()
	return err
}

// EnsureIndex creates the RediSearch index if it doesn't exist.
// If forceReindex is true, the existing index will be dropped and recreated.
// The index enables searching users by username (TAG field).
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
		&redis.FieldSchema{FieldName: "$.username", As: "username", FieldType: redis.SearchFieldTypeTag},
		&redis.FieldSchema{FieldName: "$.created_at", As: "created_at", FieldType: redis.SearchFieldTypeText, Sortable: true},
		&redis.FieldSchema{FieldName: "$.updated_at", As: "updated_at", FieldType: redis.SearchFieldTypeText, Sortable: true},
		&redis.FieldSchema{
			FieldName: "$.last_scraped_at",
			As:        "last_scraped_at",
			FieldType: redis.SearchFieldTypeNumeric,
			Sortable:  true,
		},
		&redis.FieldSchema{
			FieldName: "$.scraped_successfully",
			As:        "scraped_successfully",
			FieldType: redis.SearchFieldTypeTag,
		},
	).Result()
	if err != nil {
		// If index already exists (race condition or timing issue), treat as success
		if err.Error() == "Index already exists" {
			return nil
		}
		return err
	}

	return nil
}

// UpdateScrapeMetadata updates the scrape timestamp and success status for a user.
func (r Repository) UpdateScrapeMetadata(ctx context.Context, username gitlab.Username, success bool) error {
	now := time.Now()

	_, err := r.rdb.JSONSet(ctx, r.userKey(username), "$.last_scraped_at", now).Result()
	if err != nil {
		return fmt.Errorf("failed to update last_scraped_at: %w", err)
	}

	_, err = r.rdb.JSONSet(ctx, r.userKey(username), "$.scraped_successfully", success).Result()
	if err != nil {
		return fmt.Errorf("failed to update scraped_successfully: %w", err)
	}

	_, err = r.rdb.JSONSet(ctx, r.userKey(username), "$.updated_at", now).Result()
	if err != nil {
		return fmt.Errorf("failed to update updated_at: %w", err)
	}

	return nil
}

// userKey returns the Redis key for a user document.
func (r Repository) userKey(u gitlab.Username) string {
	return fmt.Sprintf("%s:%s", r.indexKey, u.String())
}
