create extension if not exists pgcrypto;

create table if not exists users (
	id uuid primary key default gen_random_uuid(),
	display_name text not null,
	created_at timestamptz not null default now()
);

create table if not exists categories (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	name text not null,
	description text,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (user_id, name)
);

create table if not exists category_attributes (
	id uuid primary key default gen_random_uuid(),
	category_id uuid not null references categories(id) on delete cascade,
	key text not null,
	label text not null,
	data_type text not null,
	required boolean not null default false,
	display_order integer not null default 0,
	config_json jsonb not null default '{}'::jsonb,
	unique (category_id, key)
);

create table if not exists items (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	category_id uuid not null references categories(id) on delete cascade,
	title text not null,
	attributes_jsonb jsonb not null default '{}'::jsonb,
	quantity integer not null default 1 check (quantity > 0),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists proposals (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	type text not null,
	status text not null default 'pending',
	proposed_payload_jsonb jsonb not null,
	reason text,
	created_at timestamptz not null default now(),
	decided_at timestamptz,
	check (status in ('pending', 'approved', 'rejected'))
);

create table if not exists agent_events (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	session_id text not null,
	event_type text not null,
	payload_jsonb jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create table if not exists settings (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	key text not null,
	value_jsonb jsonb not null,
	unique (user_id, key)
);

create table if not exists item_assets (
	id uuid primary key default gen_random_uuid(),
	item_id uuid not null references items(id) on delete cascade,
	object_key text not null,
	mime_type text not null,
	metadata_jsonb jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index if not exists categories_user_id_idx on categories(user_id);
create index if not exists category_attributes_category_id_idx on category_attributes(category_id);
create index if not exists items_category_id_idx on items(category_id);
create index if not exists items_attributes_jsonb_idx on items using gin(attributes_jsonb);
create index if not exists proposals_user_status_idx on proposals(user_id, status);
create index if not exists agent_events_session_created_idx on agent_events(session_id, created_at desc);
