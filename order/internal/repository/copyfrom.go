// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: copyfrom.go

package repository

import (
	"context"
)

// iteratorForInsertOrderItem implements pgx.CopyFromSource.
type iteratorForInsertOrderItem struct {
	rows                 []InsertOrderItemParams
	skippedFirstNextCall bool
}

func (r *iteratorForInsertOrderItem) Next() bool {
	if len(r.rows) == 0 {
		return false
	}
	if !r.skippedFirstNextCall {
		r.skippedFirstNextCall = true
		return true
	}
	r.rows = r.rows[1:]
	return len(r.rows) > 0
}

func (r iteratorForInsertOrderItem) Values() ([]interface{}, error) {
	return []interface{}{
		r.rows[0].OrderID,
		r.rows[0].ProductID,
		r.rows[0].Quantity,
		r.rows[0].Price,
	}, nil
}

func (r iteratorForInsertOrderItem) Err() error {
	return nil
}

func (q *Queries) InsertOrderItem(ctx context.Context, arg []InsertOrderItemParams) (int64, error) {
	return q.db.CopyFrom(ctx, []string{"order_items"}, []string{"order_id", "product_id", "quantity", "price"}, &iteratorForInsertOrderItem{rows: arg})
}
