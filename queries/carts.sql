-- name: InsertCart :one
insert into carts (user_id) values ($1) returning *;

-- name: FindCartById :one
select * from carts where id = $1;

-- name: FindCartByUserId :many
select * from carts where user_id = $1;

-- name: DeleteCartItemFromCartsById :one
delete from cart_items where id = $1 returning *;

-- name: InsertCartItem :copyfrom
insert into cart_items (cart_id, product_id, quantity, price) values (
    $1, $2, $3, $4
);
