create table if not exists remy_conversations (
    id uuid primary key,
    turns jsonb not null default '[]'::jsonb,
    updated_at timestamptz not null default now()
);
