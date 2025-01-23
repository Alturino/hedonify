create table if not exists users (
    id uuid primary key not null default (gen_random_uuid()),
    username varchar(128) unique not null default (''),
    email varchar(128) unique not null default (''),
    password varchar(256) not null default (''),
    created_at timestamptz not null default current_timestamp,
    updated_at timestamptz not null default current_timestamp
);

create index if not exists idx_username on users (username);
create index if not exists idx_email on users (email);
