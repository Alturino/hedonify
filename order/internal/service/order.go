package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/common/otel"
	inResponse "github.com/Alturino/ecommerce/order/internal/response"
	"github.com/Alturino/ecommerce/order/pkg/request"
	"github.com/Alturino/ecommerce/order/pkg/response"
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
	param request.FindOrders,
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

func (s OrderService) BatchCreateOrder(c context.Context, params []request.CreateOrder) error {
	traceLinks := make([]trace.Link, 0, len(params))
	for _, param := range params {
		traceLinks = append(traceLinks, param.TraceLink)
	}
	c, span := otel.Tracer.Start(c, "OrderService BatchCreateOrder", trace.WithLinks(traceLinks...))
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService BatchCreateOrder").
		Any(log.KeyOrders, params).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "merge-order-item").Logger()
	span.AddEvent("merging order items quantity")
	type mergedOrderItem struct {
		Items               []request.OrderItem `json:"items"`
		OrderedItemQuantity int32               `json:"ordered_item_quantity"`
	}
	mapMergedOrderItem := map[string]mergedOrderItem{}
	mapOrder := map[string]request.CreateOrder{}
	productIds := []uuid.UUID{}
	for _, order := range params {
		orderId := order.ID.String()
		_, ok := mapOrder[orderId]
		if !ok {
			mapOrder[orderId] = order
		}
		for _, orderItem := range order.OrderItems {
			productId := orderItem.ProductID.String()
			existing, ok := mapMergedOrderItem[productId]
			if !ok {
				mapMergedOrderItem[productId] = mergedOrderItem{
					Items:               []request.OrderItem{orderItem},
					OrderedItemQuantity: orderItem.Quantity,
				}
				productIds = append(productIds, orderItem.ProductID)
				continue
			}
			existing.OrderedItemQuantity += orderItem.Quantity
			existing.Items = append(existing.Items, orderItem)
			mapMergedOrderItem[productId] = existing
		}
	}
	logger = logger.With().
		Any(log.KeyOrderItemsMerged, mapMergedOrderItem).
		Any(log.KeyOrderAndItems, mapOrder).
		Any(log.KeyProductIDs, productIds).
		Logger()
	logger.Info().Msg("merged order items quantity")
	span.AddEvent("merged order items quantity")

	logger = logger.With().Str(log.KeyProcess, "initializing-transaction").Logger()
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
	}
	defer func() {
		logger.Info().Msg("rolling back transaction")
		span.AddEvent("rolling back transaction")
		err := tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			if errors.Is(err, pgx.ErrTxClosed) {
				logger.Info().Err(err).Msg(err.Error())
				span.AddEvent(err.Error())
				return
			}
			logger.Error().Err(err).Msg(err.Error())
			commonErrors.HandleError(err, span)
		}
		logger.Info().Msg("rolled back transaction")
		span.AddEvent("rolled back transaction")
	}()
	logger.Info().Msg("initialized transaction")
	span.AddEvent("initialized transaction")

	logger = logger.With().Str(log.KeyProcess, "check-quantity").Logger()
	span.AddEvent("get products")
	products, err := s.queries.WithTx(tx).FindProductsByIds(c, productIds)
	if err != nil {
		err = fmt.Errorf("failed get products with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
	}
	logger = logger.With().Any(log.KeyProducts, products).Logger()
	logger.Info().Msg("got products")
	span.AddEvent("got products")

	span.AddEvent("check and decrease product quantity")
	for _, product := range products {
		productId := product.ID.String()
		merged := mapMergedOrderItem[productId]
		logger.Info().Msg("checking quantity")
		for product.Quantity-merged.OrderedItemQuantity < 0 {
			if len(merged.Items) == 0 {
				logger.Info().Msg("no order items")
				break
			}
			lastOrderItem := merged.Items[len(merged.Items)-1]
			merged.OrderedItemQuantity -= lastOrderItem.Quantity
			merged.Items = merged.Items[:len(merged.Items)-1]

			orderId := lastOrderItem.OrderID.String()
			span.AddEvent(
				"poping back order item from order",
				trace.WithAttributes(
					attribute.String(log.KeyOrderID, orderId),
					attribute.String(log.KeyOrderItemID, lastOrderItem.ID.String()),
					attribute.String(log.KeyProductID, lastOrderItem.ProductID.String()),
				),
			)
			existing := mapOrder[orderId]
			existing.OrderItems = existing.OrderItems[:len(existing.OrderItems)-1]
			mapOrder[orderId] = existing
			span.AddEvent(
				"poped back order item from order",
				trace.WithAttributes(
					attribute.String(log.KeyOrderID, orderId),
					attribute.String(log.KeyOrderItemID, lastOrderItem.ID.String()),
					attribute.String(log.KeyProductID, lastOrderItem.ProductID.String()),
				),
			)
		}
		mapMergedOrderItem[productId] = merged
	}
	logger = logger.With().
		Any(log.KeyOrderItemsMerged, mapMergedOrderItem).
		Any(log.KeyOrderAndItems, mapOrder).
		Logger()
	logger.Info().Msg("checked and decreased product quantity")
	span.AddEvent("checked and decreased product quantity")

	logger = logger.With().Str(log.KeyProcess, "update-product-quantity").Logger()
	span.AddEvent("update product quantity")
	var sb strings.Builder
	for productId, item := range mapMergedOrderItem {
		sb.WriteString(fmt.Sprintf(`when id = '%s' then %d `, productId, item.OrderedItemQuantity))
	}
	query := fmt.Sprintf(
		`update products set updated_at = now(), quantity = quantity - case %s end where id = any($1::uuid[]) returning *;`,
		sb.String(),
	)
	logger = logger.With().Str("query", query).Logger()
	span.AddEvent("updating product quantity")
	rows, err := tx.Query(c, query, productIds)
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		returnOrderError(c, params, err)
		return err
	}
	_, err = pgx.CollectRows(rows, pgx.RowToStructByName[repository.Product])
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		returnOrderError(c, params, err)
		return err
	}
	logger.Info().Msg("updated product quantity")
	span.AddEvent("updated product quantity")

	logger = logger.With().Str(log.KeyProcess, "create-order").Logger()
	span.AddEvent("inserting orders")
	insertOrderArgs := []repository.InsertOrdersParams{}
	for _, item := range mapOrder {
		if len(item.OrderItems) == 0 {
			continue
		}
		insertOrderArgs = append(insertOrderArgs, repository.InsertOrdersParams{
			ID:     item.ID,
			UserID: item.UserId,
			CreatedAt: pgtype.Timestamptz{
				Time:             time.Now(),
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
			UpdatedAt: pgtype.Timestamptz{
				Time:             time.Now(),
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
		})
	}
	logger = logger.With().Any("insert_order_args", insertOrderArgs).Logger()
	_, err = s.queries.WithTx(tx).InsertOrders(c, insertOrderArgs)
	if err != nil {
		err = fmt.Errorf("failed inserting order with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
	}
	logger.Info().Msg("inserted orders")
	span.AddEvent("inserted orders")
	logger.Info().Msg("inserting order items")
	span.AddEvent("inserting order items")
	insertOrderItemArgs := []repository.InsertOrderItemParams{}
	for _, item := range mapMergedOrderItem {
		for _, orderItem := range item.Items {
			insertOrderItemArgs = append(insertOrderItemArgs, repository.InsertOrderItemParams{
				OrderID:   orderItem.OrderID,
				ProductID: orderItem.ProductID,
				Quantity:  orderItem.Quantity,
				Price: pgtype.Numeric{
					Exp:              orderItem.Price.Exponent(),
					InfinityModifier: pgtype.Finite,
					Int:              orderItem.Price.Coefficient(),
					NaN:              false,
					Valid:            true,
				},
			})
		}
	}
	_, err = s.queries.WithTx(tx).InsertOrderItem(c, insertOrderItemArgs)
	if err != nil {
		err = fmt.Errorf("failed inserting order items with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
	}
	logger.Info().Msg("inserted order items")
	span.AddEvent("inserted order items")

	orders, err := s.queries.WithTx(tx).GetOrders(c)
	if err != nil {
		err = fmt.Errorf("failed getting orders with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
	}
	mapResponseOrder := map[string]response.Order{}
	for _, order := range orders {
		orderId := order.ID.String()
		_, ok := mapOrder[orderId]
		if !ok {
			continue
		}
		orderResponse, err := order.Response()
		if err != nil {
			err = fmt.Errorf("failed getting orders with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			returnOrderError(c, params, err)
			return err
		}
		mapResponseOrder[orderId] = orderResponse
	}

	logger = logger.With().Str(log.KeyProcess, "commit-transaction").Logger()
	span.AddEvent("committing transaction")
	err = tx.Commit(c)
	if err != nil {
		err = fmt.Errorf("failed committing transaction with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		commonErrors.HandleError(err, span)
		var wg sync.WaitGroup
		for _, param := range params {
			wg.Add(1)
			go func() {
				defer wg.Done()
				param.ResultChannel <- inResponse.Result{Order: response.Order{}, Err: err}
			}()
		}
		wg.Wait()
		return err
	}
	logger.Info().Msg("committed transaction")
	span.AddEvent("committed transaction")

	var wg sync.WaitGroup
	for _, param := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orderId := param.ID.String()
			order, ok := mapResponseOrder[orderId]
			if !ok {
				param.ResultChannel <- inResponse.Result{Order: response.Order{}, Err: commonErrors.ErrOutOfStock}
				return
			}
			param.ResultChannel <- inResponse.Result{Order: order, Err: nil}
		}()
	}
	wg.Wait()
	return nil
}

func returnOrderError(c context.Context, params []request.CreateOrder, err error) {
	c, span := otel.Tracer.Start(c, "OrderService-returnOrderResult")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "OrderService-returnOrderResult").
		Str(log.KeyProcess, "returning order error").
		Logger()

	var wg sync.WaitGroup
	for _, param := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info().Msg("returning order error")
			param.ResultChannel <- inResponse.Result{Order: response.Order{}, Err: err}
			logger.Info().Msg("return order error")
		}()
	}
	wg.Wait()
}
