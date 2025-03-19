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

	"github.com/Alturino/ecommerce/internal/constants"
	inErrors "github.com/Alturino/ecommerce/internal/errors"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/order/internal/otel"
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
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService FindOrderById").
		Str(constants.KEY_PROCESS, "finding order by id").
		Logger()

	logger.Info().Msg("finding order by id")
	order, err := s.queries.FindOrderById(
		c,
		repository.FindOrderByIdParams{ID: param.UserId, ID_2: param.OrderId},
	)
	if err != nil {
		err = fmt.Errorf("failed finding order by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("found order by id")

	logger = logger.With().Str(constants.KEY_PROCESS, "mapping order").Logger()
	logger.Info().Msg("mapping order")
	res, err := order.ResponseOrder()
	if err != nil {
		err = fmt.Errorf("failed mapping order with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().Msg("mapped order")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding order item by id").Logger()
	logger.Info().Msgf("finding order item by orderId=%s", param.OrderId)
	repoOrderItem, err := s.queries.FindOrderItemById(c, param.OrderId)
	if err != nil {
		err = fmt.Errorf("failed finding order by id=%s with error=%w", param.OrderId, err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Order{}, err
	}
	logger.Info().
		Any(constants.KEY_ORDER_ITEMS, repoOrderItem).
		Msgf("found order item by id=%s", param.OrderId)

	logger = logger.With().
		Str(constants.KEY_PROCESS, "mapping orderItems").
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
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService FindOrders").
		Str(constants.KEY_PROCESS, "finding order by userId").
		Str(constants.KEY_USER_ID, param.UserId.String()).
		Str(constants.KEY_ORDER_ID, param.OrderId.String()).
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
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Any(constants.KEY_ORDERS, orders).Msg("found orders")

	return orders, nil
}

type mergedOrderItem struct {
	Items               []request.OrderItem `json:"items"`
	OrderedItemQuantity int32               `json:"ordered_item_quantity"`
}

func (s OrderService) BatchCreateOrder(
	c context.Context,
	params []request.CreateOrder,
) (map[string]response.Order, error) {
	traceLinks := createTraceLink(params)

	orderCount := len(params)
	c, span := otel.Tracer.Start(
		c,
		"OrderService BatchCreateOrder",
		trace.WithLinks(traceLinks...),
		trace.WithAttributes(attribute.Int(constants.KEY_BATCH_ORDER_COUNT, orderCount)),
	)
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService BatchCreateOrder").
		Int(constants.KEY_BATCH_ORDER_COUNT, orderCount).
		Logger()

	if orderCount == 0 {
		err := errors.New("no order received")
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return map[string]response.Order{}, err
	}

	logger = logger.With().Str(constants.KEY_PROCESS, "merge order items").Logger()
	logger.Trace().Msg("merging order items quantity")
	span.AddEvent("merging order items quantity")
	mapMergedOrderItem, mapOrder, productIds, orderIds := mergeOrderItems(c, params)
	logger = logger.With().Any(constants.KEY_PRODUCT_IDS, productIds).Logger()
	logger.Info().Msg("merged order items quantity")
	span.AddEvent("merged order items quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "initalizing transaction").Logger()
	logger.Trace().Msg("initializing transaction")
	span.AddEvent("initializing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	defer func() {
		logger.Trace().Msg("rolling back transaction")
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
			inOtel.RecordError(err, span)
			return
		}
		logger.Info().Msg("rolled back transaction")
		span.AddEvent("rolled back transaction")
	}()
	logger.Info().Msg("initialized transaction")
	span.AddEvent("initialized transaction")

	logger = logger.With().Str(constants.KEY_PROCESS, "check-quantity").Logger()
	logger.Trace().Msg("get product quantity")
	span.AddEvent("get product quantity")
	products, err := s.queries.WithTx(tx).FindProductsByIds(c, productIds)
	if err != nil {
		err = fmt.Errorf("failed get products with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger = logger.With().Any(constants.KEY_PRODUCTS, products).Logger()
	logger.Info().Msg("got product quantity")
	span.AddEvent("got product quantity")

	span.AddEvent("check and decrease product quantity")
	logger.Trace().Msg("check and decrease product quantity")
	mapMergedOrderItem, mapOrder = checkDecreaseQuantity(c, products, mapMergedOrderItem, mapOrder)
	span.AddEvent("checked and decreased product quantity")
	logger = logger.With().Logger()
	logger.Info().Msg("checked and decreased product quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "update-product-quantity").Logger()
	logger.Trace().Msg("preparing query for update product quantity")
	span.AddEvent("preparing query for update product quantity")
	query := buildQuery(c, mapMergedOrderItem)
	logger = logger.With().Str(constants.KEY_QUERY, query).Logger()
	logger.Info().Msg("prepared query for update product quantity")
	span.AddEvent("prepared query for update product quantity")

	logger.Trace().Msg("updating product quantity")
	span.AddEvent("updating product quantity")
	rows, err := tx.Query(c, query, productIds)
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	_, err = pgx.CollectRows(rows, pgx.RowToStructByName[repository.Product])
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger.Info().Msg("updated product quantity")
	span.AddEvent("updated product quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "create-order").Logger()
	logger.Trace().Msg("preparing order args")
	span.AddEvent("preparing order args")
	insertOrderArgs := prepareOrderArgs(c, mapOrder)
	span.AddEvent("prepared order args")
	logger = logger.With().Any("insert_order_args", insertOrderArgs).Logger()
	logger.Info().Msg("prepared order args")

	logger.Trace().Msg("inserting orders")
	span.AddEvent("inserting orders")
	_, err = s.queries.WithTx(tx).InsertOrders(c, insertOrderArgs)
	if err != nil {
		err = fmt.Errorf("failed inserting order with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger.Info().Msg("inserted orders")
	span.AddEvent("inserted orders")

	logger.Trace().Msg("preparing insert order items args")
	span.AddEvent("preparing insert order items args")
	insertOrderItemArgs := prepareOrderItemArgs(c, mapMergedOrderItem)
	span.AddEvent("prepared insert order items args")
	logger = logger.With().Any("insert_order_item_args", insertOrderItemArgs).Logger()
	logger.Info().Msg("prepared insert order items args")

	logger.Trace().Msg("inserting order items")
	span.AddEvent("inserting order items")
	_, err = s.queries.WithTx(tx).InsertOrderItem(c, insertOrderItemArgs)
	if err != nil {
		err = fmt.Errorf("failed inserting order items with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger.Info().Msg("inserted order items")
	span.AddEvent("inserted order items")

	logger = logger.With().Str(constants.KEY_PROCESS, "get orders").Logger()
	logger.Trace().Msg("getting orders")
	span.AddEvent("getting orders")
	orders, err := s.queries.WithTx(tx).GetOrders(c, orderIds)
	if err != nil {
		err = fmt.Errorf("failed getting orders with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger.Info().Msg("got orders")
	span.AddEvent("got orders")

	logger = logger.With().Str(constants.KEY_PROCESS, "preparing order response").Logger()
	logger.Trace().Msg("preparing order response")
	span.AddEvent("preparing order response")
	mapResponseOrder := orderToResponse(c, orders, mapOrder)
	span.AddEvent("prepared order response")
	logger.Info().Msg("prepared order response")

	logger = logger.With().Str(constants.KEY_PROCESS, "commit-transaction").Logger()
	span.AddEvent("committing transaction")
	err = tx.Commit(c)
	if err != nil {
		err = fmt.Errorf("failed committing transaction with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		returnOrderError(c, params, err)
		return map[string]response.Order{}, err
	}
	logger.Info().Msg("committed transaction")
	span.AddEvent("committed transaction")

	logger = logger.With().Str(constants.KEY_PROCESS, "sending result").Logger()
	logger.Trace().Msg("sending result to the orders")
	span.AddEvent("sending result to the orders")
	returnOrderResult(c, params, mapResponseOrder)
	span.AddEvent("sent result to each order")
	logger.Info().Msg("sent result to each order")

	return mapResponseOrder, nil
}

func returnOrderError(c context.Context, params []request.CreateOrder, err error) {
	c, span := otel.Tracer.Start(c, "OrderService-returnOrderResult")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService-returnOrderResult").
		Str(constants.KEY_PROCESS, "returning order error").
		Logger()

	var wg sync.WaitGroup
	for _, param := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(param.ResultChannel)
			logger.Trace().Msg("returning order error")
			param.ResultChannel <- inResponse.Result{Order: response.Order{}, Err: err}
			logger.Info().Msg("return order error")
		}()
	}
	wg.Wait()
}

func createTraceLink(params []request.CreateOrder) []trace.Link {
	traceLinks := make([]trace.Link, len(params))
	var wg sync.WaitGroup
	for i, param := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			traceLinks[i] = param.TraceLink
		}()
	}
	wg.Wait()
	return traceLinks
}

func returnOrderResult(
	c context.Context,
	params []request.CreateOrder,
	mapResponseOrder map[string]response.Order,
) {
	c, span := otel.Tracer.Start(c, "OrderService returnOrderResult")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService returnOrderResult").
		Logger()

	var wg sync.WaitGroup
	for _, param := range params {
		ld := logger.With().Str(constants.KEY_ORDER_ID, param.ID.String()).Logger()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(param.ResultChannel)
			orderId := param.ID.String()
			ld.Trace().Msg("checking is order created")
			order, ok := mapResponseOrder[orderId]
			if !ok {
				ld.Debug().Msg("order is not created")
				param.ResultChannel <- inResponse.Result{Order: response.Order{}, Err: inErrors.ErrOutOfStock}
				return
			}
			ld.Debug().Msg("order is created")
			ld.Trace().Msg("sending result to order request")
			param.ResultChannel <- inResponse.Result{Order: order, Err: nil}
			ld.Debug().Msg("sent order result to order request")
		}()
	}
	wg.Wait()
}

func prepareOrderArgs(
	c context.Context,
	mapOrder map[string]request.CreateOrder,
) []repository.InsertOrdersParams {
	_, span := otel.Tracer.Start(c, "OrderService prepareOrderArgs")
	defer span.End()

	insertOrderArgs := make([]repository.InsertOrdersParams, 0, len(mapOrder))
	for _, order := range mapOrder {
		if len(order.OrderItems) == 0 {
			continue
		}
		insertOrderArgs = append(insertOrderArgs, repository.InsertOrdersParams{
			ID:     order.ID,
			UserID: order.UserId,
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
	return insertOrderArgs
}

func prepareOrderItemArgs(
	c context.Context,
	mapMergedOrderItem map[string]mergedOrderItem,
) []repository.InsertOrderItemParams {
	_, span := otel.Tracer.Start(c, "OrderService prepareOrderItemArgs")
	defer span.End()

	insertOrderItemArgs := make([]repository.InsertOrderItemParams, 0, len(mapMergedOrderItem))
	for _, item := range mapMergedOrderItem {
		for _, orderItem := range item.Items {
			insertOrderItemArgs = append(insertOrderItemArgs, repository.InsertOrderItemParams{
				ID:        orderItem.ID,
				OrderID:   orderItem.OrderID,
				ProductID: orderItem.ProductID,
				Quantity:  orderItem.Quantity,
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
	return insertOrderItemArgs
}

func buildQuery(c context.Context, mapMergedOrderItem map[string]mergedOrderItem) string {
	_, span := otel.Tracer.Start(c, "OrderService buildQuery")
	defer span.End()

	var sb strings.Builder
	for productId, item := range mapMergedOrderItem {
		sb.WriteString(fmt.Sprintf(`when id = '%s' then %d `, productId, item.OrderedItemQuantity))
	}
	query := fmt.Sprintf(
		`update products set updated_at = now(), quantity = quantity - case %s end where id = any($1::uuid[]) returning *;`,
		sb.String(),
	)
	return query
}

func mergeOrderItems(
	c context.Context,
	params []request.CreateOrder,
) (map[string]mergedOrderItem, map[string]request.CreateOrder, []uuid.UUID, []uuid.UUID) {
	c, span := otel.Tracer.Start(c, "OrderService mergeOrderItems")
	defer span.End()

	orderCount := len(params)
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService mergeOrderItems").
		Int(constants.KEY_BATCH_ORDER_COUNT, orderCount).
		Logger()

	mapMergedOrderItem := make(map[string]mergedOrderItem, orderCount)
	mapOrder := make(map[string]request.CreateOrder, orderCount)
	productIds := make([]uuid.UUID, 0, orderCount)
	orderIds := make([]uuid.UUID, 0, orderCount)
	logger.Trace().Msg("merging order items quantity")
	span.AddEvent("merging order items quantity")
	for _, order := range params {
		orderId := order.ID.String()
		orderIds = append(orderIds, order.ID)
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
	span.AddEvent("merged order items quantity")
	logger.Info().Msg("merged order items quantity")

	return mapMergedOrderItem, mapOrder, productIds, orderIds
}

func checkDecreaseQuantity(
	c context.Context,
	products []repository.Product,
	mapMergedOrderItem map[string]mergedOrderItem,
	mapOrder map[string]request.CreateOrder,
) (map[string]mergedOrderItem, map[string]request.CreateOrder) {
	_, span := otel.Tracer.Start(c, "OrderService checkDecreaseQuantity")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "OrderService checkDecreaseQuantity").
		Logger()

	logger.Trace().
		Any(constants.KEY_ORDERS, mapOrder).
		Any(constants.KEY_CART_ITEMS_MERGED, mapMergedOrderItem).
		Msg("start checking product")
	span.AddEvent("start checking product")
	for _, product := range products {
		productId := product.ID.String()
		lg := logger.With().Str(constants.KEY_PRODUCT_ID, productId).Logger()
		merged, ok := mapMergedOrderItem[productId]
		if !ok {
			lg.Debug().Msg("no order items with product id")
			continue
		}
	check:
		for product.Quantity-merged.OrderedItemQuantity < 0 {
			if len(merged.Items) == 0 {
				lg.Debug().Msg("order items empty")
				break check
			}
			endOrderItemIndex := len(merged.Items) - 1
			orderItem := merged.Items[endOrderItemIndex]
			orderId := orderItem.OrderID.String()

			ld := lg.With().
				Str(constants.KEY_ORDER_ID, orderId).
				Str(constants.KEY_ORDER_ITEM_ID, orderItem.ID.String()).
				Int32(constants.KEY_ORDER_ITEM_QUANTITY, merged.OrderedItemQuantity).
				Logger()

			ld.Trace().Msg("reducing order item quantity")
			merged.OrderedItemQuantity -= orderItem.Quantity
			ld = ld.With().Int32(constants.KEY_ORDER_ITEM_QUANTITY, merged.OrderedItemQuantity).Logger()
			ld.Debug().Msg("reduced order item quantity")

			ld.Trace().Msg("removing order item from merged order items")
			merged.Items = merged.Items[:endOrderItemIndex]
			ld.Debug().Msg("removed order item from merged order items")

			ld.Trace().Msg("checking if order item exist in map order")
			if order, ok := mapOrder[orderId]; ok {
				ld.Debug().Msg("order item exist in map order")
				ld.Trace().Msg("removing order item from map order")
				order.OrderItems = order.OrderItems[:len(order.OrderItems)-1]
				mapOrder[orderId] = order
				ld.Debug().Msg("removed order item from map order")
			}
			ld.Debug().Msg("order item not exist in map order")
		}
		mapMergedOrderItem[productId] = merged
	}
	span.AddEvent("finished checking product")
	logger.Info().
		Any(constants.KEY_ORDERS, mapOrder).
		Any(constants.KEY_ORDER_ITEMS_MERGED, mapMergedOrderItem).
		Msg("finished checking product")

	return mapMergedOrderItem, mapOrder
}

func orderToResponse(
	c context.Context,
	orders []repository.GetOrdersRow,
	mapOrder map[string]request.CreateOrder,
) map[string]response.Order {
	c, span := otel.Tracer.Start(c, "OrderService mapOrderToResponse")
	defer span.End()

	logger := zerolog.Ctx(c).With().Logger()

	mapResponseOrder := map[string]response.Order{}
	for _, order := range orders {
		orderId := order.ID.String()
		orderReq, ok := mapOrder[orderId]
		if !ok || len(orderReq.OrderItems) == 0 {
			continue
		}
		orderRes, err := order.Response()
		if err != nil {
			err = fmt.Errorf("failed getting orders with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			continue
		}
		mapResponseOrder[orderId] = orderRes
	}
	return mapResponseOrder
}
