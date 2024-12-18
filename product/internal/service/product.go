package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/repository"
	"github.com/Alturino/ecommerce/product/request"
	"github.com/Alturino/ecommerce/product/response"
)

type ProductService struct {
	queries *repository.Queries
	cache   *redis.Client
}

func NewProductService(queries *repository.Queries, cache *redis.Client) ProductService {
	return ProductService{queries: queries, cache: cache}
}

func (p *ProductService) InsertProduct(
	c context.Context,
	param request.Product,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService InsertProduct").
		Logger()

	logger = logger.With().Str(log.KeyProcess, "inserting product").Logger()
	logger.Info().Msg("inserting product")
	var price pgtype.Numeric
	if err := price.Scan(param.Price); err != nil {
		err = fmt.Errorf("failed to insert product with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	product, err := p.queries.InsertProduct(
		c,
		repository.InsertProductParams{
			Name:     param.Name,
			Price:    price,
			Quantity: int32(param.Quantity),
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to insert product with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return repository.Product{}, err
	}
	logger.Info().Msg("inserted product")

	logger = logger.With().Str(log.KeyProcess, "caching product").Logger()
	logger.Info().Msg("caching product")
	cache := response.Product{
		ID:        product.ID,
		Name:      product.Name,
		Price:     decimal.NewFromBigInt(product.Price.Int, product.Price.Exp),
		Quantity:  product.Quantity,
		CreatedAt: product.CreatedAt.Time,
		UpdatedAt: product.UpdatedAt.Time,
	}
	err = p.cache.HSet(c, fmt.Sprintf("products:%s", product.ID.String()), cache).Err()
	if err != nil {
		err = fmt.Errorf("failed caching product with error=%w", err)
		logger.Error().Err(err).Msg(err.Error())
		return product, nil
	}
	logger.Info().Msg("cached product")

	return product, nil
}

func (p *ProductService) FindProducts(
	c context.Context,
	param request.FindProduct,
) (products []repository.Product, err error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProducts")
	defer span.End()

	// logger := zerolog.Ctx(c).
	// 	With().
	// 	Str(log.KeyTag, "ProductService FindProducts").
	// 	Str(log.KeyProductName, param.Name).
	// 	Logger()
	//
	// logger = logger.With().Str(log.KeyProcess, "finding products in cache").Logger()
	// logger.Info().Msgf("finding products in cache")
	//
	// cacheKey := fmt.Sprintf("products:%s", param.Name)
	//
	// cache, err := p.cache.HGet(c, cacheKey, cacheKey).Result()
	// if err != nil {
	// 	err = fmt.Errorf("failed finding products in cache with error=%w", err)
	// 	logger.Info().Err(err).Msg(err.Error())
	//
	// 	logger = logger.With().Str(log.KeyProcess, "finding products in database").Logger()
	// 	logger.Info().Msg("finding products in database")
	// 	products, err := p.queries.FindProductByIdOrName(
	// 		c,
	// 		repository.FindProductByIdOrNameParams{ID: param.ID, Column2: param.Name},
	// 	)
	// 	if err != nil || len(products) == 0 {
	// 		err = fmt.Errorf(
	// 			"products by id=%s or name=%s not found with error=%w",
	// 			param.ID.String(),
	// 			param.Name,
	// 			err,
	// 		)
	// 		logger.Error().Err(err).Msg(err.Error())
	// 		return nil, err
	// 	}
	// 	logger = logger.With().Any("products", products).Logger()
	// 	logger.Info().Msg("found products in database")
	//
	// 	logger = logger.With().Str(log.KeyProcess, "inserting products to cache").Logger()
	// 	logger.Info().Msg("inserting products to cache")
	// 	err = p.cache.Set(c, cacheKey, products, time.Hour*6).Err()
	// 	if err != nil {
	// 		err = fmt.Errorf("failed inserting products to cache with error=%w", err)
	// 		logger.Error().Err(err).Msg(err.Error())
	// 		return nil, err
	// 	}
	// 	logger.Info().Msg("inserted products to cache")
	//
	// 	return products, nil
	// }
	//
	// logger = logger.With().Str(log.KeyProcess, "unmarshal json").Logger()
	// logger.Info().Msg("unmarshal json")
	// err = json.Unmarshal([]byte(cache), &products)
	// if err != nil {
	// 	err = fmt.Errorf("failed unmarshal json with error=%w", err)
	// 	logger.Error().Err(err).Msg(err.Error())
	// 	return nil, err
	// }
	// logger = logger.With().Any(log.KeyProducts, products).Logger()
	// logger.Info().Msg("success unmarshal json")
	//
	return products, nil
}

func (p *ProductService) FindProductById(
	c context.Context,
	id uuid.UUID,
) (product repository.Product, err error) {
	return product, err
}
