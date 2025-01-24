package request

import (
	"github.com/google/uuid"
)

type FindOrderByUserId struct {
	UserId uuid.UUID `validate:"uuid"`
}

type FindOrderById struct {
	UserId  uuid.UUID `validate:"required,uuid"`
	OrderId uuid.UUID `validate:"required,uuid"`
}

type FindOrders struct {
	UserId  uuid.UUID `validate:"required,uuid"`
	OrderId uuid.UUID `validate:"required,uuid"`
}
