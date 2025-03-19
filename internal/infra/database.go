package infra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/otel"
)

var (
	dbOnce sync.Once
	pool   *pgxpool.Pool
)

func NewDatabaseClient(
	c context.Context,
	config config.Database,
) *pgxpool.Pool {
	dbOnce.Do(func() {
		c, span := otel.Tracer.Start(c, "main NewDatabaseClient")
		defer span.End()

		logger := zerolog.Ctx(c).
			With().
			Ctx(c).
			Str(constants.KEY_TAG, "main NewDatabaseClient").
			Str(constants.KEY_PROCESS, "connecting to database").
			Logger()

		logger.Trace().Msg("connecting to database")

		logger = logger.With().Str(constants.KEY_PROCESS, "initializing postgresUrl").Logger()
		logger.Trace().Msg("initializing postgresUrl")
		span.AddEvent("initializing postgresUrl")
		postgresUrl := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=disable",
			config.Username,
			config.Password,
			config.Host,
			int(config.Port),
			config.Name,
		)
		span.AddEvent("initialized postgresUrl")
		logger = logger.With().Str(constants.KEY_DB_URL, postgresUrl).Logger()
		logger.Info().Msg("initialized postgresUrl")

		logger = logger.With().Str(constants.KEY_PROCESS, "initializing pgx config").Logger()
		logger.Trace().Msg("initializing pgx config")
		span.AddEvent("initializing pgx config")
		pgxConfig, err := pgxpool.ParseConfig(postgresUrl)
		if err != nil {
			err = fmt.Errorf("failed creating pgx config with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("initialized pgx config")
		logger.Info().Msg("initialized pgx config")

		pgxConfig.AfterConnect = func(ctx context.Context, pgxConn *pgx.Conn) error {
			pgxUUID.Register(pgxConn.TypeMap())
			return nil
		}

		logger = logger.With().Str(constants.KEY_PROCESS, "attaching otel tracer to pgx").Logger()
		logger.Trace().Msg("attaching otel tracer to pgx")
		span.AddEvent("attaching otel tracer to pgx")
		pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer(
			otelpgx.WithAttributes(semconv.DBSystemPostgreSQL),
		)
		span.AddEvent("attached otel tracer to pgx")
		logger.Info().Msgf("attached otel tracer to pgx")

		logger = logger.With().Str(constants.KEY_PROCESS, "creating connection pool").Logger()
		logger.Trace().Msg("creating connection pool")
		span.AddEvent("creating connection pool")
		pool, err = pgxpool.NewWithConfig(c, pgxConfig)
		if err != nil {
			err = fmt.Errorf("failed creating connection pool with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("created connection pool")
		logger.Info().Msg("created connection pool")

		logger = logger.With().Str(constants.KEY_PROCESS, "ping db").Logger()
		logger.Trace().Msg("ping db")
		span.AddEvent("ping db")
		err = pool.Ping(c)
		if err != nil {
			err = fmt.Errorf("failed ping db with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("successed ping db")
		logger.Info().Msg("successed ping db")

		logger = logger.With().Str(constants.KEY_PROCESS, "creating sql.DB instance").Logger()
		logger.Trace().Msg("creating sql.DB instance")
		span.AddEvent("creating sql.DB instance")
		db := stdlib.OpenDBFromPool(pool)
		span.AddEvent("created sql.DB instance")
		logger.Info().Msg("created sql.DB instance")

		logger = logger.With().Str(constants.KEY_PROCESS, "initializing db driver").Logger()
		logger.Trace().Msg("initializing db driver")
		span.AddEvent("initializing db driver")
		driver, err := postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			err = fmt.Errorf("failed creating postgres driver to do migration with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("initialized db driver")
		logger.Info().Msg("initialized db driver")

		logger = logger.With().Str(constants.KEY_PROCESS, "initializing migration").Logger()
		logger.Trace().Msg("initializing migration")
		span.AddEvent("initializing migration")
		migration, err := migrate.NewWithDatabaseInstance(config.MigrationPath, postgresUrl, driver)
		if err != nil {
			err = fmt.Errorf("failed migration postgres with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("initialized migration")
		logger.Info().Msg("initialized migration")

		logger = logger.With().Str(constants.KEY_PROCESS, "migration down").Logger()
		logger.Trace().Msg("migration down")
		span.AddEvent("migration down")
		err = migration.Down()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			err = fmt.Errorf("failed migration down with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("successed migration down")
		logger.Info().Msg("successed migration down")

		logger = logger.With().Str(constants.KEY_PROCESS, "migration up").Logger()
		logger.Trace().Msg("migration up")
		span.AddEvent("migration up")
		err = migration.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			err = fmt.Errorf("failed migration up with error=%w", err)
			logger.Fatal().Err(err).Msg(err.Error())
		}
		span.AddEvent("successed migration up")
		logger.Info().Msg("successed migration up")

		db.SetConnMaxLifetime(time.Minute * 15)
		db.SetConnMaxIdleTime(time.Minute * 5)
		db.SetMaxOpenConns(config.MaxConnections)
		db.SetMaxIdleConns(config.MinConnections)

		logger.Info().
			Str(constants.KEY_PROCESS, "connecting to database").
			Msg("successed connecting to database")
	})
	return pool
}
