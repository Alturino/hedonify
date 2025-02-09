package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/internal/common/constants"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/order/internal/common/cache"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/common/otel"
	internalRequest "github.com/Alturino/ecommerce/order/internal/request"
	"github.com/Alturino/ecommerce/order/internal/response"
	"github.com/Alturino/ecommerce/order/pkg/request"
)

type OrderService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
	cache   *redis.Client
}

func NewOrderService(
	pool *pgxpool.Pool,
	queries *repository.Queries,
	cache *redis.Client,
) *OrderService {
	return &OrderService{pool: pool, queries: queries, cache: cache}
}

func (s OrderService) FindOrderById(
	c context.Context,
	param internalRequest.FindOrderById,
) (response.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService FindOrderById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService FindOrderById").
		Str(log.KeyProcess, "finding order by id").
		Logger()

	logger.Info().Msg("finding order by id")
	order, err := s.queries.FindOrderById(
		c,
		repository.FindOrderByIdParams{ID: param.UserId, ID_2: param.OrderId},
	)
	if err != nil {
		err = fmt.Errorf("failed finding order by id with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("found order by id")

	logger = logger.With().Str(log.KeyProcess, "mapping order").Logger()
	logger.Info().Msg("mapping order")
	res, err := order.ResponseOrder()
	if err != nil {
		err = fmt.Errorf("failed mapping order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("mapped order")

	logger = logger.With().Str(log.KeyProcess, "finding order item by id").Logger()
	logger.Info().Msgf("finding order item by orderId=%s", param.OrderId)
	repoOrderItem, err := s.queries.FindOrderItemById(c, param.OrderId)
	if err != nil {
		err = fmt.Errorf("failed finding order by id=%s with error=%w", param.OrderId, err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
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

func (s OrderService) FindOrders(
	c context.Context,
	param internalRequest.FindOrders,
) ([]repository.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService FindOrders")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService FindOrders").
		Str(log.KeyProcess, "finding order by userId").
		Str(log.KeyUserID, param.UserId.String()).
		Str(log.KeyOrderID, param.OrderId.String()).
		Logger()

	logger.Info().Msg("finding orders")
	orders, err := s.queries.FindOrderUserId(c, param.UserId)
	if err != nil {
		err = fmt.Errorf(
			"failed finding order by id=%s userId=%s with error=%w",
			param.OrderId,
			param.UserId,
			err,
		)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Any(log.KeyOrders, orders).Msg("found orders")

	return orders, nil
}

func (s OrderService) CreateOrder(
	c context.Context,
	param request.CreateOrder,
) (response.Order, error) {
	c, span := otel.Tracer.Start(c, "OrderService CreateOrder")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_ORDER, param.ID)
	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService CreateOrder").
		Str(log.KeyCacheKey, cacheKey).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "initializing transaction").Logger()
	logger.Info().Msg("initializing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("initialized transaction")
	defer func() {
		logger := logger.With().Str(log.KeyProcess, "rolling back transaction").Logger()
		logger.Info().Msg("rolling back transaction")
		span.AddEvent("rolling back transaction")

		logger.Info().Msg("rolling back database transaction")
		span.AddEvent("rolling back database transaction")
		err := tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			if errors.Is(err, pgx.ErrTxClosed) {
				logger.Info().Err(err).Msg(err.Error())
				return
			}
			logger.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			return
		}
		logger.Info().Msg("rolled back database transaction")
		span.AddEvent("rolled back database transaction")

		logger.Info().Msg("rolling back order cache")
		span.AddEvent("rolling back order cache")
		err = s.cache.JSONDel(c, cacheKey, "$").Err()
		if err != nil {
			err = fmt.Errorf("failed deleting cache with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return
		}
		span.AddEvent("rolled back order cache")
		logger.Info().Msg("rolled back order cache")

		logger.Info().Msg("rolling back decrement product quantity")
		span.AddEvent("rolling back decrement product quantity")
		for _, item := range param.OrderItems {
			cacheKey := cache.KEY_PRODUCTS + item.ProductID.String()
			err := s.cache.JSONNumIncrBy(c, cacheKey, ".quantity", -float64(item.Quantity)).Err()
			if err != nil {
				err = fmt.Errorf(
					"failed rolling back decrement product quantity with error=%w",
					err,
				)
				commonErrors.HandleError(err, span)
				logger.Error().Err(err).Msg(err.Error())
				return
			}
		}
		logger.Info().Msg("rolled back decrement product quantity")
		span.AddEvent("rolled back decrement product quantity")

		logger.Info().Msg("rolled back transaction")
		span.AddEvent("rolled back transaction")
	}()

	logger = logger.With().Str(log.KeyProcess, "checking and decreasing product quantity").Logger()
	logger.Info().Msg("checking and decreasing product quantity")
	canceledItems := []request.OrderItem{}
	for i, item := range param.OrderItems {
		productKey := cache.KEY_PRODUCTS + item.ProductID.String()

		lg := logger.With().
			Str(log.KeyProductID, item.ProductID.String()).
			Str(log.KeyCacheKey, productKey).
			Int("itemCount", i).
			Logger()

		lg.Info().Msg("decreasing product quantity")
		str, err := s.cache.JSONNumIncrBy(c, productKey, ".quantity", -float64(item.Quantity)).
			Result()
		if err != nil {
			err = fmt.Errorf("failed decreasing product quantity with error=%w", err)
			lg.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			continue
		}
		lg = logger.With().Str(log.KeyProductQuantity, str).Logger()
		lg.Info().Msg("decreased product quantity")

		lg.Info().Msg("converting product quantity")
		quantity, err := strconv.Atoi(str)
		if err != nil {
			canceledItems = append(canceledItems, item)
			err = fmt.Errorf("failed converting quantity to int with error=%w", err)
			lg.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			continue
		}
		lg = logger.With().Int(log.KeyProductQuantity, quantity).Logger()
		lg.Info().Msg("converted product quantity")

		lg.Info().Msg("checking product quantity still available")
		if quantity < 0 {
			canceledItems = append(canceledItems, item)
			err = commonErrors.ErrOutOfStock
			lg.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
			continue
		}
	}
	if len(canceledItems) > 0 {
		logger = logger.With().
			Str(log.KeyProcess, "rolling back decrement product quantity").
			Logger()
		logger.Info().Msg("rolling back decrement product quantity")
		for _, item := range canceledItems {
			productKey := cache.KEY_PRODUCTS + item.ProductID.String()
			err = s.cache.JSONNumIncrBy(c, productKey, ".quantity", float64(item.Quantity)).Err()
			if err != nil {
				err = fmt.Errorf(
					"failed rolling back decrement product quantity with error=%w",
					errors.Join(err, commonErrors.ErrOutOfStock),
				)
				logger.Error().Err(err).Msg(err.Error())
				commonErrors.HandleError(err, span)
				return response.Order{}, nil
			}
		}
		logger.Info().Msg("rolling back decrement product quantity")
		return response.Order{}, err
	}

	logger = logger.With().Str(log.KeyProcess, "inserting order").Logger()
	logger.Info().Msg("inserting order")
	insertedOrder, err := s.queries.WithTx(tx).
		InsertOrder(c, repository.InsertOrderParams{ID: param.ID, UserID: param.UserId})
	if err != nil {
		err = fmt.Errorf("failed inserting order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("inserted order")

	logger = logger.With().Str(log.KeyProcess, "inserting orderItem").Logger()
	logger.Info().Msg("preparing orderItem")
	args := make([]repository.InsertOrderItemParams, len(param.OrderItems))
	for i, item := range param.OrderItems {
		args[i] = repository.InsertOrderItemParams{
			OrderID:   insertedOrder.ID,
			ProductID: item.ProductID,
			Price: pgtype.Numeric{
				Exp:              item.Price.Exponent(),
				InfinityModifier: pgtype.Finite,
				Int:              item.Price.Coefficient(),
				NaN:              false,
				Valid:            true,
			},
			Quantity: item.Quantity,
		}
	}
	logger = logger.With().Any(log.KeyOrderItems, args).Logger()
	logger.Info().Msg("prepared orderItem")

	logger = logger.With().Str(log.KeyProcess, "inserting orderItem").Logger()
	logger.Info().Msg("inserting orderItem")
	insertedOrderItem, err := s.queries.WithTx(tx).InsertOrderItem(c, args)
	if err != nil {
		err = fmt.Errorf("failed inserting orderItem with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msgf("inserted orderItem count=%d", insertedOrderItem)

	logger = logger.With().Str(log.KeyProcess, "get inserted order").Logger()
	logger.Info().Msg("getting inserted order")
	order, err := s.queries.WithTx(tx).
		FindOrderById(c, repository.FindOrderByIdParams{ID: param.UserId, ID_2: param.ID})
	if err != nil {
		err = fmt.Errorf("failed getting inserted order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger = logger.With().Any(log.KeyOrder, order).Logger()
	logger.Info().Msg("got inserted orderItems")

	logger = logger.With().Str(log.KeyProcess, "inserting order to cache").Logger()
	logger.Info().Msg("inserting order to cache")
	err = s.cache.JSONSet(c, cacheKey, "$", order).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting order to cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("inserted order to cache")

	logger = logger.With().Str(log.KeyProcess, "publishing update product quantity").Logger()
	logger.Info().Msg("publishing update product quantity")
	span.AddEvent("publishing update product quantity")
	paramJson, err := json.Marshal(param)
	if err != nil {
		err = fmt.Errorf("failed marshalling update product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		return response.Order{}, err
	}
	err = s.cache.Publish(c, constants.UPDATE_PRODUCT_QUANTITY, paramJson).Err()
	if err != nil {
		err = fmt.Errorf("failed publishing update product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		return response.Order{}, err
	}
	logger.Info().Msg("published update product quantity")
	span.AddEvent("published update product quantity")

	logger = logger.With().Str(log.KeyProcess, "committing transaction").Logger()
	logger.Info().Msg("committing transaction")
	err = tx.Commit(c)
	if err != nil {
		err = fmt.Errorf("failed committing transaction with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("commited transaction")

	logger = logger.With().Str(log.KeyProcess, "mapping order").Logger()
	logger.Info().Msg("mapping order")
	resp, err := order.ResponseOrder()
	if err != nil {
		err = fmt.Errorf("failed mapping order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("mapped order")

	return resp, nil
}
