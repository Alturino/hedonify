package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/product/internal/common/cache"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/pkg/request"
	"github.com/Alturino/ecommerce/product/pkg/response"
)

type ProductService struct {
	pool    *pgxpool.Pool
	queries *repository.Queries
	cache   *redis.Client
}

func NewProductService(
	pool *pgxpool.Pool,
	queries *repository.Queries,
	cache *redis.Client,
) ProductService {
	return ProductService{pool: pool, queries: queries, cache: cache}
}

func (svc ProductService) InsertProduct(
	c context.Context,
	param request.Product,
) (response.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService InsertProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding product in database").Logger()
	logger.Info().Msg("finding product in database")
	span.AddEvent("finding product in database")
	product, err := svc.queries.FindProductByName(c, param.Name)
	if err == nil {
		err = fmt.Errorf("product is already exist with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Info().Err(err).Msg(err.Error())
		return product.Response(), err
	}
	span.AddEvent("product is not exist in database")
	logger.Info().Msg("product is not exist in database")

	logger = logger.With().Str(log.KeyProcess, "inserting product to database").Logger()
	logger.Info().Msg("inserting product to database")
	product, err = svc.queries.InsertProduct(
		c,
		repository.InsertProductParams{
			Name: param.Name,
			Price: pgtype.Numeric{
				Exp:              param.Price.Exponent(),
				InfinityModifier: pgtype.Finite,
				Int:              param.Price.Coefficient(),
				NaN:              false,
				Valid:            true,
			},
			Quantity: int32(param.Quantity),
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to insert product with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, err
	}
	logger = logger.With().Any(log.KeyProduct, product).Logger()
	logger.Info().Msg("inserted product")

	cacheKey := cache.KEY_PRODUCTS + product.ID.String()
	logger = logger.With().
		Str(log.KeyProcess, "inserting product to cache").
		Str(log.KeyCacheKey, cacheKey).
		Logger()
	logger.Info().Msg("inserting product to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", product).Err()
	if err != nil {
		err = fmt.Errorf("failed to inserting product to cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, nil
	}
	logger.Info().Msg("inserted product to cache")

	return product.Response(), nil
}

func (svc ProductService) GetProducts(
	c context.Context,
) ([]repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).With().Str(log.KeyTag, "ProductService FindProducts").Logger()

	logger = logger.With().Str(log.KeyProcess, "find.product.database").Logger()
	logger.Info().Msg("finding products in database")
	span.AddEvent("finding products in database")
	products, err := svc.queries.FindProducts(c)
	if err != nil {
		err = fmt.Errorf("failed to get products from database with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Info().Err(err).Msg(err.Error())
		return nil, err
	}
	logger = logger.With().Any(log.KeyProducts, products).Logger()
	span.AddEvent("found products in database")
	logger.Info().Msg("found products in database")

	return products, err
}

func (svc ProductService) FindProductById(
	c context.Context,
	id uuid.UUID,
) (response.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProductById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService FindProductById").
		Logger()

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger = logger.With().
		Str(log.KeyProcess, "finding product in cache").
		Str(log.KeyCacheKey, cacheKey).
		Logger()
	logger.Info().Msg("finding product in cache")
	jsonCache, err := svc.cache.JSONGet(c, cacheKey).Result()
	if err != nil {
		err = fmt.Errorf("failed to get product from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())
		logger = logger.With().Str(log.KeyProcess, "finding product in database").Logger()
		product, err := svc.queries.FindProductById(c, id)
		if err != nil {
			err = fmt.Errorf("failed to find product in database with error=%w", err)
			commonErrors.HandleError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Product{}, err
		}
		logger = logger.With().Any(log.KeyProduct, product).Logger()
		logger.Info().Msg("found product in database")
		return product.Response(), nil
	}
	logger = logger.With().RawJSON(log.KeyJsonCache, []byte(jsonCache)).Logger()
	logger.Info().Msg("found product in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	product := response.Product{}
	err = json.Unmarshal([]byte(jsonCache), &product)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal product from cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, err
	}
	logger = logger.With().Any(log.KeyProduct, product).Logger()
	logger.Info().Msg("unmarshaled product from cache")

	return product, nil
}

func (svc ProductService) UpdateProduct(
	c context.Context,
	id uuid.UUID,
	param request.Product,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService UpdateProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService UpdateProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "updating product to database").Logger()
	logger.Info().Msg("updating product to database")
	product, err := svc.queries.UpdateProduct(c, repository.UpdateProductParams{
		Name: param.Name,
		Price: pgtype.Numeric{
			Exp:              param.Price.Exponent(),
			InfinityModifier: pgtype.Finite,
			Int:              param.Price.Coefficient(),
			NaN:              false,
			Valid:            true,
		},
		Quantity: int32(param.Quantity),
		ID:       id,
	})
	if err != nil {
		err = fmt.Errorf("failed to update product with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	logger = logger.With().Any("product", product).Logger()
	logger.Info().Msg("updated product to database")

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger = logger.With().Str(log.KeyProcess, "updating product to cache").
		Str(log.KeyCacheKey, cacheKey).
		Logger()
	logger.Info().Msg("updating product to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", product).Err()
	if err != nil {
		err = fmt.Errorf("failed to update product to cache with error=%w", err)
		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	logger.Info().Msg("updated product to cache")

	return product, nil
}

func (svc ProductService) RemoveProduct(
	c context.Context,
	id uuid.UUID,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService RemoveProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService RemoveProduct").
		Logger()

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger = logger.With().
		Str(log.KeyProcess, "removing product in cache").
		Str(log.KeyCacheKey, cacheKey).
		Logger()
	logger.Info().Msg("removing product in cache")
	err := svc.cache.JSONDel(c, cacheKey, "$").Err()
	if err != nil {
		err = fmt.Errorf("failed to remove product in cache with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return repository.Product{}, err
	}
	logger.Info().Msg("removed product in cache")

	logger = logger.With().Str(log.KeyProcess, "removing product in database").Logger()
	logger.Info().Msg("removing product in database")
	product, err := svc.queries.DeleteProduct(c, id)
	if err != nil {
		err = fmt.Errorf("failed to remove product in database with error=%w", err)

		commonErrors.HandleError(err, span)
		logger.Error().Err(err).Msg(err.Error())

		return repository.Product{}, err
	}
	logger.Info().Msg("removed product in database")

	return product, nil
}
