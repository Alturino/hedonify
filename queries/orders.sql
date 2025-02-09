-- name: InsertOrder :one
insert into orders (id, user_id) values ($1, $2) returning *;

-- name: FindOrderById :one
select
    o.*,
    json_agg(to_json(oi.*)) as order_items
from users as u
inner join orders as o on u.id = o.user_id
inner join order_items as oi on o.id = oi.order_id
where u.id = $1 and o.id = $2
group by o.id, o.user_id, o.created_at, o.updated_at;

-- name: FindOrderItemById :many
select * from order_items
where id = $1;

-- name: FindOrderByUserId :many
select * from orders
where user_id = $1;

-- name: FindOrderUserId :many
select o.*
from users as u
inner join orders as o on u.id = o.user_id
where u.id = $1;

-- name: FindOrderItemByIdAndUserId :many
select oi.*
from orders as o
inner join order_items as oi on o.id = oi.order_id
where
    id = coalesce(nullif($1, ''), $1, id) and user_id = coalesce(nullif($2, ''), $2, user_id);

-- name: DeleteOrderItemFromOrdersById :one
delete from order_items
where id = $1 returning *;

-- name: InsertOrders :copyfrom
insert into orders (id, user_id, created_at, updated_at) values (
    $1, $2, $3, $4
);

-- name: InsertOrderItem :copyfrom
insert into order_items (order_id, product_id, quantity, price) values (
    $1, $2, $3, $4
);

-- name: GetOrders :many
select
    o.*,
    json_agg(to_json(oi.*)) as order_items
from orders as o
inner join order_items as oi on o.id = oi.order_id
group by o.id, o.user_id, o.created_at, o.updated_at;
