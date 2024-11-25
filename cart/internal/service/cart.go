package service

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
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
	param request.InsertCartRequest,
) (repository.Cart, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCart")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCart").
		Str(log.KeyProcess, "validating request body").
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())
	logger.Info().Msg("initialized validator")

	logger.Info().Msg("validating request body")
	cartBody := request.InsertCartRequest{}
	if err := validate.StructCtx(c, cartBody); err != nil {
		logger.Error().Err(err).Msgf("failed validating request body with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger.Info().Msg("validated request body")

	logger = logger.With().
		Str(log.KeyProcess, "initalizing transaction").
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("initalizing transaction")
	tx, err := s.pool.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		logger.Error().
			Err(err).
			Msgf("failed initalizing transaction with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger.Info().Msg("initialized transaction")
	defer func() {
		logger.Info().
			Str(log.KeyProcess, "rollback transaction").
			Msg("rolling back transaction")
		err = tx.Rollback(c)
		if err != nil {
			logger.Error().
				Err(err).
				Str(log.KeyProcess, "rollback transaction").
				Msgf("failed rolling back transaction with error=%s", err.Error())
			return
		}
		logger.Info().
			Str(log.KeyProcess, "rollback transaction").
			Msg("rolled back transaction")
	}()

	logger = logger.With().
		Str(log.KeyProcess, "inserting cart request").
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("inserting cart request")
	cart, err := s.queries.InsertCart(c, param.UserID)
	if err != nil {
		logger.Error().Err(err).Msgf("failed inserting cart request with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger = logger.With().
		Any(log.KeyCart, cart).
		Logger()
	c = logger.WithContext(c)
	logger.Info().Msg("inserted cart request")

	logger = logger.With().
		Str(log.KeyProcess, "inserting cart item request").
		Logger()
	c = logger.WithContext(c)

	logger.Info().Msg("inserting cart item")
	args := []repository.InsertCartItemParams{}
	for i, item := range param.CartItems {
		price, err := decimal.NewFromString(item.Price)
		if err != nil {
			logger.Error().
				Err(err).
				Msgf("failed inserting cart item i=%d with error=%s", i, err.Error())
			return repository.Cart{}, err
		}
		args = append(
			args,
			repository.InsertCartItemParams{
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
	insertedCount, err := s.queries.InsertCartItem(c, args)
	if err != nil || insertedCount <= 0 {
		err = fmt.Errorf("failed inserting cart item with error=%s", err.Error())
		logger.Error().Err(err).Msg(err.Error())
		return repository.Cart{}, err
	}
	logger.Info().Msg("inserted cart request")
	logger.Info().Msgf("inserted cart item with count=%d", insertedCount)

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
		err = fmt.Errorf("cart with id=%s not found", cartId)
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
