package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/cart/internal/common/cache"
	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/request"
	"github.com/Alturino/ecommerce/cart/response"
	"github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
)

type CartService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
	cache   *redis.Client
}

func NewCartService(pool *pgxpool.Pool, queries *repository.Queries) CartService {
	return CartService{pool: pool, queries: queries}
}

func (s *CartService) InsertCart(
	c context.Context,
	param request.Cart,
) (cart response.Cart, err error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCart").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "mapping to response cart").Logger()
	logger.Info().Msg("mapping to response cart")
	cart = param.Cart()
	logger.Info().Msg("mapped to response cart")

	logger = logger.With().Str(log.KeyProcess, "marshaling cart").Logger()
	logger.Info().Msg("marshaling cart")
	json, err := json.Marshal(cart)
	if err != nil {
		err = fmt.Errorf("failed marshaling cart with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("marshaled cart")

	logger = logger.With().Str(log.KeyProcess, "inserting cart to cache").Logger()
	logger.Info().Msg("inserting cart to cache")
	err = s.cache.Set(c, fmt.Sprintf(cache.KEY_CARTS, cart.ID.String()), json, time.Hour*3).Err()
	if err != nil {
		err = fmt.Errorf("failed inserting cart to cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("inserted cart to cache")

	return cart, nil
}

func (s *CartService) InsertCartItem(
	c context.Context,
	param request.InsertCartItem,
) (response.CartItem, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCartItem").
		Str(log.KeyProcess, "validating price").
		Logger()

	logger.Info().Msg("validating price")
	_, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed validating price with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	cacheJson, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_CARTS, param.CartId.String())).Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart in cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cart").Logger()
	logger.Info().Msg("unmarshaling cart")
	cart := response.Cart{}
	err = json.Unmarshal([]byte(cacheJson), &cart)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cart with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("unmarshaled cart")

	logger = logger.With().Str(log.KeyProcess, "inserting cartItem to cache").Logger()

	logger = logger.With().Str(log.KeyProcess, "validating price").Logger()
	logger.Info().Msg("validating price")
	price, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed validating price with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "inserting cartItem to cart").Logger()
	logger.Info().Msg("inserting cartItem to cart")
	cartItem := response.CartItem{
		ID:        uuid.New(),
		CartID:    param.CartId,
		ProductID: param.ProductId,
		Quantity:  param.Quantity,
		Price:     price,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	cart.CartItems = append(cart.CartItems, cartItem)
	logger.Info().Msg("inserted cartItem to cart")

	logger.Info().Msg("marshaling cart")
	marshaled, err := json.Marshal(cart)
	if err != nil {
		err = fmt.Errorf("failed marshaling cart with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("marshaled cart")

	logger.Info().Msg("inserting cart to cache")
	err = s.cache.Set(c, fmt.Sprintf(cache.KEY_CARTS, param.CartId.String()), marshaled, time.Hour*3).
		Err()
	if err != nil {
		err = fmt.Errorf("failed inserting cartItem to cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("inserted cartItem to cache")

	return cartItem, nil
}

func (s *CartService) FindCartById(
	c context.Context,
	id uuid.UUID,
) (cart response.Cart, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartById").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	cache, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_CARTS, id.String())).Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart in cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(cache), &cart)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("unmarshaled cache")

	return cart, nil
}

func (s *CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) (carts []repository.FindCartByUserIdRow, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserId")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartByUserId").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	jsonString, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_PRODUCTS_USER_ID, userId.String())).
		Result()
	if err != nil {
		err = fmt.Errorf("failed finding cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())

		logger = logger.With().Str(log.KeyProcess, "finding cart in db").Logger()
		logger.Info().Msg("finding cart in db")
		carts, err := s.queries.FindCartByUserId(c, userId)
		if err != nil {
			err = fmt.Errorf("failed finding cart in db with error=%w", err)
			errors.HandleError(err, logger, span)
			return nil, err
		}
		logger.Info().Msg("found cart in db")

		logger = logger.With().Str(log.KeyProcess, "marshaling cache").Logger()
		logger.Info().Msg("marshaling cache")
		json, err := json.Marshal(carts)
		if err != nil {
			err = fmt.Errorf("failed marshaling cache with error=%w", err)
			errors.HandleError(err, logger, span)
			return nil, err
		}
		logger.Info().Msg("marshaled cache")

		logger = logger.With().Str(log.KeyProcess, "inserting cache").Logger()
		logger.Info().Msg("inserting cache")
		err = s.cache.Set(c, fmt.Sprintf(cache.KEY_PRODUCTS_USER_ID, userId.String()), json, time.Hour*3).
			Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cache with error=%w", err)
			errors.HandleError(err, logger, span)
			return nil, err
		}
		logger.Info().Msg("inserted cache")

		return nil, err
	}
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonString), &carts)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return nil, err
	}
	logger.Info().Msg("unmarshaled cache")

	return carts, nil
}

func (s *CartService) RemoveCartItem(c context.Context, param request.RemoveCartItem) error {
	c, span := otel.Tracer.Start(c, "CartService RemoveCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService RemoveCartItem").
		Str(log.KeyProcess, "finding cartId").
		Logger()

	logger.Info().Msg("finding cartId")
	_, err := s.queries.FindCartById(c, param.CartId)
	if err != nil {
		err = fmt.Errorf("failed finding cartId=%s with error=%w", param.ID.String(), err)
		errors.HandleError(err, logger, span)
		return err
	}
	logger.Info().Msg("found cartId")

	logger = logger.With().Str(log.KeyProcess, "finding cartItemId").Logger()
	logger.Info().Msg("finding cartItemId")
	_, err = s.queries.FindCartItemById(c, param.ID)
	if err != nil {
		err = fmt.Errorf(
			"failed finding cartItemId=%s in cartId=%s with error=%w",
			param.ID.String(),
			param.CartId.String(),
			err,
		)
		errors.HandleError(err, logger, span)
		return err
	}
	logger.Info().Msg("found cartItemId")

	logger = logger.With().Str(log.KeyProcess, "deleting cartItem").Logger()
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
		errors.HandleError(err, logger, span)
		return err
	}
	logger.Info().Msg("deleted cartItem")

	return nil
}
