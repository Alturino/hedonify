package service

import (
	"context"

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
	param request.ProductRequest,
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
	product, err := p.queries.InsertProduct(
		c,
		repository.InsertProductParams{ProductName: param.Name, Price: param.Price},
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
