-- name: InsertCart :one
insert into carts (user_id) values ($1) returning *;

-- name: FindCartById :one
select * from carts where id = $1;

-- name: FindCartByUserId :many
select * from carts where user_id = $1;

-- name: FindCartItemById :one
select * from cart_items where id = $1;

-- name: DeleteCartItemFromCartsById :one
delete from cart_items where id = $1 and cart_id = $2 returning *;

-- name: InsertCartItem :one
insert into cart_items (cart_id, product_id, quantity, price) values (
    $1, $2, $3, $4
) returning *;

-- name: InsertCartItems :copyfrom
insert into cart_items (cart_id, product_id, quantity, price) values (
    $1, $2, $3, $4
);
