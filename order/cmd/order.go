package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Alturino/ecommerce/internal/config"
	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/infra"
	"github.com/Alturino/ecommerce/internal/log"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/controller"
	"github.com/Alturino/ecommerce/order/internal/otel"
	"github.com/Alturino/ecommerce/order/internal/service"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

func RunOrderService(c context.Context) {
	c, span := otel.Tracer.Start(c, "RunOrderService")
	defer span.End()

	cfg := config.Get(c, constants.APP_ORDER_SERVICE)

	logger := log.Get(filepath.Join("/var/log/", constants.APP_ORDER_SERVICE+".log"), cfg.Application).
		With().
		Str(constants.KEY_APP_NAME, constants.APP_ORDER_SERVICE).
		Str(constants.KEY_TAG, "main RunOrderService").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing router").Logger()
	logger.Info().Msg("initializing router")
	mux := mux.NewRouter()
	mux.Handle("/metrics", promhttp.Handler())
	logger.Info().Msg("initialized router")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing otel sdk").Logger()
	logger.Info().Msg("initializing otel sdk")
	c = logger.WithContext(c)
	shutdownFuncs, err := inOtel.InitOtelSdk(c, constants.APP_ORDER_SERVICE, cfg.Otel)
	if err != nil {
		err = fmt.Errorf("failed initializing otel sdk with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	defer func() {
		logger.Info().Msg("shutting down otel")
		c = logger.WithContext(c)
		err = inOtel.ShutdownOtel(c, shutdownFuncs)
		if err != nil {
			err = fmt.Errorf("failed shutting down otel with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("shutdown otel")
	}()
	logger.Info().Msg("initialized otel sdk")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing database").Logger()
	logger.Info().Msg("initializing database")
	db := infra.NewDatabaseClient(c, cfg.Database)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "closing database").Logger()
		logger.Info().Msg("closing database")
		db.Close()
		logger.Info().Msg("closed database")
	}()
	queries := repository.New(db)
	logger.Info().Msg("initialized database")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing cache").Logger()
	logger.Info().Msg("initializing cache")
	cache := infra.NewCacheClient(c, cfg.Cache)
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "closing cache").Logger()
		logger.Info().Msg("closing cache")
		err = cache.Close()
		if err != nil {
			err = fmt.Errorf("failed closing cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("closed cache")
	}()
	logger.Info().Msg("initialized cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing order service").Logger()
	logger.Info().Msg("initializing order service")
	c = logger.WithContext(c)
	orderService := service.NewOrderService(db, queries, cache)
	logger.Info().Msg("initialized order service")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing order controller").Logger()
	logger.Info().Msg("initializing order controller")
	queue := make(chan request.CreateOrder, 1)
	defer close(queue)
	controller.AttachOrderController(mux, orderService, queue)
	logger.Info().Msg("initializing order controller")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing server").Logger()
	logger.Info().Msg("initializing server")
	server := http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Application.Host, cfg.Application.Port),
		BaseContext: func(net.Listener) context.Context {
			lg := logger.With().
				Reset().
				Timestamp().
				Caller().
				Stack().
				Str(constants.KEY_APP_NAME, constants.APP_ORDER_SERVICE).
				Logger()
			c = lg.WithContext(c)
			return c
		},
		Handler:      mux,
		ReadTimeout:  45 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	defer func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "shutting down server").Logger()
		logger.Info().Msg("shutting down server")
		err = server.Shutdown(c)
		if err != nil {
			err = fmt.Errorf("failed shutting down server with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
		}
		logger.Info().Msg("shutdown server")
	}()
	logger.Info().Msg("initialized server")

	go func() {
		logger = logger.With().Str(constants.KEY_PROCESS, "start server").Logger()
		logger.Info().Msgf("start listening request at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
			err = fmt.Errorf("encounter error=%w while running server", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())

			c = logger.WithContext(c)
			if err := inOtel.ShutdownOtel(c, shutdownFuncs); err != nil {
				err = fmt.Errorf("failed shutting down otel with error=%w", err)
				inOtel.RecordError(err, span)
				logger.Error().Err(err).Msg(err.Error())
			}
			return
		}
		logger.Info().Msg("shutdown server")
	}()

	orderWorker := NewOrderWorker(orderService, queue)
	logger = logger.With().Str(constants.KEY_PROCESS, "start-worker").Logger()
	logger.Info().Msg("start order worker")
	span.AddEvent("start order worker")
	var wg sync.WaitGroup
	wg.Add(1)
	c = logger.WithContext(c)
	go orderWorker.StartWorker(c, &wg)
	wg.Wait()

	<-c.Done()
	logger = logger.With().Str(constants.KEY_PROCESS, "shutdown server").Logger()
	logger.Info().Msg("received interuption signal shutting down")
}
