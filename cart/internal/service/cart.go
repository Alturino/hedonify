package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/cart/internal/cache"
	"github.com/Alturino/ecommerce/cart/internal/otel"
	"github.com/Alturino/ecommerce/cart/pkg/request"
	"github.com/Alturino/ecommerce/cart/pkg/response"
	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/log"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
)

type CartService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
	cache   *redis.Client
}

func NewCartService(
	pool *pgxpool.Pool,
	queries *repository.Queries,
	cache *redis.Client,
) CartService {
	return CartService{pool: pool, queries: queries, cache: cache}
}

func (svc CartService) InsertCart(
	c context.Context,
	param request.Cart,
	userID uuid.UUID,
) (response.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService InsertCart").
		Int(constants.KEY_CART_ITEMS, len(param.CartItems)).
		Str(constants.KEY_USER_ID, userID.String()).
		Logger()

	logger = logger.With().
		Str(constants.KEY_PROCESS, fmt.Sprintf("finding user by userId=%s in %s", userID.String(), constants.APP_USER_SERVICE)).
		Logger()
	logger.Info().Msgf("finding user by userId=%s", userID.String())
	req, err := http.NewRequestWithContext(
		c,
		http.MethodGet,
		constants.URL_USER_SERVICE+"/"+userID.String(),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("failed getting userId=%s with error=%w", userID.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	requestId := log.RequestIDFromContext(c)
	req.Header.Add(log.KEY_REQUEST_ID, requestId)
	resp, err := otelhttp.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed getting userId=%s with error=%w", userID.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("userId=%s not found", userID.String())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger.Info().Msgf("found user by userId=%s", userID.String())

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing transaction").Logger()
	logger.Info().Msg("initializing transaction")
	tx, err := svc.pool.BeginTx(c, pgx.TxOptions{})
	logger.Info().Msg("initialized transaction")
	defer func() {
		lg := logger.With().Str(constants.KEY_PROCESS, "rolling back transaction").Logger()
		lg.Trace().Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			if errors.Is(err, pgx.ErrTxClosed) {
				logger.Info().Err(err).Msg(err.Error())
				return
			}
			lg.Error().Err(err).Msg(err.Error())
			inOtel.RecordError(err, span)
			return
		}
		lg.Info().Msg("rolled back transaction")
	}()

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart to database").Logger()
	logger.Info().Msg("inserting cart to database")
	cart, err := svc.queries.WithTx(tx).InsertCart(c, userID)
	if err != nil {
		err = fmt.Errorf("failed inserting cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger = logger.With().Any(constants.KEY_CART, cart).Logger()
	logger.Info().Msg("inserted cart to database")

	logger.Info().Msg("merging cart items")
	span.AddEvent("merging cart items")
	mp := map[string]request.CartItem{}
	merged := []request.CartItem{}
	for _, item := range param.CartItems {
		lg := logger.With().
			Str(constants.KEY_PRODUCT_ID, item.ProductId.String()).
			Int32(constants.KEY_CART_ITEM_QUANTITY, item.Quantity).
			Logger()
		existing, ok := mp[item.ProductId.String()]
		if ok {
			lg.Info().Msg("merging cart item")
			mp[item.ProductId.String()] = request.CartItem{
				ProductId: item.ProductId,
				Price:     item.Price,
				Quantity:  existing.Quantity + item.Quantity,
			}
			lg.Info().
				Int32(constants.KEY_CART_ITEM_MERGED_QUANTITY, existing.Quantity+item.Quantity).
				Msg("merged cart item")
			continue
		}
		mp[item.ProductId.String()] = item
	}
	for _, item := range mp {
		merged = append(merged, item)
	}
	param.CartItems = merged
	span.AddEvent("merged cart items")
	logger = logger.With().Any(constants.KEY_CART_ITEMS_MERGED, merged).Logger()
	logger.Info().Msg("merged cart items")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart items to database").Logger()
	logger.Info().Msg("inserting cart items to database")
	args := make([]repository.InsertCartItemsParams, len(param.CartItems))
	for i, item := range param.CartItems {
		args[i] = repository.InsertCartItemsParams{
			ID:        uuid.New(),
			CartID:    cart.ID,
			ProductID: item.ProductId,
			Quantity:  item.Quantity,
			Price: pgtype.Numeric{
				Exp:              item.Price.Exponent(),
				InfinityModifier: pgtype.Finite,
				Int:              item.Price.Coefficient(),
				NaN:              false,
				Valid:            true,
			},
		}
	}
	logger = logger.With().Any(constants.KEY_CART_ITEMS, args).Logger()
	logger.Info().Msg("inserting cart items to database")
	insertedCount, err := svc.queries.WithTx(tx).InsertCartItems(c, args)
	if err != nil {
		err = fmt.Errorf("failed inserting cartItems to database with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger = logger.With().Int64(constants.KEY_CART_ITEMS_COUNT, insertedCount).Logger()
	logger.Info().Msgf("inserted %d cartItems to database", insertedCount)

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cart by id").Logger()
	logger.Info().Msg("finding cart by id")
	cartDb, err := svc.queries.WithTx(tx).
		FindCartById(c, repository.FindCartByIdParams{ID: userID, ID_2: cart.ID})
	if err != nil {
		err = fmt.Errorf("failed finding cart by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger = logger.With().RawJSON(constants.KEY_CART_ITEMS, cartDb.CartItems).Logger()
	logger.Info().Msg("found cart by id")

	logger = logger.With().Str(constants.KEY_PROCESS, "mapping cart").Logger()
	logger.Info().Msg("mapping cart")
	cartResponse, err := cartDb.Response()
	if err != nil {
		err = fmt.Errorf("failed mapping cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger = logger.With().Any(constants.KEY_CART_RESPONSE, cartResponse).Logger()
	logger.Info().Msg("mapped cart")

	cacheKey := fmt.Sprintf(cache.KEY_CARTS, cart.ID.String())
	logger = logger.With().
		Str(constants.KEY_PROCESS, "inserting cart to cache").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()
	logger.Info().Msg("inserting cart to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", cartResponse).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting cart to cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger.Info().Msg("inserted cart to cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "committing transaction").Logger()
	logger.Info().Msg("committing transaction")
	err = tx.Commit(c)
	if err != nil {
		newErr := svc.cache.JSONDel(c, cacheKey, "$").Err()
		if newErr != nil {
			err = fmt.Errorf(
				"failed committing transaction with error=%w",
				errors.Join(err, newErr),
			)
		}
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger.Info().Msg("committed transaction")

	return cartResponse, nil
}

func (s CartService) FindCartById(
	c context.Context,
	param request.FindCartById,
) (cart response.Cart, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartById")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_CARTS, param.ID.String())

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService FindCartById").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	jsonCache, err := s.cache.JSONGet(c, cacheKey).Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart in cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in db").Logger()
		cart, err := s.queries.FindCartById(
			c,
			repository.FindCartByIdParams{ID: param.ID, ID_2: param.UserId},
		)
		if err != nil {
			err = fmt.Errorf("failed finding cart in db with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		logger = logger.With().Any(constants.KEY_CART, cart).Logger()
		logger.Info().Msg("found cart in db")

		logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart in cache").Logger()
		err = s.cache.JSONSet(c, cacheKey, "$", cart).Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cart in cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		logger.Info().Msg("inserted cart in cache")

		logger = logger.With().Str(constants.KEY_PROCESS, "mapping cart").Logger()
		logger.Info().Msg("mapping cart")
		cartResponse, err := cart.Response()
		if err != nil {
			err = fmt.Errorf("failed mapping cart with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		logger = logger.With().Any(constants.KEY_CART_RESPONSE, cartResponse).Logger()
		logger.Info().Msg("mapped cart")

		return cart.Response()
	}
	logger = logger.With().RawJSON(constants.KEY_JSON_CACHE, []byte(jsonCache)).Logger()
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonCache), &cart)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger.Info().Msg("unmarshaled cache")

	return cart, nil
}

func (s CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) (carts []repository.FindCartByUserIdRow, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserId")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService FindCartByUserId").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	jsonString, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_CARTS_BY_USER_ID, userId.String())).
		Result()
	if err != nil {
		err = fmt.Errorf("failed finding cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())

		logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in db").Logger()
		logger.Info().Msg("finding cart in db")
		carts, err := s.queries.FindCartByUserId(c, userId)
		if err != nil {
			err = fmt.Errorf("failed finding cart in db with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msg("found cart in db")

		logger = logger.With().Str(constants.KEY_PROCESS, "marshaling cache").Logger()
		logger.Info().Msg("marshaling cache")
		json, err := json.Marshal(carts)
		if err != nil {
			err = fmt.Errorf("failed marshaling cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msg("marshaled cache")

		logger = logger.With().Str(constants.KEY_PROCESS, "inserting cache").Logger()
		logger.Info().Msg("inserting cache")
		err = s.cache.Set(c, fmt.Sprintf(cache.KEY_CARTS_BY_USER_ID, userId.String()), json, time.Hour*1).
			Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msg("inserted cache")
		return carts, err
	}
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonString), &carts)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("unmarshaled cache")

	return carts, nil
}

func (s CartService) RemoveCartItem(c context.Context, param request.RemoveCartItem) error {
	c, span := otel.Tracer.Start(c, "CartService RemoveCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService RemoveCartItem").
		Str(constants.KEY_PROCESS, "finding cartId").
		Logger()

	logger.Info().Msg("finding cartId")
	_, err := s.queries.FindCartById(
		c,
		repository.FindCartByIdParams{ID: param.CartId, ID_2: param.UserId},
	)
	if err != nil {
		err = fmt.Errorf("failed finding cartId=%s with error=%w", param.ID.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger.Info().Msg("found cartId")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cartItemId").Logger()
	logger.Info().Msg("finding cartItemId")
	_, err = s.queries.FindCartItemById(c, param.ID)
	if err != nil {
		err = fmt.Errorf(
			"failed finding cartItemId=%s in cartId=%s with error=%w",
			param.ID.String(),
			param.CartId.String(),
			err,
		)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger.Info().Msg("found cartItemId")

	logger = logger.With().Str(constants.KEY_PROCESS, "deleting.cart.from.cache").Logger()
	logger.Info().Msg("deleting cart from cache")
	err = s.cache.Del(c, cache.KEY_CARTS+param.CartId.String()).Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart from cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger.Info().Msg("deleted cart from cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "deleting cartItem").Logger()
	logger.Info().Msg("deleting cartItem")
	_, err = s.queries.DeleteCartItemFromCartsById(
		c,
		repository.DeleteCartItemFromCartsByIdParams{ID: param.ID, CartID: param.CartId},
	)
	if err != nil {
		err = fmt.Errorf(
			"failed deleting cartItemId=%s in cartId=%s with error=%w",
			param.ID.String(),
			param.CartId.String(),
			err,
		)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger.Info().Msg("deleted cartItem")

	return nil
}

func (s CartService) RemoveCart(c context.Context, param request.RemoveCart) error {
	c, span := otel.Tracer.Start(c, "CartService RemoveCart")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_CARTS, param.ID.String())

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService RemoveCart").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	msg := fmt.Sprintf("finding cartId=%s and userId=%s", param.ID.String(), param.UserId.String())
	logger = logger.With().Str(constants.KEY_PROCESS, msg).Logger()
	logger.Info().Msg(msg)
	span.AddEvent(msg)
	_, err := s.FindCartById(c, request.FindCartById(param))
	if err != nil {
		err = fmt.Errorf("failed finding cartId=%s with error=%w", param.ID.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	msg = fmt.Sprintf("found cartId=%s and userId=%s", param.ID.String(), param.UserId.String())
	span.AddEvent(msg)
	logger.Info().Msg(msg)

	msg = fmt.Sprintf(
		"deleting cartId=%s and userId=%s from database",
		param.ID.String(),
		param.UserId.String(),
	)
	logger = logger.With().Str(constants.KEY_PROCESS, msg).Logger()
	span.AddEvent(msg)
	logger.Info().Msg(msg)
	_, err = s.queries.DeleteCartByIdAndUserId(
		c,
		repository.DeleteCartByIdAndUserIdParams{ID: param.ID, UserID: param.UserId},
	)
	if err != nil {
		err = fmt.Errorf(
			"failed deleting cartId=%s and userId=%s with error=%w",
			param.ID.String(),
			param.UserId.String(),
			err,
		)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	msg = fmt.Sprintf(
		"deleted cartId=%s and userId=%s from database",
		param.ID.String(),
		param.UserId.String(),
	)
	span.AddEvent(msg)
	logger.Info().Msg(msg)

	msg = fmt.Sprintf("deleting cartId=%s from cache", param.ID.String())
	logger.Info().Msg(msg)
	span.AddEvent(msg)
	err = s.cache.JSONDel(c, cacheKey, "$").Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart from cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	msg = fmt.Sprintf("deleted cartId=%s from cache", param.ID.String())
	span.AddEvent(msg)
	logger.Info().Msg(msg)

	return nil
}

func (s CartService) CheckoutCart(
	c context.Context,
	jwt *jwt.Token,
	param request.CheckoutCart,
) (response.Cart, error) {
	requestId := log.RequestIDFromContext(c)
	requestIdAttr := attribute.String(constants.KEY_REQUEST_ID, requestId)
	userIdAttr := attribute.String(constants.KEY_USER_ID, param.UserId.String())
	cartIdAttr := attribute.String(constants.KEY_CART_ID, param.CartId.String())
	orderIdAttr := attribute.String(constants.KEY_ORDER_ID, param.CartId.String())
	attrs := trace.WithAttributes(requestIdAttr, userIdAttr, cartIdAttr, orderIdAttr)

	c, span := otel.Tracer.Start(c, "CartService CheckoutCart", attrs)
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(constants.KEY_TAG, "CartService CheckoutCart").
		Str(constants.KEY_USER_ID, param.UserId.String()).
		Str(constants.KEY_CART_ID, param.CartId.String()).
		Str(constants.KEY_ORDER_ID, param.CartId.String()).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find-user").Logger()
	logger.Info().Msg("finding user by id")
	span.AddEvent("finding user by id")
	req, err := http.NewRequestWithContext(
		c,
		http.MethodGet,
		constants.URL_USER_SERVICE+"/"+param.UserId.String(),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("failed finding user by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	req.Header.Add(log.KEY_REQUEST_ID, requestId)
	resp, err := otelhttp.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed getting userId with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("user not found")
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("found user")
	logger.Info().Msg("found user")

	logger = logger.With().Str(constants.KEY_PROCESS, "find-cart").Logger()
	logger.Info().Msg("finding cart by id")
	span.AddEvent("finding cart by id")
	c = logger.WithContext(c)
	cart, err := s.FindCartById(c, request.FindCartById{ID: param.CartId, UserId: param.UserId})
	if err != nil {
		err = fmt.Errorf("failed finding cart by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("found cart by id")
	logger.Info().Msg("found cart by id")

	logger = logger.With().Str(constants.KEY_PROCESS, "mapping-cart").Logger()
	logger.Info().Msg("mapping cart to order")
	span.AddEvent("mapping cart to order")
	order := cart.Order()
	span.AddEvent("mapped cart to order")

	logger = logger.With().Str(constants.KEY_PROCESS, "create-checkout-request").Logger()
	logger.Info().Msg("creating checkout request to order-service")
	span.AddEvent("creating checkout request to order-service")
	orderJson, err := json.Marshal(order)
	if err != nil {
		err = fmt.Errorf("failed marshaling order with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	req, err = http.NewRequestWithContext(
		c,
		http.MethodPost,
		constants.URL_ORDER_SERVICE+"/"+"checkout",
		bytes.NewBuffer(orderJson),
	)
	if err != nil {
		err = fmt.Errorf("failed creating request to order-service with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("created checkout request to order-service")
	span.AddEvent("created checkout request to order-service")

	logger = logger.With().Str(constants.KEY_PROCESS, "sending-checkout-request").Logger()
	logger.Info().Msg("sending checkout request to order-service")
	span.AddEvent("sending checkout request to order-service")
	req.Header.Add("Authorization", "Bearer "+jwt.Raw)
	req.Header.Add(log.KEY_REQUEST_ID, requestId)
	resp, err = otelhttp.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed checkout cart to order-service with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	defer resp.Body.Close()
	span.AddEvent("sent checkout request to order-service")
	logger.Info().Msg("sent checkout request to order-service")

	logger = logger.With().Str(constants.KEY_PROCESS, "unmarshaling-checkout-response").Logger()
	logger.Info().Msg("unmarshaling checkout response")
	respBody := map[string]interface{}{}
	span.AddEvent("unmarshaling checkout response")
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	logger = logger.With().
		Dict("response", zerolog.Dict().
			Str(constants.KEY_REQUEST_ID, requestId).
			Any(constants.KEY_HEADER, checkoutResp.Header).
			Any(constants.KEY_BODY, checkoutRespBody)).
		Logger()
	span.AddEvent("unmarshaled checkout response")
	logger.Info().Msg("unmarshaled checkout response")
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf(
			"order service returned status code=%d with message=%s",
			resp.StatusCode,
			respBody["message"],
		)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	span.AddEvent("cart successfully checked out")
	logger.Info().Msg("cart successfully checked out")

	logger = logger.With().Str(constants.KEY_PROCESS, "remove-cart").Logger()
	logger.Info().Msg("removing cart")
	span.AddEvent("removing cart")
	c = logger.WithContext(c)
	err = s.RemoveCart(c, request.RemoveCart{ID: param.CartId, UserId: param.UserId})
	if err != nil {
		err = fmt.Errorf("failed removing cart with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	span.AddEvent("removed cart")
	logger.Info().Msg("removed cart")

	return cart, nil
}
