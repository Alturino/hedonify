package response

import "github.com/Alturino/ecommerce/order/pkg/response"

type Result struct {
	Order response.Order `json:"order"`
	Err   error          `json:"err"`
}
