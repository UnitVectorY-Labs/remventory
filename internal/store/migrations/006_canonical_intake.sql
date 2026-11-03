-- External facts never replace user-entered item attributes.
create table canonical_entities (
 id uuid primary key default gen_random_uuid(),
 kind text not null, label text not null,
 created_at timestamptz not null default now()
);
create table canonical_identifiers (
 provider text not null, kind text not null, external_id text not null,
 entity_id uuid not null references canonical_entities(id), uri text not null,
 primary key(provider,kind,external_id)
);
create index canonical_identifiers_entity_idx on canonical_identifiers(entity_id);
create table canonical_metadata (
 entity_id uuid not null references canonical_entities(id), provider text not null,
 fields jsonb not null, raw jsonb, description text not null default '',
 license text not null, retrieved_at timestamptz not null, refresh_after timestamptz not null,
 refresh_status text not null default 'current', primary key(entity_id,provider)
);
create index canonical_metadata_fields_idx on canonical_metadata using gin(fields);
create table canonical_relationships (
 from_id uuid not null references canonical_entities(id), to_id uuid not null references canonical_entities(id),
 relation text not null, provider text not null, evidence text not null,
 retrieved_at timestamptz not null default now(),
 primary key(from_id,to_id,relation,provider)
);
create index canonical_relationships_to_idx on canonical_relationships(to_id);
create table identification_jobs (
 id uuid primary key default gen_random_uuid(), user_id uuid not null references users(id),
 item_id uuid references items(id) on delete set null,
 category text not null default '', input jsonb not null default '{}',
 state text not null default 'queued' check(state in ('queued','processing','review','failed','rejected','search_only','approved')),
 attempts integer not null default 0, lease_token uuid, lease_until timestamptz,
 available_at timestamptz not null default now(), result jsonb not null default '{}', error text not null default '',
 created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create index identification_jobs_claim_idx on identification_jobs(state,available_at,lease_until);
create unique index identification_jobs_active_item_idx on identification_jobs(item_id) where item_id is not null and state in ('queued','processing','review');
create table identification_photos (
 id uuid primary key, job_id uuid not null references identification_jobs(id),
 object_key text not null, thumbnail_key text not null, mime_type text not null,
 filename text not null, width integer not null, height integer not null, size_bytes bigint not null
);
create table item_canonical_links (
 item_id uuid not null references items(id) on delete cascade,
 entity_id uuid not null references canonical_entities(id), relation text not null,
 job_id uuid references identification_jobs(id), proposal_id uuid not null references proposals(id),
 confidence text not null, approved_at timestamptz not null default now(),
 primary key(item_id,entity_id,relation)
);
create index item_canonical_links_entity_idx on item_canonical_links(entity_id);
create table provider_rate_gates (provider text primary key, next_at timestamptz not null default now());
