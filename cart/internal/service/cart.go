package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
) *CartService {
	return &CartService{pool: pool, queries: queries, cache: cache}
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
		Ctx(c).
		Str(constants.KEY_TAG, "CartService InsertCart").
		Int(constants.KEY_CART_ITEMS, len(param.CartItems)).
		Str(constants.KEY_USER_ID, userID.String()).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find user").Logger()
	logger.Trace().Msg("find user by id in user service")
	findUserReq, err := http.NewRequestWithContext(
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
	findUserReq.Header.Add(inHttp.KEY_HEADER_REQUEST_ID, requestId)
	findUserResp, err := otelhttp.DefaultClient.Do(findUserReq)
	if err != nil {
		err = fmt.Errorf("failed getting userId=%s with error=%w", userID.String(), err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	defer findUserResp.Body.Close()
	if findUserResp.StatusCode != http.StatusOK {
		err = fmt.Errorf("userId=%s not found", userID.String())
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("found user")
	logger.Info().Msg("found user")

	logger = logger.With().Str(constants.KEY_PROCESS, "initializing transaction").Logger()
	logger.Trace().Msg("initializing transaction")
	span.AddEvent("initializing transaction")
	tx, err := svc.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		err = fmt.Errorf("failed initializing transaction with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	defer func() {
		lg := logger.With().Str(constants.KEY_PROCESS, "rolling back transaction").Logger()
		lg.Trace().Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			if errors.Is(err, pgx.ErrTxClosed) {
				lg.Debug().Err(err).Msg(err.Error())
				return
			}
			lg.Error().Err(err).Msg(err.Error())
			inOtel.RecordError(err, span)
			return
		}
		lg.Info().Msg("rolled back transaction")
	}()
	span.AddEvent("initialized transaction")
	logger.Trace().Msg("initialized transaction")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart to database").Logger()
	logger.Trace().Msg("inserting cart to database")
	span.AddEvent("inserting cart to database")
	cart, err := svc.queries.WithTx(tx).InsertCart(c, userID)
	if err != nil {
		err = fmt.Errorf("failed inserting cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("inserted cart to database")
	logger = logger.With().Any(constants.KEY_CART, cart).Logger()
	logger.Trace().Msg("inserted cart to database")

	span.AddEvent("merging cart items")
	logger.Trace().Msg("merging cart items")
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
	logger.Trace().Msg("merged cart items")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart items to database").Logger()
	logger.Trace().Msg("inserting cart items to database")
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
	logger.Trace().Msg("inserting cart items to database")
	insertedCount, err := svc.queries.WithTx(tx).InsertCartItems(c, args)
	if err != nil {
		err = fmt.Errorf("failed inserting cartItems to database with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("inserted cart items to database")
	logger = logger.With().Int64(constants.KEY_CART_ITEMS_COUNT, insertedCount).Logger()
	logger.Info().Msg("inserted cart items to database")

	logger = logger.With().
		Str(constants.KEY_PROCESS, "finding cart by id").
		Str(constants.KEY_CART_ID, cart.ID.String()).
		Logger()
	logger.Trace().Msg("finding cart by id")
	span.AddEvent("finding cart by id")
	cartDb, err := svc.queries.WithTx(tx).
		FindCartById(c, repository.FindCartByIdParams{ID: userID, ID_2: cart.ID})
	if err != nil {
		err = fmt.Errorf("failed finding cart by id with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("found cart by id")
	logger = logger.With().RawJSON(constants.KEY_CART_ITEMS, cartDb.CartItems).Logger()
	logger.Trace().Msg("found cart by id")

	logger = logger.With().Str(constants.KEY_PROCESS, "mapping cart").Logger()
	logger.Trace().Msg("mapping cart to cart response")
	span.AddEvent("mapping cart to cart response")
	cartResponse, err := cartDb.Response()
	if err != nil {
		err = fmt.Errorf("failed mapping cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("mapped cart db to cart response")
	logger = logger.With().Any(constants.KEY_CART_RESPONSE, cartResponse).Logger()
	logger.Trace().Msg("mapped cart db to cart response")

	cacheKey := fmt.Sprintf(cache.KEY_CARTS, cart.ID.String())
	logger = logger.With().
		Str(constants.KEY_PROCESS, "inserting cart to cache").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()
	logger.Trace().Msg("inserting cart to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", cartResponse).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting cart to cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	logger.Trace().Msg("inserted cart to cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "committing transaction").Logger()
	logger.Trace().Msg("committing transaction")
	span.AddEvent("committing transaction")
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
	span.AddEvent("committed transaction")
	logger.Trace().Msg("committed transaction")

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
		Ctx(c).
		Str(constants.KEY_TAG, "CartService FindCartById").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in cache").Logger()
	logger.Trace().Msg("finding cart in cache")
	span.AddEvent("finding cart in cache")
	jsonCache, err := s.cache.JSONGet(c, cacheKey).Result()
	if err != nil || err == redis.Nil || errors.Is(err, redis.Nil) || jsonCache == "" {
		err = fmt.Errorf("failed finding cart in cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Debug().Err(err).Msg(err.Error())

		logger = logger.With().Str(constants.KEY_PROCESS, "finding cart in db").Logger()
		logger.Trace().Msg("finding cart in db")
		span.AddEvent("finding cart in db")
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
		span.AddEvent("found cart in db")
		logger = logger.With().Any(constants.KEY_CART, cart).Logger()
		logger.Debug().Msg("found cart in db")

		logger = logger.With().Str(constants.KEY_PROCESS, "inserting cart in cache").Logger()
		logger.Trace().Msg("inserting cart in cache")
		span.AddEvent("inserting cart in cache")
		err = s.cache.JSONSet(c, cacheKey, "$", cart).Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cart in cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		span.AddEvent("inserted cart in cache")
		logger.Debug().Msg("inserted cart in cache")

		logger = logger.With().Str(constants.KEY_PROCESS, "mapping cart").Logger()
		logger.Trace().Msg("mapping cart")
		span.AddEvent("mapping cart")
		cartResponse, err := cart.Response()
		if err != nil {
			err = fmt.Errorf("failed mapping cart with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		span.AddEvent("mapped cart")
		logger = logger.With().Any(constants.KEY_CART_RESPONSE, cartResponse).Logger()
		logger.Debug().Msg("mapped cart")

		logger.Info().Msg("found cart in db and inserted in cache")
		return cart.Response()
	}
	span.AddEvent("found cart in cache")
	logger = logger.With().RawJSON(constants.KEY_JSON_CACHE, []byte(jsonCache)).Logger()
	logger.Debug().Msg("found cart in cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "unmarshaling cache").Logger()
	logger.Trace().Msg("unmarshaling cache")
	span.AddEvent("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonCache), &cart)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("unmarshaling cache")
	logger.Debug().Msg("unmarshaled cache")

	logger.Info().Msg("found cart in cache")
	return cart, nil
}

func (s CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) (carts []repository.FindCartByUserIdRow, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserId")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_CARTS_BY_USER_ID, userId.String())
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Str(constants.KEY_TAG, "CartService FindCartByUserId").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "finding carts in cache").Logger()
	logger.Trace().Msg("finding carts in cache")
	jsonString, err := s.cache.JSONGet(c, cacheKey).Result()
	if err != nil || err == redis.Nil || errors.Is(err, redis.Nil) || jsonString == "" {
		err = fmt.Errorf("failed finding carts in cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())

		logger = logger.With().Str(constants.KEY_PROCESS, "finding carts in db").Logger()
		logger.Trace().Msg("finding carts in db")
		span.AddEvent("finding carts in db")
		carts, err := s.queries.FindCartByUserId(c, userId)
		if err != nil {
			err = fmt.Errorf("failed finding carts in db with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		span.AddEvent("found carts in db")
		logger.Debug().Msg("found carts in db")

		logger = logger.With().Str(constants.KEY_PROCESS, "inserting cache").Logger()
		logger.Trace().Msg("inserting carts to cache")
		span.AddEvent("inserting carts to cache")
		err = s.cache.JSONSet(c, cacheKey, "$", carts).Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cache with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		span.AddEvent("inserted carts to cache")
		logger.Debug().Msg("inserted carts to cache")

		logger.Info().Msg("found carts in database and inserted to cache")
		return carts, err
	}
	span.AddEvent("found carts in cache")
	logger = logger.With().Str(constants.KEY_JSON_CACHE, jsonString).Logger()

	logger.Info().Msg("found carts in cache")
	return carts, nil
}

func (s CartService) RemoveCartItem(c context.Context, param request.RemoveCartItem) error {
	c, span := otel.Tracer.Start(c, "CartService RemoveCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartService RemoveCartItem").
		Str(constants.KEY_CART_ID, param.CartId.String()).
		Str(constants.KEY_CART_ITEM_ID, param.ID.String()).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find cart").Logger()
	logger.Trace().Msg("finding cartId")
	span.AddEvent("finding cartId")
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
	span.AddEvent("found cartId")
	logger.Debug().Msg("found cartId")

	logger = logger.With().Str(constants.KEY_PROCESS, "finding cartItemId").Logger()
	logger.Trace().Msg("finding cartItemId")
	span.AddEvent("finding cartItemId")
	_, err = s.queries.FindCartItemById(c, param.ID)
	if err != nil {
		err = fmt.Errorf("failed finding cart item with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("found cartItemId")
	logger.Debug().Msg("found cartItemId")

	logger = logger.With().Str(constants.KEY_PROCESS, "deleting cart item from cache").Logger()
	logger.Trace().Msg("deleting cart item from cache")
	err = s.cache.Del(c, cache.KEY_CARTS+param.CartId.String()).Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart from cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("deleted cart item from cache")
	logger.Info().Msg("deleted cart item from cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "deleting cart item from database").Logger()
	logger.Trace().Msg("deleting cart item from database")
	_, err = s.queries.DeleteCartItemFromCartsById(
		c,
		repository.DeleteCartItemFromCartsByIdParams{ID: param.ID, CartID: param.CartId},
	)
	if err != nil {
		err = fmt.Errorf("failed deleting cart item with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("deleted cart item from database")
	logger.Info().Msg("deleted cart item from database")

	logger.Info().Msg("deleted cart item")
	return nil
}

func (s CartService) RemoveCart(c context.Context, param request.RemoveCart) error {
	c, span := otel.Tracer.Start(c, "CartService RemoveCart")
	defer span.End()

	cacheKey := fmt.Sprintf(cache.KEY_CARTS, param.ID.String())
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "CartService RemoveCart").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find cart").Logger()
	logger.Trace().Msg("finding cart")
	span.AddEvent("finding cart")
	_, err := s.FindCartById(c, request.FindCartById(param))
	if err != nil {
		err = fmt.Errorf("failed finding cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("found cart")
	logger.Info().Msg("found cart")

	logger = logger.With().Str(constants.KEY_PROCESS, "delete cart from database").Logger()
	span.AddEvent("deleting cart from database")
	logger.Trace().Msg("deleting cart from database")
	_, err = s.queries.DeleteCartByIdAndUserId(
		c,
		repository.DeleteCartByIdAndUserIdParams{ID: param.ID, UserID: param.UserId},
	)
	if err != nil {
		err = fmt.Errorf("failed deleting cart with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("deleted cart from database")
	logger.Info().Msg("deleted cart from database")

	logger = logger.With().Str(constants.KEY_PROCESS, "delete cart from cache").Logger()
	logger.Trace().Msg("deleting cart from cache")
	span.AddEvent("deleting cart from cache")
	err = s.cache.JSONDel(c, cacheKey, "$").Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart from cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	span.AddEvent("deleted cart from cache")
	logger.Info().Msg("deleted cart from cache")

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
		Ctx(c).
		Str(constants.KEY_TAG, "CartService CheckoutCart").
		Str(constants.KEY_USER_ID, param.UserId.String()).
		Str(constants.KEY_CART_ID, param.CartId.String()).
		Str(constants.KEY_ORDER_ID, param.CartId.String()).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find user").Logger()
	logger.Trace().Msg("creating request to user service")
	span.AddEvent("creating request to user service")
	findUserReq, err := http.NewRequestWithContext(
		c,
		http.MethodGet,
		constants.URL_USER_SERVICE+"/"+param.UserId.String(),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("failed creating request to user service with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	findUserReq.Header.Add(inHttp.KEY_HEADER_REQUEST_ID, requestId)
	logger.Debug().Msg("sending request find user to user service")
	findUserResp, err := otelhttp.DefaultClient.Do(findUserReq)
	if err != nil {
		err = fmt.Errorf("failed sending request to user service with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	defer findUserResp.Body.Close()
	if findUserResp.StatusCode != http.StatusOK {
		err = errors.New("user not found in user service")
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	span.AddEvent("found user")
	logger.Info().Msg("found user in user service")

	logger = logger.With().Str(constants.KEY_PROCESS, "find cart").Logger()
	logger.Trace().Msg("finding cart by id")
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
	logger.Trace().Msg("mapping cart to order")
	span.AddEvent("mapping cart to order")
	order := cart.Order()
	span.AddEvent("mapped cart to order")
	logger.Debug().Msg("mapped cart to order")

	logger = logger.With().Str(constants.KEY_PROCESS, "checkout cart").Logger()
	logger.Trace().Msg("creating checkout request to order service")
	span.AddEvent("creating checkout request to order service")
	orderJson, err := json.Marshal(order)
	if err != nil {
		err = fmt.Errorf("failed marshaling order with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Cart{}, err
	}
	checkoutReq, err := http.NewRequestWithContext(
		c,
		http.MethodPost,
		constants.URL_ORDER_SERVICE+"/"+"checkout",
		bytes.NewBuffer(orderJson),
	)
	if err != nil {
		err = fmt.Errorf("failed creating request to order service with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	checkoutReq.Header.Add("Authorization", "Bearer "+jwt.Raw)
	checkoutReq.Header.Add(inHttp.KEY_HEADER_REQUEST_ID, requestId)
	span.AddEvent("created checkout request to order service")
	logger.Debug().Msg("created checkout request to order service")

	logger.Trace().Msg("sending checkout request to order service")
	span.AddEvent("sending checkout request to order service")
	checkoutResp, err := otelhttp.DefaultClient.Do(checkoutReq)
	if err != nil {
		err = fmt.Errorf("failed sending checkout request to order service with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	span.AddEvent("sent checkout request to order service")
	logger.Info().Msg("sent checkout request to order service")

	logger.Trace().Msg("unmarshaling checkout response")
	span.AddEvent("unmarshaling checkout response")
	checkoutRespBody := map[string]interface{}{}
	err = json.NewDecoder(checkoutResp.Body).Decode(&checkoutRespBody)
	if err != nil {
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	logger = logger.With().
		Dict("checkout_response", zerolog.Dict().
			Str(constants.KEY_REQUEST_ID, requestId).
			Any(constants.KEY_HEADER, checkoutResp.Header).
			Any(constants.KEY_BODY, checkoutRespBody)).
		Logger()
	span.AddEvent("unmarshaled checkout response")
	logger.Debug().Msg("unmarshaled checkout response")

	logger.Trace().Msg("checking checkout cart response status")
	span.AddEvent("checking checkout cart response status")
	if checkoutResp.StatusCode != http.StatusCreated {
		err = fmt.Errorf(
			"order service returned status code=%d with message=%s",
			checkoutResp.StatusCode,
			checkoutRespBody["message"],
		)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	span.AddEvent("cart successfully checked out to order service")
	logger.Info().Msg("cart successfully checked out to order service")

	logger = logger.With().Str(constants.KEY_PROCESS, "remove-cart").Logger()
	logger.Trace().Msg("removing cart")
	span.AddEvent("removing cart")
	c = logger.WithContext(c)
	err = s.RemoveCart(c, request.RemoveCart{ID: param.CartId, UserId: param.UserId})
	if err != nil {
		err = fmt.Errorf("failed removing cart with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		inOtel.RecordError(err, span)
		return response.Cart{}, err
	}
	span.AddEvent("removed cart after checkout to order service")
	logger.Info().Msg("removed cart after checkout to order service")

	return cart, nil
}
