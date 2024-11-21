package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/request"
	"github.com/Alturino/ecommerce/internal/log"
)

type CartService struct {
	queries *repository.Queries
}

func NewCartService(queries *repository.Queries) CartService {
	return CartService{queries: queries}
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
		Logger()

	logger.Info().
		Str(log.KeyProcess, "validating price is actually a number").
		Msg("validating price is actually a number")
	rat, ok := new(big.Rat).SetString(param.Price)
	if !ok {
		err := errors.New("price is not actually a number")
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "validating price is actually a number").
			Msg(err.Error())
		return repository.Cart{}, err
	}
	logger.Info().
		Str(log.KeyProcess, "validating price is actually a number").
		Msg("validated price is actually a number")

	logger.Info().
		Str(log.KeyProcess, "inserting cart request").
		Msg("inserting cart")
	cart, err := s.queries.InsertCart(
		c,
		repository.InsertCartParams{
			UserID:     uuid.MustParse(param.UserID),
			TotalPrice: rat.String(),
		},
	)
	r := recover()
	if err != nil || r != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "inserting cart request").
			Msgf("failed inserting cart with error=%s", err.Error())
		return repository.Cart{}, err
	}
	logger.Info().
		Str(log.KeyProcess, "inserting cart request").
		Msg("inserted cart")

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
