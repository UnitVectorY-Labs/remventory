-- A shared field vocabulary, suggested by tags and copied by creation templates.
create table field_definitions (
 key text primary key,
 label text not null,
 data_type text not null,
 config jsonb not null default '{}',
 created_at timestamptz not null default now()
);
insert into field_definitions(key,label,data_type) values
 ('title','Title','text'), ('description','Description','text'), ('year','Year','number'),
 ('creator','Creator','text'), ('publisher','Publisher','text'), ('model','Model','text'),
 ('serial_number','Serial number','text'), ('condition','Condition','text'),
 ('format','Format','text'), ('platform','Platform','text'), ('location','Location','text'),
 ('quantity','Quantity','number'), ('links','Links','links');
-- Preserve vocabulary already deliberately created in earlier versions.
insert into field_definitions(key,label,data_type,config)
 select distinct on (key) key,label,case when data_type='url' then 'text' else data_type end,config_json
 from category_attributes order by key,category_id on conflict(key) do nothing;
insert into field_definitions(key,label,data_type)
 select distinct on (e.key) e.key,initcap(replace(e.key,'_',' ')),
 case jsonb_typeof(e.value) when 'number' then 'number' when 'boolean' then 'boolean' when 'array' then 'json' when 'object' then 'json' else 'text' end
 from items cross join lateral jsonb_each(attributes_jsonb) e
 order by e.key,items.created_at on conflict(key) do nothing;
create table tag_field_suggestions (
 tag text not null,
 field_key text not null references field_definitions(key),
 primary key(tag,field_key)
);
alter table categories add column default_tags text[] not null default '{}', add column default_values jsonb not null default '{}';
update categories set default_tags=array[name];
insert into tag_field_suggestions(tag,field_key) select c.name,a.key from categories c join category_attributes a on a.category_id=c.id on conflict do nothing;
update category_attributes set required=false;

-- Carry existing data forward, without retaining template ownership.
update items set attributes_jsonb=attributes_jsonb || jsonb_build_object('title',title)
 where not attributes_jsonb ? 'title' and title not in ('','Untitled item');
update items set attributes_jsonb=attributes_jsonb || jsonb_build_object('location',location)
 where not attributes_jsonb ? 'location' and location<>'';
update items set attributes_jsonb=attributes_jsonb || jsonb_build_object('quantity',quantity)
 where not attributes_jsonb ? 'quantity' and quantity<>1;
-- Keep earlier category organization as tags when detaching template membership.
update items i set tags=array_append(i.tags,c.name)
 from categories c where i.category_id=c.id and not exists(select 1 from unnest(i.tags) tag where lower(tag)=lower(c.name));
update items set category_id=null;
alter table items alter column title set default '';
alter table item_assets add column is_primary boolean not null default false;
update item_assets set is_primary=true where id in (select distinct on(item_id) id from item_assets order by item_id,created_at,id);
create unique index item_assets_primary_idx on item_assets(item_id) where is_primary;
