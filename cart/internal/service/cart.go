package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/request"
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
) (repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCart").
		Str(log.KeyProcess, "initializing transaction").
		Logger()

	logger.Info().Msg("initializing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msgf("failed initializing transaction with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger.Info().Msg("initialized transaction")
	defer func(lg zerolog.Logger) {
		logger = lg.With().Str(log.KeyProcess, "rollback transaction").Logger()
		logger.Info().Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			err = fmt.Errorf("failed rolling back transaction with error=%w", err)
			logger.Error().Err(err).Str(log.KeyProcess, "rollback transaction").Msg(err.Error())
			return
		}
		logger.Info().Msg("rolled back transaction")
	}(logger)

	logger = logger.With().Str(log.KeyProcess, "inserting cart request").Logger()
	logger.Info().Msg("inserting cart request")
	cart, err := s.queries.InsertCart(c, param.UserID)
	if err != nil {
		logger.Error().Err(err).Msgf("failed inserting cart request with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger = logger.With().Any(log.KeyCart, cart).Logger()
	logger.Info().Msg("inserted cart request")

	logger = logger.With().Str(log.KeyProcess, "inserting cart item request").Logger()
	logger.Info().Msg("inserting cart item")
	args := []repository.InsertCartItemsParams{}
	for i, item := range param.CartItems {
		price, err := decimal.NewFromString(item.Price)
		if err != nil {
			err = fmt.Errorf("failed inserting cart item i=%d with error=%w", i, err)
			logger.Error().Err(err).Msg(err.Error())
			return repository.Cart{}, err
		}
		args = append(
			args,
			repository.InsertCartItemsParams{
				CartID:    cart.ID,
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
	insertedCount, err := s.queries.InsertCartItems(c, args)
	if err != nil || insertedCount <= 0 {
		err = fmt.Errorf("failed inserting cart item with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Cart{}, err
	}
	logger.Info().Msg("inserted cart request")
	logger.Info().Msgf("inserted cart item with count=%d", insertedCount)

	return cart, nil
}

func (s *CartService) InsertCartItem(
	c context.Context,
	param request.InsertCartItem,
) (repository.CartItem, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCartItem").
		Str(log.KeyProcess, "validating price").
		Logger()

	logger.Info().Msg("validating price")
	price, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed validating price with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.CartItem{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "inserting cartItem").Logger()
	logger.Info().Msg("inserting cartItem")
	cart, err := s.queries.InsertCartItem(
		c,
		repository.InsertCartItemParams{
			CartID:    param.CartId,
			ProductID: param.ProductId,
			Quantity:  int32(param.Quantity),
			Price: pgtype.Numeric{
				Int:              price.BigInt(),
				Exp:              price.Exponent(),
				NaN:              false,
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
		},
	)
	if err != nil {
		err = fmt.Errorf("failed inserting cartItem with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.CartItem{}, err
	}
	logger.Info().Msg("inserted cartItem")

	return cart, nil
}

func (s *CartService) FindCartById(
	c context.Context,
	id uuid.UUID,
) (repository.FindCartByIdRow, error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartById").
		Logger()

	cacheKey := fmt.Sprintf("carts:%s", id.String())

	logger = logger.With().Str(log.KeyProcess, "finding cart").Logger()
	logger.Info().Msgf("finding cart id=%s in cache", id.String())
	cache, err := s.cache.Get(c, cacheKey).Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart id=%s in cache with error=%w", id.String(), err)
		logger.Info().Err(err).Msg(err.Error())

		logger.Info().Msgf("finding cart id=%s in database", id.String())
		cart, err := s.queries.FindCartById(c, id)
		if err != nil {
			err = fmt.Errorf(
				"failed finding cart id=%s in database with error=%w",
				id.String(),
				err,
			)
			logger.Error().Err(err).Msg(err.Error())
			return repository.FindCartByIdRow{}, err
		}
		logger.Info().Msgf("found cart id=%s in database", id.String())

		logger.Info().Msg("marshaling cart")
		json, err := json.Marshal(cart)
		if err != nil {
			err = fmt.Errorf("failed marshaling cart with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			return repository.FindCartByIdRow{}, err
		}
		logger.Info().Msg("marhsaled cart")

		logger.Info().Msgf("inserting cart id=%s to cache", id.String())
		err = s.cache.Set(c, "carts:%s", json, time.Hour*3).Err()
		if err != nil {
			err = fmt.Errorf("inserting cart id=%s to cache with error=%w", id.String(), err)
			logger.Error().Err(err).Msg(err.Error())
			return repository.FindCartByIdRow{}, err
		}
		logger.Info().Msgf("inserted cart id=%s to cache", id.String())

		return cart, nil
	}
	logger.Info().Msgf("found cart id=%s in cache", id.String())

	logger.Info().Msg("unmarshal cache")
	res := repository.FindCartByIdRow{}
	err = json.Unmarshal([]byte(cache), &res)
	if err != nil {
		err = fmt.Errorf("failed unmarshal cache with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.FindCartByIdRow{}, err
	}
	logger.Info().Msg("unmarshaled cache")

	return res, nil
}

func (s *CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) ([]repository.FindCartByUserIdRow, error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserId")
	defer span.End()

	cacheKey := fmt.Sprintf("carts:user_id:%s", userId.String())

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartByUserId").
		Str(log.KeyCacheKey, cacheKey).
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding cart").Logger()
	logger.Info().Msgf("finding cart with userId=%s in cache", userId.String())
	cache, err := s.cache.Get(c, cacheKey).Result()
	if err != nil {
		err = fmt.Errorf(
			"failed find cart with userId=%s in cache with error=%w",
			userId.String(),
			err,
		)
		logger.Info().Err(err).Msg(err.Error())

		logger.Info().Msgf("finding cart with userId=%s in database", userId.String())
		carts, err := s.queries.FindCartByUserId(c, userId)
		if err != nil {
			err = fmt.Errorf("cart with userId=%s not found with error=%w", userId.String(), err)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msgf("found cart with userId=%s", userId.String())

		logger.Info().Msg("marshaling carts")
		json, err := json.Marshal(carts)
		if err != nil {
			err = fmt.Errorf("failed marshal carts with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msg("marshaled carts")

		logger.Info().Msgf("inserting carts with userId=%s to cache", userId.String())
		err = s.cache.Set(c, cacheKey, json, time.Hour*3).Err()
		if err != nil {
			err = fmt.Errorf(
				"failed inserting carts with userId=%s to cache with error=%w",
				userId.String(),
				err,
			)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msgf("inserted carts with userId=%s to cache", userId.String())

		return carts, nil
	}
	logger.Info().Msgf("found cart with userId=%s in cache", userId.String())

	logger.Info().Msg("unmarshaling cache")
	carts := []repository.FindCartByUserIdRow{}
	err = json.Unmarshal([]byte(cache), &carts)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
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
		logger.Error().Err(err).Msg(err.Error())
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
		logger.Error().Err(err).Msg(err.Error())
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
		logger.Error().Err(err).Msg(err.Error())
		return err
	}
	logger.Info().Msg("deleted cartItem")

	return nil
}
