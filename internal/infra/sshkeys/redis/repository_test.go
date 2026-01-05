package redis_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/merlindorin/sshark-api/internal/domain/github"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	"github.com/merlindorin/sshark-api/internal/domain/stats"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
)

func TestRepository_CreateFromAuthorizedKeys(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	r := sshkeysrepository.NewRedisRepository(rdb)

	t.Run("schema", func(t *testing.T) {
		_ = r.DropIndex(t.Context())

		err := r.EnsureIndex(t.Context(), false)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("create", func(t *testing.T) {
		entities := []sshkeys.Entity{}

		authorizedKeys := &github.AuthorizedKeys{
			Username: "merlin",
			Keys:     bytes.NewBufferString("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPYhndfr5o7SYHYpsoUtUvDGpiHqNy57Z2MqZqqI1iWZ"),
			Source:   "https://github.com/merlindorin.keys",
		}
		err := r.CreateFromAuthorizedKeys(
			t.Context(),
			authorizedKeys,
			&entities,
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(entities) != 1 {
			t.Fatalf("got %d entities, expected 1", len(entities))
		}
	})
}

func TestRepository_SSHKeyCount(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	r := sshkeysrepository.NewRedisRepository(rdb)

	t.Cleanup(func() {
		_ = r.DropIndex(t.Context())
	})

	// r.CreateFromAuthorizedKeys()

	got, err := r.SSHKeyCount(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 2, got)
}

func TestRepository_Create(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	r := sshkeysrepository.NewRedisRepository(rdb)
	assert.NoError(t, r.EnsureIndex(t.Context(), true))

	defer t.Cleanup(func() {
		rdb.FlushAll(context.Background())
	})

	type args struct {
		sshkey sshkeys.Entity
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "common",
			args: args{
				sshkey: sshkeys.Entity{
					Username: "merlin",
					Source:   "https://github.com/merlindorin.keys",
					Type:     "ssh-ed25519",
					Comment:  "some comment",
					Options:  []string{"option1", "option2"},
					Key:      []byte("raw-key-..."),
					Provider: "github",
				},
			},
		},
		{
			name: "uuid and dates are overwritten",
			args: args{
				sshkey: sshkeys.Entity{
					ID:        uuid.New(),
					Username:  "merlin",
					Source:    "https://github.com/merlindorin.keys",
					Type:      "ssh-ed25519",
					Comment:   "some comment",
					Options:   []string{"option1", "option2"},
					Key:       []byte("raw-key-..."),
					Provider:  "github",
					UpdatedAt: time.Now(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, r.Create(t.Context(), &tt.args.sshkey))

			res := sshkeys.ListResult{}
			assert.NoError(t, r.List(t.Context(), 10, 0, &res))

			var got sshkeys.Entity
			assert.NoError(t, r.Get(t.Context(), tt.args.sshkey.ID, &got))
			assert.Equal(t, got, tt.args.sshkey)
		})
	}
}

func TestRepository_GetStats(t *testing.T) {
	type args struct {
		result *stats.Stats
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "common",
			args: args{
				result: &stats.Stats{
					TotalKeys:      3,
					TotalUsernames: 2,
					TotalProviders: 2,
				},
			},
		},
	}
	for _, tt := range tests {
		rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
		r := sshkeysrepository.NewRedisRepository(rdb)
		assert.NoError(t, r.EnsureIndex(t.Context(), true))

		defer t.Cleanup(func() {
			rdb.FlushAll(context.Background())
		})

		assert.NoError(t, r.Create(t.Context(), &sshkeys.Entity{
			Username: "merlindorin",
			Provider: "github",
		}))
		assert.NoError(t, r.Create(t.Context(), &sshkeys.Entity{
			Username: "merlindorin",
			Provider: "another",
		}))
		assert.NoError(t, r.Create(t.Context(), &sshkeys.Entity{
			Username: "another",
			Provider: "github",
		}))

		t.Run(tt.name, func(t *testing.T) {
			var res stats.Stats
			assert.NoError(t, r.GetStats(t.Context(), &res))

			assert.Equal(t, res.TotalKeys, tt.args.result.TotalKeys)
		})
	}
}
