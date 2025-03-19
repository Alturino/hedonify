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

	"github.com/Alturino/ecommerce/internal/constants"
	inOtel "github.com/Alturino/ecommerce/internal/otel"
	"github.com/Alturino/ecommerce/internal/repository"
	"github.com/Alturino/ecommerce/product/internal/cache"
	"github.com/Alturino/ecommerce/product/internal/otel"
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
		Ctx(c).
		Str(constants.KEY_TAG, "ProductService InsertProduct").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "finding product in database").Logger()
	logger.Trace().Msg("finding product in database")
	span.AddEvent("finding product in database")
	product, err := svc.queries.FindProductByName(c, param.Name)
	if err == nil {
		err = fmt.Errorf("product is already exist with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Info().Err(err).Msg(err.Error())
		return product.Response(), err
	}
	span.AddEvent("product is not exist in database")
	logger.Info().Msg("product is not exist in database")

	logger = logger.With().Str(constants.KEY_PROCESS, "inserting product to database").Logger()
	logger.Trace().Msg("inserting product to database")
	span.AddEvent("inserting product to database")
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
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, err
	}
	span.AddEvent("inserted product to database")
	logger = logger.With().Any(constants.KEY_PRODUCT, product).Logger()
	logger.Info().Msg("inserted product to database")

	cacheKey := cache.KEY_PRODUCTS + product.ID.String()
	logger = logger.With().
		Str(constants.KEY_PROCESS, "inserting product to cache").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()
	logger.Trace().Msg("inserting product to cache")
	span.AddEvent("inserting product to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", product).Err()
	if err != nil {
		err = fmt.Errorf("failed to inserting product to cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, nil
	}
	span.AddEvent("inserted product to cache")
	logger.Info().Msg("inserted product to cache")

	logger.Info().Msg("inserted product to database and cache")
	return product.Response(), nil
}

func (svc ProductService) GetProducts(
	c context.Context,
) ([]repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductService FindProducts").
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find.product.database").Logger()
	logger.Trace().Msg("finding products in database")
	span.AddEvent("finding products in database")
	products, err := svc.queries.FindProducts(c)
	if err != nil {
		err = fmt.Errorf("failed to get products from database with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Info().Err(err).Msg(err.Error())
		return nil, err
	}
	logger = logger.With().Any(constants.KEY_PRODUCTS, products).Logger()
	span.AddEvent("found products in database")
	logger.Info().Msg("found products in database")

	return products, err
}

func (svc ProductService) FindProductById(
	c context.Context,
	id uuid.UUID,
) (product response.Product, err error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProductById")
	defer span.End()

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductService FindProductById").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "find product in cache").Logger()
	logger.Trace().Msg("finding product in cache")
	jsonCache, err := svc.cache.JSONGet(c, cacheKey).Result()
	if err != nil || err == redis.Nil || jsonCache == "" {
		err = fmt.Errorf("failed to get product from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())

		logger = logger.With().Str(constants.KEY_PROCESS, "finding product in database").Logger()
		logger.Trace().Msg("finding product in database")
		span.AddEvent("finding product in database")
		product, err := svc.queries.FindProductById(c, id)
		if err != nil {
			err = fmt.Errorf("failed to find product in database with error=%w", err)
			inOtel.RecordError(err, span)
			logger.Error().Err(err).Msg(err.Error())
			return response.Product{}, err
		}
		span.AddEvent("found product in database")
		logger = logger.With().Any(constants.KEY_PRODUCT, product).Logger()

		logger.Info().Msg("found product in database")
		return product.Response(), nil
	}
	span.AddEvent("found product in cache")
	logger = logger.With().Str(constants.KEY_JSON_CACHE, jsonCache).Logger()
	logger.Debug().Msg("found product in cache")

	logger.Trace().Msg("unmarshalling product from cache")
	err = json.Unmarshal([]byte(jsonCache), &product)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal jsonCache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return response.Product{}, err
	}
	logger = logger.With().Any(constants.KEY_PRODUCT, product).Logger()
	logger.Trace().Msg("unmarshalling product from cache")

	logger.Info().Msg("found product in cache")
	return product, nil
}

func (svc ProductService) UpdateProduct(
	c context.Context,
	id uuid.UUID,
	param request.Product,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService UpdateProduct")
	defer span.End()

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductService UpdateProduct").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "updating product to database").Logger()
	logger.Trace().Msg("updating product to database")
	span.AddEvent("updating product to database")
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
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	span.AddEvent("updated product to database")
	logger = logger.With().Any("product", product).Logger()
	logger.Info().Msg("updated product to database")

	logger = logger.With().Str(constants.KEY_PROCESS, "update product to cache").Logger()
	logger.Trace().Msg("updating product to cache")
	span.AddEvent("updating product to cache")
	err = svc.cache.JSONSet(c, cacheKey, "$", product).Err()
	if err != nil {
		err = fmt.Errorf("failed to update product to cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	span.AddEvent("updated product to cache")
	logger.Info().Msg("updated product to cache")

	return product, nil
}

func (svc ProductService) RemoveProduct(
	c context.Context,
	id uuid.UUID,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService RemoveProduct")
	defer span.End()

	cacheKey := cache.KEY_PRODUCTS + id.String()
	logger := zerolog.Ctx(c).
		With().
		Ctx(c).
		Str(constants.KEY_TAG, "ProductService RemoveProduct").
		Str(constants.KEY_CACHE_KEY, cacheKey).
		Logger()

	logger = logger.With().Str(constants.KEY_PROCESS, "removing product in cache").Logger()
	logger.Trace().Msg("removing product in cache")
	span.AddEvent("removing product in cache")
	err := svc.cache.JSONDel(c, cacheKey, "$").Err()
	if err != nil {
		err = fmt.Errorf("failed to remove product in cache with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	span.AddEvent("removed product in cache")
	logger.Info().Msg("removed product in cache")

	logger = logger.With().Str(constants.KEY_PROCESS, "removing product in database").Logger()
	logger.Trace().Msg("removing product in database")
	span.AddEvent("removing product in database")
	product, err := svc.queries.DeleteProduct(c, id)
	if err != nil {
		err = fmt.Errorf("failed to remove product in database with error=%w", err)
		inOtel.RecordError(err, span)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	span.AddEvent("removed product in database")
	logger.Info().Msg("removed product in database")

	return product, nil
}
