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
	"github.com/Alturino/ecommerce/cart/response"
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
	cartId uuid.UUID,
) (result response.Cart, err error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartById").
		Logger()

	cacheKey := fmt.Sprintf("carts:%s", cartId.String())

	logger = logger.With().Str(log.KeyProcess, "finding cart").Logger()
	logger.Info().Msgf("finding cart id=%s in cache", cartId.String())
	cache, err := s.cache.JSONGet(c, cacheKey, "$").Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart id=%s in cache with error=%w", cartId.String(), err)
		logger.Info().Err(err).Msg(err.Error())

		logger.Info().Msg("initializing transaction")
		tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
		if err != nil {
			err = fmt.Errorf("failed begin transaction with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		logger.Info().Msg("initialized transaction")
		defer func(lg zerolog.Logger) {
			lg = lg.With().Str(log.KeyProcess, "rolling back transaction").Logger()

			lg.Info().Msg("rolling back transaction")
			err = tx.Rollback(c)
			if err != nil {
				err = fmt.Errorf("failed rolling back transaction with error=%w", err)
				lg.Info().Err(err).Msg(err.Error())
				return
			}
			lg.Info().Msg("rolled back transaction")
		}(logger)

		logger.Info().Msgf("finding cart id=%s in database", cartId.String())
		cart, err := s.queries.WithTx(tx).FindCartById(c, cartId)
		if err != nil {
			err = fmt.Errorf("cart id=%s not found with error=%w", cartId.String(), err)
			logger.Error().Err(err).Str(log.KeyProcess, "finding cart").Msg(err.Error())
			return result, err
		}
		logger = logger.With().Any(log.KeyCart, cart).Logger()
		logger.Info().Msgf("found cart id=%s in database", cartId.String())

		logger.Info().Msgf("finding cartItems with cartId=%s in database", cartId.String())
		cartItems, err := s.queries.WithTx(tx).FindCartItemByCartId(c, cartId)
		if err != nil {
			err = fmt.Errorf(
				"cartItems with cartId=%s not found with error=%w",
				cartId.String(),
				err,
			)
			logger.Error().Err(err).Str(log.KeyProcess, "finding cart").Msg(err.Error())
			return result, err
		}
		logger = logger.With().Any(log.KeyCartItems, cartItems).Logger()
		resCartItems := make([]response.CartItem, len(cartItems))
		for _, item := range cartItems {
			resCartItems = append(resCartItems, response.CartItem{
				ID:        item.ID,
				CartID:    item.CartID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				Price:     decimal.NewFromBigInt(item.Price.Int, item.Price.Exp).String(),
				CreatedAt: item.CreatedAt.Time,
				UpdatedAt: item.UpdatedAt.Time,
			})
		}
		logger.Info().Msgf("found cartItems cartId=%s in database", cartId.String())

		result = response.Cart{
			ID:        cart.ID,
			UserID:    cart.UserID,
			CartItems: resCartItems,
			CreatedAt: cart.CreatedAt.Time,
			UpdatedAt: cart.UpdatedAt.Time,
		}

		logger.Info().Msgf("inserting cart id=%s to cache", cartId.String())
		err = s.cache.Set(c, cacheKey, result, time.Hour*3).Err()
		if err != nil {
			err = fmt.Errorf(
				"failed inserting cart id=%s to cache with error=%w",
				cartId.String(),
				err,
			)
			logger.Error().Err(err).Msg(err.Error())
			return response.Cart{}, err
		}
		return result, nil
	}
	logger.Info().Msgf("found cart id=%s in cache", cartId.String())

	logger.Info().Msg("unmarshal cache")
	err = json.Unmarshal([]byte(cache), &result)
	if err != nil {
		err = fmt.Errorf("failed unmarshal cache with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return
	}
	return result, nil
}

func (s *CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) ([]repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserId")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartByUserId").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding cart").Logger()
	logger.Info().Msgf("finding cart with userId=%s in cache", userId.String())
	cacheKey := fmt.Sprintf("carts:user_id:%s", userId.String())
	cache, err := s.cache.Get(c, cacheKey).Result()
	if err != nil {
		err = fmt.Errorf(
			"failed find cart with userId=%s in cache with error=%w",
			userId.String(),
			err,
		)
		logger.Info().Err(err).Msg(err.Error())

		carts, err := s.queries.FindCartByUserId(c, userId)
		if err != nil {
			err = fmt.Errorf("cart with userId=%s not found with error=%w", userId.String(), err)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		logger.Info().Msgf("found cart with userId=%s", userId.String())

		json, err := json.Marshal(carts)
		if err != nil {
			err = fmt.Errorf("failed marshal carts with error=%w", err)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}
		err = s.cache.Set(c, cacheKey, json, time.Hour*3).Err()
		if err != nil {
			err = fmt.Errorf(
				"failed inserting carts with userId=%s with error=%w",
				userId.String(),
				err,
			)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}

		return carts, nil
	}
	logger.Info().Msgf("found cart with userId=%s in cache", userId.String())

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	result := []repository.Cart{}
	err = json.Unmarshal([]byte(cache), &result)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("unmarshaled cache")

	return result, nil
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
