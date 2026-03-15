package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // file source for migrate
	"github.com/merlindorin/go-shared/pkg/cmd"
	"go.uber.org/zap"

	"github.com/merlindorin/sshark-api/cmd/sshark-api/globals"
)

type Migrate struct {
	MigrationsPath string `help:"Path to migrations directory" default:"db/migrations"`
}

func (m *Migrate) Run(_ context.Context, common *cmd.Commons, postgres *globals.Postgres) error {
	logger := common.MustLogger().Named("migrate")

	logger.Info(
		"Running migrations...",
		zap.String("name", fmt.Sprintf("%s-migration", common.Version.Name())),
		zap.String("version", common.Version.Version()),
		zap.String("postgres.host", postgres.Host),
		zap.String("postgres.database", postgres.Database),
		zap.String("migrations_path", m.MigrationsPath),
	)

	hostPort := net.JoinHostPort(postgres.Host, fmt.Sprintf("%d", postgres.Port))
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(postgres.User, postgres.Password),
		Host:     hostPort,
		Path:     postgres.Database,
		RawQuery: "sslmode=" + postgres.SSLMode,
	}).String()

	migrator, err := migrate.New(
		fmt.Sprintf("file://%s", m.MigrationsPath),
		dsn,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	defer func(migrator *migrate.Migrate) {
		errMigrator, _ := migrator.Close()
		if errMigrator != nil {
			err = errors.Join(err, fmt.Errorf("failed to close migrator: %w", errMigrator))
		}
	}(migrator)

	upErr := migrator.Up()
	if upErr != nil {
		if errors.Is(upErr, migrate.ErrNoChange) {
			logger.Info("No migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", upErr)
	}

	version, dirty, err := migrator.Version()
	if err != nil {
		logger.Warn("Failed to get migration version", zap.Error(err))
	} else {
		logger.Info("Migrations applied successfully",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty),
		)
	}

	return nil
}
