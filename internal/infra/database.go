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
	logger := zerolog.Ctx(c).With().
		Str(log.KeyTag, "main NewDatabaseClient").
		Logger()

	postgresUrl := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		int(dbConfig.Port),
		dbConfig.DbName,
	)

	logger.Info().
		Str(log.KeyProcess, "initializing pgx config").
		Msgf("initializing connection to database")
	pgxConfig, err := pgxpool.ParseConfig(postgresUrl)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "NewPostgreSQLClient").
			Msgf("failed creating pgx config with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "initializing pgx config").
		Msgf("initialized connection to database")

	logger.Info().
		Str(log.KeyProcess, "attach tracer to pgx").
		Msgf("attaching tracer to pgx")
	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	logger.Info().
		Str(log.KeyProcess, "attach tracer to pgx").
		Msgf("attached tracer to pgx")

	logger.Info().
		Str(log.KeyProcess, "creating connection pool").
		Msg("creating connection pool to postgres")
	pool, err := pgxpool.NewWithConfig(c, pgxConfig)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "creating connection pool").
			Msgf("failed creating connection pool to postgres with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "creating connection pool").
		Msg("created connection pool to postgres")

	logger.Info().
		Str(log.KeyProcess, "ping db connection").
		Msg("pinging db connection")
	err = pool.Ping(c)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "ping db connection").
			Msgf("failed ping connction to db with error=%s", err.Error())
	}
	logger.Info().
		Str(log.KeyProcess, "ping db connection").
		Msg("successed ping db connection")

	db := stdlib.OpenDBFromPool(pool)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "NewPostgreSQLClient").
			Msgf("failed creating postgres driver to do migration with error=%s", err.Error())
	}

	logger.Info().
		Str(log.KeyProcess, "NewPostgreSQLClient").
		Msgf("failed migration postgres with error=%s", err.Error())
	migration, err := migrate.NewWithDatabaseInstance(dbConfig.MigrationPath, postgresUrl, driver)
	if err != nil {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "NewPostgreSQLClient").
			Msgf("failed migration postgres with error=%s", err.Error())
	}

	err = migration.Down()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "NewPostgreSQLClient").
			Msgf("failed migration down postgres with error=%s", err.Error())
	}

	err = migration.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Fatal().
			Err(err).
			Str(log.KeyProcess, "NewPostgreSQLClient").
			Msgf("failed migration up postgres with error=%s", err.Error())
	}

	db.SetConnMaxLifetime(time.Minute * 15)
	db.SetConnMaxIdleTime(time.Minute * 5)
	db.SetMaxOpenConns(int(dbConfig.MaxConnections))
	db.SetMaxIdleConns(int(dbConfig.MinConnections))

	logger.Info().
		Str(log.KeyProcess, "NewPostgreSQLClient").
		Msgf("successed connecting to database")

	return pool
}
