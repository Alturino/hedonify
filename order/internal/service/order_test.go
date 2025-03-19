package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/errors"
	"github.com/Alturino/ecommerce/internal/repository"
	inResponse "github.com/Alturino/ecommerce/order/internal/response"
	"github.com/Alturino/ecommerce/order/pkg/request"
	"github.com/Alturino/ecommerce/order/pkg/response"
	productRes "github.com/Alturino/ecommerce/product/pkg/response"
)

func TestBatchOrderCreation(t *testing.T) {
	tests := []BatchOrderCreationTest{
		{
			name: "given empty order and available product quantity should return empty order",
			input: func() (orders []request.CreateOrder, orderId []uuid.UUID, orderItemIds []uuid.UUID, product []productRes.Product, users []repository.User) {
				return nil, nil, nil, nil, nil
			},
			seedPath: []string{filepath.Join("seed", "products.seed.sql")},
			setup:    setup(t),
			teardown: teardown(t),
			expected: func(orderId []uuid.UUID, orderItemIds []uuid.UUID, product []productRes.Product, users []repository.User) map[string]response.Order {
				return map[string]response.Order{}
			},
			expectedRemainingQuantity: 1000,
			expectedErr:               nil,
		},
		{
			name: "given order and available product quantity should return order",
			input: func() (orders []request.CreateOrder, orderIds []uuid.UUID, orderItemIds []uuid.UUID, products []productRes.Product, users []repository.User) {
				productByte, err := os.ReadFile(filepath.Join("seed", "products.seed.json"))
				if err != nil {
					t.Fatalf("failed reading products.seed.json with error: %s", err)
				}

				products = []productRes.Product{}
				err = json.NewDecoder(bytes.NewReader(productByte)).Decode(&products)
				if err != nil {
					t.Fatalf("failed decoding products.seed.json with error: %s", err)
				}

				userByte, err := os.ReadFile(filepath.Join("seed", "users.seed.json"))
				if err != nil {
					t.Fatalf("failed reading users.seed.json with error: %s", err)
				}

				users = []repository.User{}
				err = json.NewDecoder(bytes.NewReader(userByte)).Decode(&users)
				if err != nil {
					t.Fatalf("failed decoding products.seed.json with error: %s", err)
				}

				orderCount := 2
				orderIds = make([]uuid.UUID, 0, orderCount)
				for range orderCount {
					orderIds = append(orderIds, uuid.New())
				}

				orderItemCount := 4
				orderItemIds = make([]uuid.UUID, 0, orderItemCount)
				for range orderItemCount {
					orderItemIds = append(orderItemIds, uuid.New())
				}

				orders = []request.CreateOrder{
					{
						ID:     orderIds[0],
						UserId: users[0].ID,
						OrderItems: []request.OrderItem{
							{
								ID:        orderItemIds[0],
								OrderID:   orderIds[0],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[1],
								OrderID:   orderIds[0],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
						ResultChannel: make(chan inResponse.Result, 1),
						TraceLink:     trace.Link{},
					},
					{
						ID:     orderIds[1],
						UserId: users[1].ID,
						OrderItems: []request.OrderItem{
							{
								ID:        orderItemIds[2],
								OrderID:   orderIds[1],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[3],
								OrderID:   orderIds[1],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
						ResultChannel: make(chan inResponse.Result, 1),
						TraceLink:     trace.Link{},
					},
				}
				return orders, orderIds, orderItemIds, products, users
			},
			seedPath: []string{filepath.Join("seed", "products.seed.sql")},
			setup:    setup(t),
			teardown: teardown(t),
			expected: func(orderIds []uuid.UUID, orderItemIds []uuid.UUID, products []productRes.Product, users []repository.User) map[string]response.Order {
				return map[string]response.Order{
					orderIds[0].String(): {
						ID:     orderIds[0],
						UserId: users[0].ID,
						OrderItems: []response.OrderItem{
							{
								ID:        orderItemIds[0],
								OrderId:   orderIds[0],
								ProductId: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[1],
								OrderId:   orderIds[0],
								ProductId: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
					},
					orderIds[1].String(): {
						ID:     orderIds[1],
						UserId: users[1].ID,
						OrderItems: []response.OrderItem{
							{
								ID:        orderItemIds[2],
								OrderId:   orderIds[1],
								ProductId: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[3],
								OrderId:   orderIds[1],
								ProductId: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
					},
				}
			},
			expectedRemainingQuantity: 960,
			expectedErr:               nil,
		},
		{
			name: "given order and unavailable product quantity should return error out of stock",
			input: func() (orders []request.CreateOrder, orderIds []uuid.UUID, orderItemIds []uuid.UUID, products []productRes.Product, users []repository.User) {
				productByte, err := os.ReadFile(filepath.Join("seed", "products.seed.json"))
				if err != nil {
					t.Fatalf("failed reading products.seed.json with error: %s", err)
				}

				products = []productRes.Product{}
				err = json.NewDecoder(bytes.NewReader(productByte)).Decode(&products)
				if err != nil {
					t.Fatalf("failed decoding products.seed.json with error: %s", err)
				}

				userByte, err := os.ReadFile(filepath.Join("seed", "users.seed.json"))
				if err != nil {
					t.Fatalf("failed reading users.seed.json with error: %s", err)
				}

				users = []repository.User{}
				err = json.NewDecoder(bytes.NewReader(userByte)).Decode(&users)
				if err != nil {
					t.Fatalf("failed decoding products.seed.json with error: %s", err)
				}

				orderCount := 2
				orderIds = make([]uuid.UUID, 0, orderCount)
				for range orderCount {
					orderIds = append(orderIds, uuid.New())
				}

				orderItemCount := 4
				orderItemIds = make([]uuid.UUID, 0, orderItemCount)
				for range orderItemCount {
					orderItemIds = append(orderItemIds, uuid.New())
				}

				orders = []request.CreateOrder{
					{
						ID:     orderIds[0],
						UserId: users[0].ID,
						OrderItems: []request.OrderItem{
							{
								ID:        orderItemIds[0],
								OrderID:   orderIds[0],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[1],
								OrderID:   orderIds[0],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
						ResultChannel: make(chan inResponse.Result, 1),
						TraceLink:     trace.Link{},
					},
					{
						ID:     orderIds[1],
						UserId: users[1].ID,
						OrderItems: []request.OrderItem{
							{
								ID:        orderItemIds[2],
								OrderID:   orderIds[1],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
							{
								ID:        orderItemIds[3],
								OrderID:   orderIds[1],
								ProductID: products[0].ID,
								Price:     decimal.NewFromInt(10),
								Quantity:  10,
							},
						},
						ResultChannel: make(chan inResponse.Result, 1),
						TraceLink:     trace.Link{},
					},
				}
				return orders, orderIds, orderItemIds, products, users
			},
			seedPath: []string{filepath.Join("seed", "empty_products.seed.sql")},
			setup:    setup(t),
			teardown: teardown(t),
			expected: func(orderId []uuid.UUID, orderItemIds []uuid.UUID, product []productRes.Product, users []repository.User) map[string]response.Order {
				return map[string]response.Order{}
			},
			expectedRemainingQuantity: 0,
			expectedErr:               errors.ErrOutOfStock,
		},
	}

	for _, tt := range tests {
		if tt.setup == nil {
			t.Fatal("setup function is required")
		}
		if tt.teardown == nil {
			t.Fatal("teardown function is required")
		}
		if tt.input == nil {
			t.Fatal("input function is required")
		}
		if tt.expected == nil {
			t.Fatal("expected function is required")
		}

		t.Run(tt.name, func(t *testing.T) {
			c := context.Background()
			c = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339Nano}).
				WithContext(c)
			redis, pool, pgContainer, redisContainer, queries, orderService := tt.setup(
				c,
				tt.seedPath...,
			)
			defer tt.teardown(redis, pool, pgContainer, redisContainer)

			requests, orderIds, orderItemIds, products, users := tt.input()
			expected := tt.expected(orderIds, orderItemIds, products, users)

			log.Println("executing test")
			actual, err := orderService.BatchCreateOrder(c, requests)
			if err != nil && tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr, "error should be equal to expected")
			}
			for orderId, order := range actual {
				order.CreatedAt = time.Time{}
				order.UpdatedAt = time.Time{}
				order.Status = ""
				for i, orderItem := range order.OrderItems {
					orderItem.CreatedAt = time.Time{}
					orderItem.UpdatedAt = time.Time{}
					order.OrderItems[i] = orderItem
				}
				actual[orderId] = order
			}
			assert.EqualValues(t, expected, actual, "order results should be equal to expected")

			orderedProductIds := []uuid.UUID{}
			for _, order := range actual {
				for _, orderItem := range order.OrderItems {
					orderedProductIds = append(orderedProductIds, orderItem.ProductId)
				}
			}
			orderedProduct, err := queries.FindProductsByIds(c, orderedProductIds)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range orderedProduct {
				assert.EqualValues(
					t,
					tt.expectedRemainingQuantity,
					p.Quantity,
					"remaining quantity should be equal to expected",
				)
			}

			for _, r := range requests {
				res := <-r.ResultChannel
				log.Println("res", res)
				if res.Err != nil {
					assert.ErrorIs(
						t,
						res.Err,
						errors.ErrOutOfStock,
						"returned order error should be equal to expected", res.Err, tt.expected,
					)
				}
			}
		})
	}
}
