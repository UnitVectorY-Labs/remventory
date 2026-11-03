alter table canonical_metadata add column search_document tsvector generated always as
 (to_tsvector('simple',description || ' ' || fields::text)) stored;
create index canonical_metadata_search_idx on canonical_metadata using gin(search_document);
create index canonical_entities_label_idx on canonical_entities using gin(to_tsvector('simple',label));
create index canonical_metadata_refresh_idx on canonical_metadata(refresh_after);
