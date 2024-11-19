-- name: GetProducts :many
select * from products;

-- name: InsertProduct :one
insert into products (product_name, price, amount) values (
    $1, $2, $3
) returning *;

-- name: FindProductByIdOrName :many
select *
from products
where id = $1 or product_name ilike '%' || $2::text || '%';
