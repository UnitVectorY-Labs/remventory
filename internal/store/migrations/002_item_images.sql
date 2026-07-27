alter table item_assets
	add column if not exists thumbnail_object_key text,
	add column if not exists original_filename text not null default '',
	add column if not exists width integer not null default 0,
	add column if not exists height integer not null default 0,
	add column if not exists size_bytes bigint not null default 0;

create index if not exists item_assets_item_created_idx on item_assets(item_id, created_at);
