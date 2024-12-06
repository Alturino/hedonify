package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
}

func NewCartService(pool *pgxpool.Pool, queries *repository.Queries) CartService {
	return CartService{pool: pool, queries: queries}
}

func (s *CartService) InsertCart(
	c context.Context,
	param request.InsertCart,
) (repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCart").
		Str(log.KeyProcess, "initalizing transaction").
		Logger()

	logger.Info().Msg("initalizing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msgf("failed initalizing transaction with error=%s", err.Error())
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
) (repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartById").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "finding cart").
		Msg("finding cart")
	cart, err := s.queries.FindCartById(c, cartId)
	if err != nil {
		err = fmt.Errorf("cartId=%s not found", cartId.String())
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding cart").
			Msgf("failed finding cart by id=%s with error=%s", cartId, err.Error())
		return repository.Cart{}, err
	}
	logger.Info().
		Str(log.KeyProcess, "finding cart").
		Msgf("found cart by id=%s", cartId)

	return cart, nil
}

func (s *CartService) FindCartByUserId(
	c context.Context,
	userId uuid.UUID,
) ([]repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService FindCartByUserIdOrProductId")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService FindCartByUserIdOrProductId").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "finding cart").
		Msg("finding cart")
	cart, err := s.queries.FindCartByUserId(c, userId)
	if err != nil {
		err = fmt.Errorf("cart with userId=%s not found", userId.String())
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "finding cart").
			Msgf("failed finding cart by userId=%s with error=%s", userId, err.Error())
		return nil, err
	}
	logger.Info().
		Str(log.KeyProcess, "finding cart").
		Msgf("failed finding cart by userId=%s with error=%s", userId, err.Error())

	return cart, nil
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
