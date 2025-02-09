package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	ID        uuid.UUID       `json:"id"         redis:"id"`
	Name      string          `json:"name"       redis:"name"`
	Price     decimal.Decimal `json:"price"      redis:"price"`
	Quantity  int32           `json:"quantity"   redis:"quantity"`
	CreatedAt time.Time       `json:"created_at" redis:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" redis:"updated_at"`
}
