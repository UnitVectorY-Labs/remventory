alter table items add column tags text[] not null default '{}', add column location text not null default '';
create index items_tags_idx on items using gin(tags);
alter table items alter column category_id drop not null;
create unique index item_image_checksum_idx on item_assets(item_id,(metadata_jsonb->>'sha256')) where metadata_jsonb->>'sha256' is not null;
alter table items drop constraint items_category_id_fkey;
alter table items add constraint items_category_id_fkey foreign key(category_id) references categories(id) on delete set null;
