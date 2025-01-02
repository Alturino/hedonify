package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	commonErrors "github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/product/internal/common/cache"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/repository"
	"github.com/Alturino/ecommerce/product/request"
)

type ProductService struct {
	queries *repository.Queries
	cache   *redis.Client
}

func NewProductService(queries *repository.Queries, cache *redis.Client) ProductService {
	return ProductService{queries: queries, cache: cache}
}

func (svc *ProductService) InsertProduct(
	c context.Context,
	param request.Product,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService InsertProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "validating price").Logger()
	logger.Info().Msg("validating price")
	price, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed to validate price with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "inserting product to database").Logger()
	logger.Info().Msg("inserting product to database")
	product, err := svc.queries.InsertProduct(
		c,
		repository.InsertProductParams{
			Name: param.Name,
			Price: pgtype.Numeric{
				Int:              price.BigInt(),
				Exp:              price.Exponent(),
				NaN:              false,
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
			Quantity: int32(param.Quantity),
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to insert product with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger = logger.With().Any(log.KeyProduct, product).Logger()
	logger.Info().Msg("inserted product")

	logger = logger.With().Str(log.KeyProcess, "inserting product to cache").Logger()
	logger.Info().Msg("inserting product to cache")
	err = svc.cache.JSONSet(c, fmt.Sprintf(cache.KEY_PRODUCTS, product.ID.String()), "$", product).
		Err()
	if err != nil {
		err = fmt.Errorf("failed to inserting product to cache with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return product, nil
	}
	logger.Info().Msg("inserted product to cache")

	return product, nil
}

func (svc *ProductService) FindProducts(
	c context.Context,
	param request.FindProduct,
) (products []repository.Product, err error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).With().Str(log.KeyTag, "ProductService FindProducts").Logger()

	logger = logger.With().Str(log.KeyProcess, "finding products in cache").Logger()
	logger.Info().Msg("finding products in cache")
	jsonCache, err := svc.cache.JSONGet(c, fmt.Sprintf(cache.KEY_PRODUCTS_QUERY, param.Name, param.MinPrice, param.MaxPrice), "$").
		Result()
	if err != nil {
		err = fmt.Errorf("failed to get products from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())

		logger = logger.With().Str(log.KeyProcess, "validating minPrice").Logger()
		logger.Info().Msg("validating minPrice")
		minPrice, err := decimal.NewFromString(param.MinPrice)
		if err != nil {
			err = fmt.Errorf("failed to validate price minPrice error=%w", err)
			commonErrors.HandleError(err, logger, span)
			return nil, err
		}
		logger.Info().Msg("validated minPrice")

		logger = logger.With().Str(log.KeyProcess, "validating maxPrice").Logger()
		logger.Info().Msg("validating maxPrice")
		maxPrice, err := decimal.NewFromString(param.MaxPrice)
		if err != nil {
			err = fmt.Errorf("failed to validate price maxPrice error=%w", err)
			commonErrors.HandleError(err, logger, span)
			return nil, err
		}
		logger.Info().Msg("validated maxPrice")

		logger = logger.With().Str(log.KeyProcess, "finding products in database").Logger()
		logger.Info().Msg("finding products in database")
		products, err := svc.queries.FindProducts(c, repository.FindProductsParams{
			Column1: param.Name,
			Price: pgtype.Numeric{
				Int:              minPrice.BigInt(),
				Exp:              minPrice.Exponent(),
				NaN:              false,
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
			Price_2: pgtype.Numeric{
				Int:              maxPrice.BigInt(),
				Exp:              maxPrice.Exponent(),
				NaN:              false,
				InfinityModifier: pgtype.Finite,
				Valid:            true,
			},
		})
		if err != nil {
			err = fmt.Errorf("failed to get products from database with error=%w", err)
			logger.Info().Err(err).Msg(err.Error())
			return nil, err
		}
		logger = logger.With().Any(log.KeyProducts, products).Logger()
		logger.Info().Msg("found products in database")

		return products, err
	}
	logger.Info().Msg("found products in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	err = json.Unmarshal([]byte(jsonCache), &products)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal products from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())
		return nil, err
	}
	logger.Info().Msg("unmarshaled products from cache")

	return products, err
}

func (svc *ProductService) FindProductById(
	c context.Context,
	id uuid.UUID,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProductById")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService FindProductById").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "finding product in cache").Logger()
	logger.Info().Msg("finding product in cache")
	jsonCache, err := svc.cache.Get(c, fmt.Sprintf(cache.KEY_PRODUCTS, id.String())).Result()
	if err != nil {
		err = fmt.Errorf("failed to get product from cache with error=%w", err)
		logger.Info().Err(err).Msg(err.Error())
		logger = logger.With().Str(log.KeyProcess, "finding product in database").Logger()
		product, err := svc.queries.FindProductById(c, id)
		if err != nil {
			err = fmt.Errorf("failed to find product in database with error=%w", err)
			commonErrors.HandleError(err, logger, span)
			return repository.Product{}, err
		}
		logger = logger.With().Any(log.KeyProduct, product).Logger()
		logger.Info().Msg("found product in database")
		return product, nil
	}
	logger.Info().Msg("found product in cache")

	logger = logger.With().Str(log.KeyProcess, "unmarshaling cache").Logger()
	logger.Info().Msg("unmarshaling cache")
	product := repository.Product{}
	err = json.Unmarshal([]byte(jsonCache), &product)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal product from cache with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger = logger.With().Any(log.KeyProduct, product).Logger()
	logger.Info().Msg("unmarshaled product from cache")

	return product, nil
}

func (svc *ProductService) UpdateProduct(
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

	logger = logger.With().Str(log.KeyProcess, "validating price").Logger()
	logger.Info().Msg("validating price")
	price, err := decimal.NewFromString(param.Price)
	if err != nil {
		err = fmt.Errorf("failed to validate price with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger.Info().Msg("validated price")

	logger = logger.With().Str(log.KeyProcess, "updating product to database").Logger()
	logger.Info().Msg("updating product to database")
	product, err := svc.queries.UpdateProduct(c, repository.UpdateProductParams{
		Name: param.Name,
		Price: pgtype.Numeric{
			Int:              price.BigInt(),
			Exp:              price.Exponent(),
			NaN:              false,
			InfinityModifier: pgtype.Finite,
			Valid:            true,
		},
		Quantity: int32(param.Quantity),
		ID:       id,
	})
	if err != nil {
		err = fmt.Errorf("failed to update product with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger = logger.With().Any("product", product).Logger()
	logger.Info().Msg("updated product to database")

	logger = logger.With().Str(log.KeyProcess, "updating product to cache").Logger()
	logger.Info().Msg("updating product to cache")
	err = svc.cache.JSONSet(c, fmt.Sprintf(cache.KEY_PRODUCTS, id.String()), "$", product).Err()
	if err != nil {
		err = fmt.Errorf("failed to update product to cache with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger.Info().Msg("updated product to cache")

	return product, nil
}

func (svc *ProductService) RemoveProduct(
	c context.Context,
	id uuid.UUID,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService RemoveProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService RemoveProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "removing product in cache").Logger()
	logger.Info().Msg("removing product in cache")
	err := svc.cache.JSONDel(c, fmt.Sprintf(cache.KEY_PRODUCTS, id.String()), "$").Err()
	if err != nil {
		err = fmt.Errorf("failed to remove product in cache with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger.Info().Msg("removed product in cache")

	logger = logger.With().Str(log.KeyProcess, "removing product in database").Logger()
	logger.Info().Msg("removing product in database")
	product, err := svc.queries.DeleteProduct(c, id)
	if err != nil {
		err = fmt.Errorf("failed to remove product in database with error=%w", err)
		commonErrors.HandleError(err, logger, span)
		return repository.Product{}, err
	}
	logger.Info().Msg("removed product in database")

	return product, nil
}
