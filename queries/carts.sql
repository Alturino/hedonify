-- name: InsertCart :one
insert into carts (user_id, total_price) values ($1, $2) returning *;

-- name: FindCartById :one
select * from carts where id = $1;

-- name: FindCartByUserId :many
select * from carts where user_id = $1;
