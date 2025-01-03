-- name: InsertCart :one
insert into carts (user_id) values ($1) returning *;

-- name: FindCartById :one
select
    c.*,
    jsonb_agg(ci.*) as cart_items
from carts as c
inner join cart_items as ci on c.id = ci.cart_id
where c.id = $1;

-- name: FindCartByUserId :many
select
    c.*,
    jsonb_agg(ci.*) as cart_items
from carts as c
inner join cart_items as ci on c.id = ci.cart_id
where user_id = $1;

-- name: FindCartItemById :one
select * from cart_items
where id = $1;

-- name: FindCartItemByCartId :many
select * from cart_items
where cart_id = $1;

-- name: DeleteCartItemFromCartsById :one
delete from cart_items
where id = $1 and cart_id = $2 returning *;

-- name: InsertCartItem :one
insert into cart_items (id, cart_id, product_id, quantity, price) values (
    $1, $2, $3, $4, $5
) returning *;

-- name: InsertCartItems :copyfrom
insert into cart_items (id, cart_id, product_id, quantity, price) values (
    $1, $2, $3, $4, $5
);
