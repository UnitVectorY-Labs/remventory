-- Extend the existing starting templates rather than creating a second type system.
alter table categories add column intake_config jsonb not null default '{}';
update categories set intake_config=jsonb_build_object(
 'providers',case when lower(name) like '%vinyl%' then '["musicbrainz"]'::jsonb when lower(name) ~ '(movie|dvd|blu.?ray|vhs|game|hardware|console|accessor)' then '["wikidata"]'::jsonb when lower(name) like '%amiibo%' then '["amiibo","wikidata"]'::jsonb else '["amiibo","musicbrainz","wikidata"]'::jsonb end,
 'prompt',case when lower(name) like '%vinyl%' then 'Identify the specific physical pressing when barcode, catalogue number, label and format support it. Offer alternatives when uncertain; an album alone does not prove a pressing.' when lower(name) ~ '(movie|dvd|blu.?ray|vhs)' then 'Search bare titles and aliases. Identify the film or television works inside the physical package. Distinguish original work dates from retail release dates. For sets, resolve individual contained works when supported by the packaging.' when lower(name) like '%game%' then 'Distinguish underlying games from platforms, physical editions and releases. Search bare titles and use the returned descriptions to disambiguate.' when lower(name) like '%amiibo%' then 'Compare character, pose, base and packaging series. Resolve the specific figure variant and offer alternatives for human review.' else '' end,
 'display_fields','{}'::jsonb);
insert into users(id,display_name) values('00000000-0000-0000-0000-000000000001','Inventory owner') on conflict do nothing;
insert into categories(user_id,name,description,default_tags,default_values,intake_config)
 select '00000000-0000-0000-0000-000000000001'::uuid,v.name,v.description,array[v.name],'{}',jsonb_build_object('providers',v.providers::jsonb,'prompt',v.prompt,'display_fields','{}'::jsonb)
 from (values
 ('Vinyl','Records and pressings','["musicbrainz"]','Identify specific releases from visible barcode, catalogue number, label and format. Compare plausible pressings and ask the human when uncertain.'),
 ('Movies','DVD, Blu-ray and box sets','["wikidata"]','Search bare titles or aliases. Distinguish film, series, season and episode entities from physical releases. Resolve contained works when packaging supports them.'),
 ('Amiibo','Nintendo figures and cards','["amiibo","wikidata"]','Compare character, pose, base and packaging series. Search the bare character name, resolve specific head+tail figure IDs, and offer close variants for human selection. Cards and figures are different products.'),
 ('Video games','Games and editions','["wikidata"]','Distinguish the underlying game from platform, edition and physical release.'),
 ('Hardware','Consoles and accessories','["wikidata"]','Look for visible model numbers. Do not infer compatibility solely from brand.'),
 ('Other','Anything else','["amiibo","musicbrainz","wikidata"]','Explain missing coverage and allow manual reference links.')) v(name,description,providers,prompt)
 where not exists(select 1 from categories c where (lower(c.name)=lower(v.name) or (v.name='Vinyl' and lower(c.name) in ('vinyl records','vinylrecords')) or (v.name='Video games' and lower(c.name) in ('video game','games'))));
insert into category_attributes(category_id,key,label,data_type,required,display_order,config_json)
 select c.id,f.key,f.label,f.data_type,false,100,f.config from categories c cross join field_definitions f
 where f.key in ('title','condition','location','links')
 and not exists(select 1 from category_attributes a where a.category_id=c.id and a.key=f.key);
