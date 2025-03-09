package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/constants"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/order/internal/service"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

type OrderWorker struct {
	svc   *service.OrderService
	queue <-chan request.CreateOrder
}

func NewOrderWorker(svc *service.OrderService, queue <-chan request.CreateOrder) *OrderWorker {
	return &OrderWorker{svc: svc, queue: queue}
}

func (wrk OrderWorker) StartWorker(c context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	logger := zerolog.Ctx(c).
		With().
		Reset().
		Str(constants.KEY_TAG, "OrderWorker-StartWorker").
		Str(constants.KEY_PROCESS, "starting-worker").
		Str(constants.KEY_APP_NAME, constants.APP_ORDER_WORKER).
		Logger()

	tick := time.Tick(time.Millisecond * 300)
	batch := make([]request.CreateOrder, 0, 50)

	for {
		select {
		case <-c.Done():
			return
		case <-tick:
			if len(batch) == 0 {
				continue
			}
			requestID := uuid.NewString()
			logger = logger.With().Str(constants.KEY_REQUEST_ID, requestID).Logger()
			logger.Info().Msg("start batch create order")
			c = logger.WithContext(c)
			c = log.AttachRequestIDToContext(c, requestID)
			err := wrk.svc.BatchCreateOrder(c, batch)
			if err != nil {
				err = fmt.Errorf("failed batch create order with error=%w", err)
				logger.Error().Err(err).Msg(err.Error())
				continue
			}
			logger.Info().Msg("batch create order completed")
			batch = batch[:0]
		case order := <-wrk.queue:
			logger = logger.With().Any(constants.KEY_ORDER, order).Logger()
			logger.Info().Msg("received request create order")
			logger.Info().Msg("inserting order to batches")
			batch = append(batch, order)
			logger.Info().Msg("inserted order to batches")
		}
	}
}
