package redis_test

import (
	"bytes"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/merlindorin/sshark-api/internal/domain/github"
	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
	sshkeysrepository "github.com/merlindorin/sshark-api/internal/infra/sshkeys/redis"
)

func TestRepository_Create(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{UnstableResp3: true})
	r := sshkeysrepository.NewRedisRepository(rdb)

	t.Run("schema", func(t *testing.T) {
		_ = r.DropIndex(t.Context())

		err := r.EnsureIndex(t.Context())
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
