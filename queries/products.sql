-- name: GetProducts :many
select * from products;

-- name: InsertProduct :one
insert into products (name, price, quantity) values (
    $1, $2, $3
) returning *;

-- name: FindProductByIdOrName :many
select *
from products
where id = $1 or name ilike '%' || $2::text || '%';
