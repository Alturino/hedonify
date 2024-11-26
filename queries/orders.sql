-- name: InsertOrder :one
insert into orders (id, user_id) values ($1, $2) returning *;

-- name: FindOrderById :one
select o.*
from orders as o
inner join order_items as oi on o.id = oi.order_id
where o.id = $1;

-- name: FindOrderItemById :many
select oi.*
from orders as o
inner join order_items as oi on o.id = oi.order_id
where o.id = $1;

-- name: FindOrderByUserId :many
select * from orders where user_id = $1;

-- name: FindOrderByIdAndUserId :many
select o.*
from orders as o
inner join order_items as oi on o.id = oi.order_id
where
    id = coalesce(nullif($1, ''), $1, id) and user_id = coalesce(nullif($2, ''), $2, user_id);

-- name: FindOrderItemByIdAndUserId :many
select oi.*
from orders as o
inner join order_items as oi on o.id = oi.order_id
where
    id = coalesce(nullif($1, ''), $1, id) and user_id = coalesce(nullif($2, ''), $2, user_id);

-- name: DeleteOrderItemFromOrdersById :one
delete from order_items where id = $1 returning *;

-- name: InsertOrderItem :copyfrom
insert into order_items (order_id, product_id, quantity, price) values (
    $1, $2, $3, $4
);
