create table if not exists orders (
    id uuid primary key not null default (gen_random_uuid()),
    user_id uuid not null references users (id),
    created_at timestamp not null default current_timestamp,
    updated_at timestamp not null default current_timestamp
);

create table if not exists order_items (
    id uuid primary key not null default (gen_random_uuid()),
    order_id uuid not null references orders (id) on delete cascade,
    product_id uuid not null references products (id),
    quantity serial not null,
    price numeric not null,
    created_at timestamp not null default current_timestamp,
    updated_at timestamp not null default current_timestamp
);
