-- Seed ONLY a disposable database. This refreshes the two Smoke categories and
-- their dependent items so expected totals stay at 254 records / 256 units.
begin;
do $$ begin
  if exists(select 1 from categories where id not in ('11000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000002')) then
    raise exception 'smoke-seed.sql requires a disposable database with no other categories';
  end if;
end $$;
delete from proposals where proposed_payload_jsonb ->> 'title' like 'Archive Game %';
delete from categories where id in (
  '11000000-0000-0000-0000-000000000001',
  '11000000-0000-0000-0000-000000000002'
);
insert into categories(id,user_id,name,description) values
('11000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000001','Smoke Games','Disposable Remy model smoke fixture'),
('11000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000001','Smoke Sets','Disposable Remy model smoke fixture');
insert into category_attributes(category_id,key,label,data_type,display_order) values
('11000000-0000-0000-0000-000000000001','platform','Platform','text',1),
('11000000-0000-0000-0000-000000000001','year','Year','number',2),
('11000000-0000-0000-0000-000000000002','pieces','Pieces','number',1),
('11000000-0000-0000-0000-000000000002','year','Year','number',2);
insert into items(user_id,category_id,title,attributes_jsonb,quantity)
select '00000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000001',
       'Archive Game '||lpad(n::text,3,'0'),jsonb_build_object('platform','SmokeBox','year',2000+n%25),1
from generate_series(1,250) n;
insert into items(id,user_id,category_id,title,attributes_jsonb,quantity) values
('21000000-0000-0000-0000-000000000001','00000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000001','Zelda: Wild','{"platform":"Switch","year":2017}',2),
('21000000-0000-0000-0000-000000000002','00000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000001','Zelda: Kingdom','{"platform":"Switch","year":2023}',1),
('21000000-0000-0000-0000-000000000003','00000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000002','Millennium Falcon','{"pieces":7541,"year":2017}',1),
('21000000-0000-0000-0000-000000000004','00000000-0000-0000-0000-000000000001','11000000-0000-0000-0000-000000000002','Galaxy Explorer','{"pieces":1254,"year":2022}',2);
commit;
