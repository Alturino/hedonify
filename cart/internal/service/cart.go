package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/cart/internal/common/cache"
	"github.com/Alturino/ecommerce/cart/internal/common/otel"
	"github.com/Alturino/ecommerce/cart/internal/repository"
	"github.com/Alturino/ecommerce/cart/request"
	"github.com/Alturino/ecommerce/cart/response"
	"github.com/Alturino/ecommerce/internal/common/constants"
	"github.com/Alturino/ecommerce/internal/common/errors"
	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
)

type CartService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
	cache   *redis.Client
	http    *http.Client
}

func NewCartService(
	pool *pgxpool.Pool,
	queries *repository.Queries,
	cache *redis.Client,
	http *http.Client,
) CartService {
	return CartService{pool: pool, queries: queries, cache: cache, http: http}
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
		Str(log.KeyTag, "CartService InsertCart").
		Logger()

	logger = logger.With().
		Str(log.KeyProcess, fmt.Sprintf("finding user by userId=%s in %s", userID.String(), constants.AppUserService)).
		Logger()
	logger.Info().Msgf("finding user by userId=%s", userID.String())
	resp, err := svc.http.Get(fmt.Sprintf("%s/users/%s", constants.AppUserService, userID.String()))
	if err != nil || resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("failed getting userId=%s with error=%w", userID.String(), err)
		commonErrors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msgf("found user by userId=%s", userID.String())

	logger = logger.With().Str(log.KeyProcess, "inserting cart to database").Logger()
	logger.Info().Msg("inserting cart to database")
	cart, err := svc.queries.InsertCart(c, userID)
	if err != nil {
		err = fmt.Errorf("failed inserting cart with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.Cart{}, err
	}
	logger.Info().Msg("inserted cart")

	insertedCount, err := svc.queries.InsertCartItems(c, []repository.InsertCartItemsParams{})

	return response.Cart{}, nil
}

func (s CartService) InsertCartItem(
	c context.Context,
	param request.InsertCartItem,
) (response.CartItem, error) {
	c, span := otel.Tracer.Start(c, "CartService InsertCartItem")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "CartService InsertCartItem").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating price").Logger()
	logger.Info().Msg("validating price")
	price, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed validating price with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "inserting cart item").Logger()
	logger.Info().Msg("inserting cart item")
	cartItem, err := s.queries.InsertCartItem(
		c,
		repository.InsertCartItemParams{
			ID:        uuid.New(),
			CartID:    param.CartId,
			ProductID: param.ProductId,
			Quantity:  int32(param.Quantity),
			Price:     pgtype.Numeric{Int: price.BigInt(), Exp: price.Exponent(), Valid: true},
		},
	)
	if err != nil {
		err = fmt.Errorf("failed inserting cart item with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}
	logger.Info().Msg("inserted cart item")

	err = s.cache.Del(c, fmt.Sprintf(cache.KEY_CARTS, param.CartId.String())).Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart item from cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return response.CartItem{}, err
	}

	return response.CartItem{
		ID:        cartItem.ID,
		CartID:    cartItem.CartID,
		ProductID: cartItem.ProductID,
		Quantity:  cartItem.Quantity,
		Price:     decimal.NewFromBigInt(cartItem.Price.Int, cartItem.Price.Exp),
		CreatedAt: cartItem.CreatedAt.Time,
		UpdatedAt: cartItem.UpdatedAt.Time,
	}, nil
}

func (s CartService) FindCartById(
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
	jsonCache, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_CARTS, id.String())).Result()
	if err != nil {
		err = fmt.Errorf("failed finding cart in cache with error=%w", err)
		errors.HandleError(err, logger, span)

		logger = logger.With().Str(log.KeyProcess, "finding cart in db").Logger()
		cart, err := s.queries.FindCartById(c, id)
		if err != nil {
			err = fmt.Errorf("failed finding cart in db with error=%w", err)
			errors.HandleError(err, logger, span)
			return response.Cart{}, err
		}
		logger = logger.With().Any(log.KeyCart, cart).Logger()
		logger.Info().Msg("found cart in db")

		logger = logger.With().Str(log.KeyProcess, "marshaling cart").Logger()
		logger.Info().Msg("marshaling cart")
		jsonString, err := json.Marshal(cart)
		if err != nil {
			err = fmt.Errorf("failed marshaling cart with error=%w", err)
			errors.HandleError(err, logger, span)
			return response.Cart{}, err
		}
		logger.Info().Msg("marshaled cart")

		logger = logger.With().Str(log.KeyProcess, "inserting cart in cache").Logger()
		err = s.cache.Set(c, fmt.Sprintf(cache.KEY_CARTS, id.String()), jsonString, time.Hour*1).
			Err()
		if err != nil {
			err = fmt.Errorf("failed inserting cart in cache with error=%w", err)
			errors.HandleError(err, logger, span)
			return response.Cart{}, err
		}
		logger.Info().Msg("inserted cart in cache")

		logger = logger.With().Str(log.KeyProcess, "unmarshaling cart items").Logger()
		logger.Info().Msg("unmarshaling cart items")
		cartItems := []response.CartItem{}
		err = json.Unmarshal(cart.CartItems, &cartItems)
		if err != nil {
			err = fmt.Errorf("failed unmarshaling cart items with error=%w", err)
			errors.HandleError(err, logger, span)
			return response.Cart{}, err
		}
		logger.Info().Msg("unmarshaled cart items")

		return response.Cart{
			ID:        cart.ID,
			UserID:    cart.UserID,
			CartItems: cartItems,
			CreatedAt: cart.CreatedAt.Time,
			UpdatedAt: cart.UpdatedAt.Time,
		}, nil
	}
	logger.Info().Msg("found cart in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonCache), &cart)
	if err != nil {
		err = fmt.Errorf("failed unmarshaling cache with error=%w", err)
		errors.HandleError(err, logger, span)
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
		Str(log.KeyTag, "CartService FindCartByUserId").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding cart in cache").Logger()
	logger.Info().Msg("finding cart in cache")
	jsonString, err := s.cache.Get(c, fmt.Sprintf(cache.KEY_CARTS_USER_ID, userId.String())).
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
		err = s.cache.Set(c, fmt.Sprintf(cache.KEY_CARTS_USER_ID, userId.String()), json, time.Hour*1).
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

func (s CartService) RemoveCartItem(c context.Context, param request.RemoveCartItem) error {
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

	logger = logger.With().Str(log.KeyProcess, "deleting cart from cache").Logger()
	logger.Info().Msg("deleting cart from cache")
	err = s.cache.Del(c, fmt.Sprintf(cache.KEY_CARTS, param.CartId.String())).Err()
	if err != nil {
		err = fmt.Errorf("failed deleting cart from cache with error=%w", err)
		errors.HandleError(err, logger, span)
		return err
	}
	logger.Info().Msg("deleted cart from cache")

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
