package request

type Product struct {
	Name     string `validate:"required" json:"name"`
	Price    string `validate:"required" json:"price"`
	Quantity int    `validate:"required" json:"quantity"`
}

type FindProduct struct {
	Name     string
	MinPrice string `validate:"numeric"`
	MaxPrice string `validate:"numeric"`
}
