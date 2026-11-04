-- Model-selected presentation is separate from externally asserted metadata.
create table canonical_presentations (
 entity_id uuid not null references canonical_entities(id), provider text not null,
 source_fingerprint text not null, plan jsonb not null,
 model text not null, created_at timestamptz not null default now(),
 primary key(entity_id,provider)
);
