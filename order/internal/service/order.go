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

func (s OrderService) BatchCreateOrder(c context.Context, params []request.CreateOrder) error {
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
		Str(constants.KEY_TAG, "OrderService BatchCreateOrder").
		Int(constants.KEY_BATCH_ORDER_COUNT, orderCount).
		Any(constants.KEY_ORDERS, params).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "merge-order-item").Logger()
	logger.Trace().Msg("merging order items quantity")
	span.AddEvent("merging order items quantity")
	mapMergedOrderItem := map[string]mergedOrderItem{}
	mapOrder := map[string]request.CreateOrder{}
	productIds := make([]uuid.UUID, 0, 25)
	orderIds := make([]uuid.UUID, 0, orderCount)
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
	logger = logger.With().
		Any(constants.KEY_ORDER_ITEMS_MERGED, mapMergedOrderItem).
		Any(constants.KEY_PRODUCT_IDS, productIds).
		Logger()
	logger.Info().Msg("merged order items quantity")
	span.AddEvent("merged order items quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing-transaction").Logger()
	logger.Trace().Msg("initializing transaction")
	span.AddEvent("initializing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
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
		return err
	}
	logger = logger.With().Any(constants.KEY_PRODUCTS, products).Logger()
	logger.Info().Msg("got product quantity")
	span.AddEvent("got product quantity")

	span.AddEvent("check and decrease product quantity")
	logger.Trace().Msg("check and decrease product quantity")
	for _, product := range products {
		productId := product.ID.String()
		productIdAttr := attribute.String(constants.KEY_PRODUCT_ID, productId)

		lg := logger.With().Str(constants.KEY_PRODUCT_ID, productId).Logger()

		lg.Trace().Msg("checking order item have product id")
		span.AddEvent("checking order item have product id", trace.WithAttributes(productIdAttr))
		merged, ok := mapMergedOrderItem[productId]
		if !ok {
			lg.Info().Msg("order item does not have product id")
			span.AddEvent("order item does not have product id")
			continue
		}
		lg.Info().Msg("order item have product id")
		span.AddEvent("order item have product id", trace.WithAttributes(productIdAttr))

		lg.Trace().Msg("checking product quantity")
		span.AddEvent("checking product quantity", trace.WithAttributes(productIdAttr))
		if product.Quantity == 0 {
			lg.Info().Msg("product is out of stock")
			span.AddEvent("product is out of stock", trace.WithAttributes(productIdAttr))

			lg.Trace().Msg("removing merged order items")
			span.AddEvent("removing merged order items", trace.WithAttributes(productIdAttr))
			merged, ok := mapMergedOrderItem[productId]
			if !ok {
				lg.Info().Msg("merged order items not found")
				span.AddEvent("merged order items not found", trace.WithAttributes(productIdAttr))
				continue
			}
			lg.Info().Msg("merged order items found")
			span.AddEvent("merged order items found", trace.WithAttributes(productIdAttr))

			lg.Trace().Msg("removing order items from order")
			span.AddEvent("removing order items from order", trace.WithAttributes(productIdAttr))
			for _, item := range merged.Items {
				orderId := item.OrderID.String()
				orderItemId := item.ID.String()
				order := mapOrder[orderId]
				lg := lg.With().
					Str(constants.KEY_ORDER_ID, orderId).
					Str(constants.KEY_ORDER_ITEM_ID, orderItemId).
					Any(constants.KEY_ORDER, order).
					Logger()

				orderIdAttr := attribute.String(constants.KEY_ORDER_ID, orderId)
				orderItemIdAttr := attribute.String(constants.KEY_ORDER_ITEM_ID, orderItemId)

				lg.Trace().Msg("checking order still have order item")
				span.AddEvent(
					"checking order still have order item",
					trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
				)
				if len(order.OrderItems) == 0 {
					lg.Info().Msg("order is empty")
					span.AddEvent(
						"order is empty",
						trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
					)
					continue
				}
				lg.Info().Msg("order is not empty")
				span.AddEvent(
					"order is not empty",
					trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
				)
				lg.Trace().Msg("poping back order item from order")
				span.AddEvent(
					"poping back order item from order",
					trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
				)
				order.OrderItems = order.OrderItems[:len(order.OrderItems)-1]
				mapOrder[orderId] = order
				lg.Info().Msg("poped back order item from order")
				span.AddEvent(
					"poped back order item from order",
					trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
				)
			}
			span.AddEvent("removed order items from order", trace.WithAttributes(productIdAttr))
			lg.Info().Msg("removed order items from order")

			logger.Trace().Msg("removing order item from mergedOrderItem")
			span.AddEvent(
				"removing order item from mergedOrderItem",
				trace.WithAttributes(productIdAttr),
			)
			merged.OrderedItemQuantity = 0
			merged.Items = merged.Items[:0]
			mapMergedOrderItem[productId] = merged
			logger.Info().Msg("removed order item from mergedOrderItem")
			span.AddEvent(
				"removed order item from mergedOrderItem",
				trace.WithAttributes(productIdAttr),
			)
			continue
		}
		span.AddEvent("product is not out of stock")
		lg.Info().Msg("product is not out of stock")

		lg.Trace().Msg("decreasing order quantity")
		span.AddEvent("decreasing order quantity")
		for product.Quantity-merged.OrderedItemQuantity < 0 {
			lg.Trace().Msg("checking order items")
			span.AddEvent("checking order items")
			if len(merged.Items) == 0 {
				lg.Info().Msg("order items empty")
				span.AddEvent("order items empty")
				break
			}
			lg.Info().Msg("order items not empty")
			span.AddEvent("order items not empty")

			lastIndex := len(merged.Items) - 1
			orderItem := merged.Items[lastIndex]
			orderId := orderItem.OrderID.String()
			orderItemId := orderItem.ID.String()
			orderIdAttr := attribute.String(constants.KEY_ORDER_ID, orderId)
			orderItemIdAttr := attribute.String(constants.KEY_ORDER_ITEM_ID, orderItemId)
			productIdAttr := attribute.String(constants.KEY_PRODUCT_ID, productId)

			lg := lg.With().
				Str(constants.KEY_ORDER_ID, orderId).
				Str(constants.KEY_ORDER_ITEM_ID, orderItemId).
				Any(constants.KEY_ORDER_ITEM, orderItem).
				Logger()

			lg.Trace().Msg("poping back order item from merged order item")
			span.AddEvent(
				"poping back order item from merged order item",
				trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
			)
			merged.OrderedItemQuantity -= orderItem.Quantity
			merged.Items = merged.Items[:lastIndex]
			span.AddEvent(
				"poped back order item from merged order item",
				trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
			)
			lg.Info().Msg("poped back order item from merged order item")

			logger.Trace().Msg("checking order still have order item")
			span.AddEvent(
				"poping back order item from order",
				trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
			)
			order := mapOrder[orderId]
			order.OrderItems = order.OrderItems[:len(order.OrderItems)-1]
			mapOrder[orderId] = order
			span.AddEvent(
				"poped back order item from order",
				trace.WithAttributes(orderIdAttr, orderItemIdAttr, productIdAttr),
			)
			lg.Info().Msg("poped back order item from order")
		}
		mapMergedOrderItem[productId] = merged
		logger.Info().Msg("decreased order quantity")
		span.AddEvent("decreased order quantity")
	}
	span.AddEvent("checked and decreased product quantity")
	logger = logger.With().
		Any(constants.KEY_ORDER_ITEMS_MERGED, mapMergedOrderItem).
		Any(constants.KEY_ORDER_AND_ORDER_ITEMS, mapOrder).
		Logger()
	logger.Info().Msg("checked and decreased product quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "update-product-quantity").Logger()
	logger.Trace().Msg("preparing query for update product quantity")
	span.AddEvent("preparing query for update product quantity")
	var sb strings.Builder
	for productId, item := range mapMergedOrderItem {
		sb.WriteString(fmt.Sprintf(`when id = '%s' then %d `, productId, item.OrderedItemQuantity))
	}
	query := fmt.Sprintf(
		`update products set updated_at = now(), quantity = quantity - case %s end where id = any($1::uuid[]) returning *;`,
		sb.String(),
	)
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
		return err
	}
	_, err = pgx.CollectRows(rows, pgx.RowToStructByName[repository.Product])
	if err != nil {
		err = fmt.Errorf("failed updating product quantity with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		returnOrderError(c, params, err)
		return err
	}
	logger.Info().Msg("updated product quantity")
	span.AddEvent("updated product quantity")

	logger = logger.With().Str(constants.KEY_PROCESS, "create-order").Logger()
	logger.Trace().Msg("preparing order args")
	span.AddEvent("preparing order args")
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
		return err
	}
	logger.Info().Msg("inserted orders")
	span.AddEvent("inserted orders")

	logger.Trace().Msg("preparing insert order items args")
	span.AddEvent("preparing insert order items args")
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
	logger = logger.With().Any("insert_order_item_args", insertOrderItemArgs).Logger()
	logger.Info().Msg("prepared insert order items args")
	span.AddEvent("prepared insert order items args")
	logger.Trace().Msg("inserting order items")
	span.AddEvent("inserting order items")
	_, err = s.queries.WithTx(tx).InsertOrderItem(c, insertOrderItemArgs)
	if err != nil {
		err = fmt.Errorf("failed inserting order items with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		returnOrderError(c, params, err)
		return err
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
		return err
	}
	logger.Info().Msg("got orders")
	span.AddEvent("got orders")

	logger = logger.With().Str(constants.KEY_PROCESS, "preparing order response").Logger()
	logger.Trace().Msg("preparing order response")
	span.AddEvent("preparing order response")
	mapResponseOrder := map[string]response.Order{}
	for _, order := range orders {
		orderId := order.ID.String()
		orderExist, ok := mapOrder[orderId]
		if !ok || len(orderExist.OrderItems) == 0 {
			continue
		}
		orderResponse, err := order.Response()
		if err != nil {
			err = fmt.Errorf("failed getting orders with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			returnOrderError(c, params, err)
			return err
		}
		mapResponseOrder[orderId] = orderResponse
	}
	logger.Info().Msg("prepared order response")
	span.AddEvent("prepared order response")

	logger = logger.With().Str(constants.KEY_PROCESS, "commit-transaction").Logger()
	span.AddEvent("committing transaction")
	err = tx.Commit(c)
	if err != nil {
		err = fmt.Errorf("failed committing transaction with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		returnOrderError(c, params, err)
		return err
	}
	logger.Info().Msg("committed transaction")
	span.AddEvent("committed transaction")

	logger = logger.With().Str(constants.KEY_PROCESS, "sending result").Logger()
	logger.Trace().Msg("sending result to the orders")
	span.AddEvent("sending result to the orders")
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
	logger.Info().Msg("sent result to each order")
	span.AddEvent("sent result to each order")

	return nil
}

func returnOrderError(c context.Context, params []request.CreateOrder, err error) {
	c, span := otel.Tracer.Start(c, "OrderService-returnOrderResult")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "OrderService-returnOrderResult").
		Str(constants.KEY_PROCESS, "returning order error").
		Logger()

	var wg sync.WaitGroup
	for _, param := range params {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(param.ResultChannel)
			logger.Info().Msg("returning order error")
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
