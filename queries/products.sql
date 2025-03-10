-- name: GetProducts :many
select * from products;

-- name: InsertProduct :one
insert into products (name, price, quantity) values ($1, $2, $3) returning *;

-- name: FindProductById :one
select * from products
where id = $1;

-- name: FindProductByIdLock :one
select * from products
where id = $1 for update skip locked;

-- name: FindProducts :many
select * from products;

-- name: FindProductByName :one
select * from products
where name = $1;

-- name: UpdateProduct :one
update products set name = $1, price = $2, quantity = $3, updated_at = now()
where id = $4 returning *;

-- name: UpdateProductQuantity :one
update products set quantity = $2
where id = $1 returning *;

-- name: DeleteProduct :one
delete from products
where id = $1 returning *;

-- name: FindProductsByIds :many
select * from products
where id = any($1::uuid []);

-- name: FindProductsByIdsLock :many
select * from products
where id = any($1::uuid []) for share;
