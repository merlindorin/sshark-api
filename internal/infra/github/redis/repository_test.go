package redis_test

import (
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/merlindorin/sshark-api/internal/domain/github"
	githubrepository "github.com/merlindorin/sshark-api/internal/infra/github/redis"
	"github.com/merlindorin/sshark-api/internal/redisquery"
)

func TestRepository_Create(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	r := githubrepository.NewRepository(rdb)

	t.Run("schema", func(t *testing.T) {
		_ = r.DropIndex(t.Context())

		err := r.EnsureIndex(t.Context(), false)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("simple", func(t *testing.T) {
		u := github.User{}

		if err := r.Create(t.Context(), "merlin", &u); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("simple get", func(t *testing.T) {
		u := github.User{}
		err := r.Get(t.Context(), "merlin", &u)

		if err != nil {
			t.Fatal(err)
		}

		if u.Username != github.Username("merlin") {
			t.Fatal("expected merlin")
		}
	})

	t.Run("exist", func(t *testing.T) {
		exist, err := r.Exist(t.Context(), "merlin")
		if err != nil {
			t.Fatal(err)
		}

		if !exist {
			t.Fatal("expected user to exist")
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := r.Delete(t.Context(), "merlin")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("list all", func(t *testing.T) {
		res := github.ListResult{}

		err := r.List(t.Context(), 10, 0, &res)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("query all", func(t *testing.T) {
		res := github.ListResult{}

		err := r.Query(t.Context(), 10, 0, redisquery.NewBuilder(), &res)
		if err != nil {
			t.Fatal(err)
		}
	})
}
