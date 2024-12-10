package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/log"
)

func NewDatabaseClient(
	c context.Context,
	dbConfig config.Database,
) *pgxpool.Pool {
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "main NewDatabaseClient").
		Str(log.KeyProcess, "connecting to database").
		Logger()

	logger.Info().Msg("connecting to database")

	logger = logger.With().Str(log.KeyProcess, "initializing postgresUrl").Logger()
	logger.Info().Msg("initializing postgresUrl")
	postgresUrl := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		int(dbConfig.Port),
		dbConfig.DbName,
	)
	logger = logger.With().Str(log.KeyDbURL, postgresUrl).Logger()
	logger.Info().Msg("initialized postgresUrl")

	logger = logger.With().Str(log.KeyProcess, "initializing pgx config").Logger()
	logger.Info().Msgf("initializing pgx config")
	pgxConfig, err := pgxpool.ParseConfig(postgresUrl)
	if err != nil {
		err = fmt.Errorf("failed creating pgx config with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msgf("initialized pgx config")

	logger = logger.With().Str(log.KeyProcess, "attaching otel tracer to pgx").Logger()
	logger.Info().Msgf("attaching otel tracer to pgx")
	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	logger.Info().Msgf("attached otel tracer to pgx")

	logger = logger.With().Str(log.KeyProcess, "creating connection pool").Logger()
	logger.Info().Msg("creating connection pool")
	pool, err := pgxpool.NewWithConfig(c, pgxConfig)
	if err != nil {
		err = fmt.Errorf("failed creating connection pool with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("created connection pool")

	logger = logger.With().Str(log.KeyProcess, "ping db").Logger()
	logger.Info().Msg("ping db")
	err = pool.Ping(c)
	if err != nil {
		err = fmt.Errorf("failed ping db with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("successed ping db")

	logger = logger.With().Str(log.KeyProcess, "creating sql.DB instance").Logger()
	logger.Info().Msg("creating sql.DB instance")
	db := stdlib.OpenDBFromPool(pool)
	logger.Info().Msg("created sql.DB instance")

	logger = logger.With().Str(log.KeyProcess, "initializing db driver").Logger()
	logger.Info().Msg("initializing db driver")
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		err = fmt.Errorf("failed creating postgres driver to do migration with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("initialized db driver")

	logger = logger.With().Str(log.KeyProcess, "initializing migration").Logger()
	logger.Info().Msg("initializing migration")
	migration, err := migrate.NewWithDatabaseInstance(dbConfig.MigrationPath, postgresUrl, driver)
	if err != nil {
		err = fmt.Errorf("failed migration postgres with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("initialized migration")

	logger = logger.With().Str(log.KeyProcess, "migration down").Logger()
	logger.Info().Msg("migration down")
	err = migration.Down()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		err = fmt.Errorf("failed migration down with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("successed migration down")

	logger = logger.With().Str(log.KeyProcess, "migration up").Logger()
	logger.Info().Msg("migration up")
	err = migration.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		err = fmt.Errorf("failed migration up with error=%w", err)
		logger.Fatal().Err(err).Msg(err.Error())
	}
	logger.Info().Msg("successed migration up")

	db.SetConnMaxLifetime(time.Minute * 15)
	db.SetConnMaxIdleTime(time.Minute * 5)
	db.SetMaxOpenConns(int(dbConfig.MaxConnections))
	db.SetMaxIdleConns(int(dbConfig.MinConnections))

	logger.Info().
		Str(log.KeyProcess, "connecting to database").
		Msg("successed connecting to database")

	return pool
}
