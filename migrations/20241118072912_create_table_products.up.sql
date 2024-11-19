create table if not exists products (
    id uuid primary key not null default (gen_random_uuid()),
    product_name varchar(128) unique not null default (''),
    price numeric not null default (0),
    created_at timestamp not null default current_timestamp,
    updated_at timestamp not null default current_timestamp
);

create index if not exists idx_product_name on products (product_name);
