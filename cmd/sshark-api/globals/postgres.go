package globals

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Host            string        `env:"POSTGRES_HOST" help:"PostgreSQL host" default:"localhost"`
	Port            int           `env:"POSTGRES_PORT" help:"PostgreSQL port" default:"5432"`
	User            string        `env:"POSTGRES_USER" help:"PostgreSQL user" default:"postgres"`
	Password        string        `env:"POSTGRES_PASSWORD" help:"PostgreSQL password"`
	Database        string        `env:"POSTGRES_DATABASE" help:"PostgreSQL database" default:"sshark"`
	SSLMode         string        `env:"POSTGRES_SSL_MODE" help:"PostgreSQL SSL mode" default:"disable"`
	MaxConns        int32         `env:"POSTGRES_MAX_CONNS" help:"Max number of connections" default:"10"`
	MinConns        int32         `env:"POSTGRES_MIN_CONNS" help:"Min number of connections" default:"2"`
	MaxConnLifetime time.Duration `env:"POSTGRES_MAX_CONN_LIFETIME" help:"Max lifetime of a connection" default:"1h"`
	MaxConnIdleTime time.Duration `env:"POSTGRES_MAX_CONN_IDLE_TIME" help:"Max idle time of a connection" default:"30m"`
}

func (p *Postgres) DSN() string {
	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Database, p.SSLMode)
	if p.Password != "" {
		dsn += fmt.Sprintf(" password=%s", p.Password)
	}
	return dsn
}

func (p *Postgres) Pool(ctx context.Context) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(p.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	config.MaxConns = p.MaxConns
	config.MinConns = p.MinConns
	config.MaxConnLifetime = p.MaxConnLifetime
	config.MaxConnIdleTime = p.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	return pool, nil
}
