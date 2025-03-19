create table if not exists carts (
    id uuid primary key not null default (gen_random_uuid()),
    user_id uuid not null references users (id),
    created_at timestamptz not null default current_timestamp,
    updated_at timestamptz not null default current_timestamp
);

create table if not exists cart_items (
    id uuid primary key not null default (gen_random_uuid()),
    cart_id uuid not null references carts (id) on delete cascade,
    product_id uuid not null references products (id),
    quantity serial not null,
    price numeric not null,
    created_at timestamptz not null default current_timestamp,
    updated_at timestamptz not null default current_timestamp
);
