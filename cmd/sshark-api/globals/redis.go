package globals

import (
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Name     string `env:"REDIS_NAME" default:"sshark"`
	Host     string `env:"REDIS_HOST" help:"Redis host" default:"localhost"`
	Port     int    `env:"REDIS_PORT" help:"Redis port" default:"6379"`
	Username string `env:"REDIS_USERNAME" help:"Redis username"`
	Password string `env:"REDIS_PASSWORD" help:"Redis password"`
	DB       int    `env:"REDIS_DB" help:"Redis db" default:"0"`
}

func (s *Redis) Client() *redis.Client {
	return redis.NewClient(&redis.Options{
		ClientName:    s.Name,
		Addr:          s.Addr(),
		Username:      s.Username,
		Password:      s.Password,
		DB:            s.DB,
		UnstableResp3: true,
		Protocol:      3,
	})
}

func (s *Redis) Addr() string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(s.Host), s.Port)
}
