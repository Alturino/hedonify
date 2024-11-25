package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/product/internal/common/otel"
	"github.com/Alturino/ecommerce/product/internal/repository"
	"github.com/Alturino/ecommerce/product/request"
)

type ProductService struct {
	queries *repository.Queries
}

func NewProductService(queries *repository.Queries) ProductService {
	return ProductService{queries: queries}
}

func (p *ProductService) InsertProduct(
	c context.Context,
	param request.InsertProductRequest,
) (repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService InsertProduct")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyTag, "ProductService InsertProduct").
		Logger()

	logger.Info().
		Str(log.KeyProcess, "inserting product").
		Msg("inserting product")
	var price pgtype.Numeric
	if err := price.Scan(param.Price); err != nil {
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "inserting product").
			Msgf("failed to insert product with error=%s", err.Error())
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
		logger.Error().
			Err(err).
			Str(log.KeyProcess, "inserting product").
			Msgf("failed to insert product with error=%s", err.Error())
		return repository.Product{}, err
	}
	logger.Info().
		Str(log.KeyProcess, "inserting product").
		Msg("inserted product")

	return product, nil
}

func (p *ProductService) FindProducts(
	c context.Context,
	param request.FindProductRequest,
) ([]repository.Product, error) {
	c, span := otel.Tracer.Start(c, "ProductService FindProducts")
	defer span.End()

	logger := zerolog.Ctx(c).
		With().
		Str(log.KeyProcess, fmt.Sprintf("finding products by id=%s or name=%s", param.ID.String(), param.Name)).
		Str(log.KeyTag, "ProductService FindProducts").
		Logger()

	logger.Info().Msgf("finding products by id=%s or name=%s", param.ID.String(), param.Name)
	products, err := p.queries.FindProductByIdOrName(
		c,
		repository.FindProductByIdOrNameParams{ID: param.ID, Column2: param.Name},
	)
	if err != nil || len(products) == 0 {
		logger.Error().
			Err(err).
			Msgf("products by id=%s or name=%s not found", param.ID.String(), param.Name)
		return nil, fmt.Errorf(
			"products by id=%s or name=%s not found",
			param.ID.String(),
			param.Name,
		)
	}
	logger.Info().
		Any("products", products).
		Msgf("found products by id=%s or name=%s", param.ID.String(), param.Name)

	return products, nil
}
