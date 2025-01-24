package validate

import (
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

func ValidatePrice(fl validator.FieldLevel) bool {
	value := fl.Field().Interface().(string)
	d, err := decimal.NewFromString(value)
	if err != nil {
		return false
	}
	return d.IsPositive()
}

func PriceValue(v reflect.Value) interface{} {
	n, ok := v.Interface().(decimal.Decimal)
	if !ok {
		return nil
	}
	return n.PowWithPrecision
}
