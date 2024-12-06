package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/order/internal/common/otel"
	"github.com/Alturino/ecommerce/order/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/request"
	"github.com/Alturino/ecommerce/order/internal/response"
)

type OrderService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
}

func NewOrderService(pool *pgxpool.Pool, queries *repository.Queries) OrderService {
	return OrderService{pool: pool, queries: queries}
}

func (s *OrderService) InsertOrder(
	c context.Context,
	param request.InsertOrderRequest,
) (repository.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService InsertOrder")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService InsertOrder").
		Str(log.KeyProcess, "initalizing transaction").
		Logger()

	logger.Info().Msg("initalizing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initalizing transaction with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Order{}, err
	}
	logger.Info().Msg("initialized transaction")
	defer func() {
		logger = logger.With().Str(log.KeyProcess, "rollback transaction").Logger()

		logger.Info().Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		logger.Info().Msg("rolled back transaction")
	}()

	logger = logger.With().
		Str(log.KeyProcess, "inserting order request").
		Logger()

	logger.Info().Msg("inserting order request")
	order, err := s.queries.InsertOrder(
		c,
		repository.InsertOrderParams{ID: param.CartId, UserID: param.UserId},
	)
	if err != nil {
		logger.Error().Err(err).Msgf("failed inserting order request with error=%s", err.Error())
		return repository.Order{}, err
	}
	logger = logger.With().
		Any(log.KeyOrder, order).
		Logger()
	logger.Info().Msg("inserted order request")

	logger = logger.With().
		Str(log.KeyProcess, "inserting order item request").
		Logger()

	logger.Info().Msg("inserting order item")
	args := []repository.InsertOrderItemParams{}
	for i, item := range param.OrderItems {
		price, err := decimal.NewFromString(item.Price)
		if err != nil {
			logger.Error().Err(err).
				Msgf("order item=%d price is not valid price with error=%s", i, err.Error())
			return repository.Order{}, err
		}
		args = append(
			args,
			repository.InsertOrderItemParams{
				OrderID:   order.ID,
				ProductID: item.ProductId,
				Quantity:  int32(item.Quantity),
				Price: pgtype.Numeric{
					Int:              price.BigInt(),
					Exp:              price.Exponent(),
					NaN:              false,
					InfinityModifier: pgtype.Finite,
					Valid:            true,
				},
			},
		)
	}
	insertedCount, err := s.queries.InsertOrderItem(c, args)
	if err != nil || insertedCount <= 0 {
		err = fmt.Errorf("failed inserting order item with error=%s", err.Error())
		logger.Error().Err(err).Msg(err.Error())
		return repository.Order{}, err
	}
	logger.Info().Msgf("inserted order item with count=%d", insertedCount)

	logger.Info().Msg("inserted order request")

	return order, nil
}

func (s *OrderService) FindOrderById(
	c context.Context,
	param request.FindOrderById,
) (response.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService FindOrderById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService FindOrderById").
		Str(log.KeyProcess, "finding order by id").
		Logger()

	logger.Info().Msg("finding order by id")
	order, err := s.queries.FindOrderById(c, param.OrderId)
	if err != nil {
		err = fmt.Errorf("failed finding order by id with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Any(log.KeyOrder, order).Msg("found order by id")

	res := response.Order{ID: order.ID, UserId: order.UserID}

	logger = logger.With().
		Str(log.KeyProcess, "finding order item by id").
		Any(log.KeyOrder, order).
		Logger()

	logger.Info().Msgf("finding order item by orderId=%s", param.OrderId)
	repoOrderItem, err := s.queries.FindOrderItemById(c, param.OrderId)
	if err != nil {
		err = fmt.Errorf("failed finding order by id=%s with error=%w", param.OrderId, err)
		logger.Error().Err(err).
			Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().
		Any(log.KeyOrderItems, repoOrderItem).
		Msgf("found order item by id=%s", param.OrderId)

	logger = logger.With().
		Str(log.KeyProcess, "mapping orderItems").
		Logger()

	logger.Info().Msgf("mapping orderItems")
	orderItems := []response.OrderItem{}
	for _, i := range repoOrderItem {
		orderItems = append(
			orderItems,
			response.OrderItem{
				ID:        i.ID,
				OrderId:   i.OrderID,
				ProductId: i.ProductID,
				Quantity:  i.Quantity,
				Price:     decimal.New(i.Price.Int.Int64(), i.Price.Exp),
			},
		)
	}
	res.OrderItems = orderItems
	logger.Info().Msgf("mapped orderItems")

	return res, nil
}

func (s *OrderService) FindOrders(
	c context.Context,
	param request.FindOrders,
) ([]repository.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService FindOrders").
		Str(log.KeyProcess, "finding order by userId").
		Str(log.KeyUserID, param.UserId).
		Str(log.KeyOrderID, param.OrderId).
		Logger()

	logger.Info().Msg("finding orders")
	orders, err := s.queries.FindOrderByIdAndUserId(
		c,
		repository.FindOrderByIdAndUserIdParams{Column1: param.OrderId, Column2: param.UserId},
	)
	if err != nil {
		err = fmt.Errorf(
			"failed finding order by id=%s userId=%s with error=%w",
			param.OrderId,
			param.UserId,
			err,
		)
		logger.Error().Err(err).
			Msg(err.Error())
		return nil, err
	}
	logger.Info().
		Any(log.KeyOrders, orders).
		Msg("found orders")

	return orders, nil
}
