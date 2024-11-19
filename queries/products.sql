-- name: GetProducts :many
select * from products;

-- name: InsertProduct :one
insert into products (product_name, price) values ($1, $2) returning *;
