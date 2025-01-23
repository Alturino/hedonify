-- name: InsertCart :one
insert into carts (user_id) values ($1) returning *;

-- name: FindCartById :one
select
    c.*,
    json_agg(to_json(ci.*)) as cart_items
from users as u
inner join carts as c on u.id = c.user_id
inner join cart_items as ci on c.id = ci.cart_id
where u.id = $1 and c.id = $2
group by c.id, c.user_id, c.created_at, c.updated_at;

-- name: FindCartByUserId :many
select
    c.*,
    to_json(ci.*) as cart_items
from users as u
inner join carts as c on u.id = c.user_id
inner join cart_items as ci on c.id = ci.cart_id
where u.id = $1;

-- name: FindCartItemById :one
select * from cart_items
where id = $1;

-- name: FindCartItemByCartId :many
select * from cart_items
where cart_id = $1;

-- name: DeleteCartByIdAndUserId :one
delete from carts
where id = $1 and user_id = $2 returning *;

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
