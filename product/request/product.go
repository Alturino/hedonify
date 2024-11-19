package request

type ProductRequest struct {
	Name  string `validate:"required" json:"name"`
	Price string `validate:"required" json:"price"`
}
