-- name: InsertUser :one
insert into users (username, email, password, created_at, updated_at) values (
    $1, $2, $3, $4, $5
) returning *;

-- name: FindByEmail :one
select * from users
where email = $1;

-- name: FindById :one
select * from users
where id = $1;
