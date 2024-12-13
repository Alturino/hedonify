-- name: GetProducts :many
select * from products;

-- name: InsertProduct :one
insert into products (name, price, quantity) values (
    $1, $2, $3
) returning *;

-- name: FindProductById :one
select * from products where id = $1;

-- name: FindProducts :one
select * from products where name like '%' | $1::text | '%' and price >= $2 and price <= $3;
