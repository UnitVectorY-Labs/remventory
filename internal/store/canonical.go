package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/jackc/pgx/v5"
	"regexp"
	"time"
)

// ProviderWait reserves a slot across replicas; no HTTP call holds a transaction.
func (s *Store) ProviderWait(ctx context.Context, provider string) error {
	var next time.Time
	err := s.pool.QueryRow(ctx, `insert into provider_rate_gates(provider,next_at) values($1,now()+interval '1200 milliseconds') on conflict(provider) do update set next_at=greatest(provider_rate_gates.next_at,now())+interval '1200 milliseconds' returning next_at-interval '1200 milliseconds'`, provider).Scan(&next)
	if err != nil {
		return err
	}
	timer := time.NewTimer(max(time.Duration(0), time.Until(next)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (s *Store) CacheGraph(ctx context.Context, g authority.Graph) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ids := map[string]string{}
	for _, e := range g.Entities {
		// Serialize identity creation even when two workers resolve the same identifier.
		if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(77626109)`); err != nil {
			return err
		}
		var id string
		err = tx.QueryRow(ctx, `select entity_id from canonical_identifiers where provider=$1 and kind=$2 and external_id=$3`, e.Provider, e.Kind, e.ID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `insert into canonical_entities(kind,label) values($1,$2) returning id`, e.Kind, e.Label).Scan(&id)
			if err == nil {
				_, err = tx.Exec(ctx, `insert into canonical_identifiers(provider,kind,external_id,entity_id,uri) values($1,$2,$3,$4,$5)`, e.Provider, e.Kind, e.ID, id, e.URI)
			}
		}
		if err != nil {
			return err
		}
		ids[e.Key()] = id
		if _, err = tx.Exec(ctx, `update canonical_entities set label=$2 where id=$1`, id, e.Label); err != nil {
			return err
		}
		fields, _ := json.Marshal(e.Fields)
		raw := e.Raw
		if len(raw) == 0 {
			raw = []byte("null")
		}
		_, err = tx.Exec(ctx, `insert into canonical_metadata(entity_id,provider,fields,raw,description,license,retrieved_at,refresh_after) values($1,$2,$3,$4,$5,$6,$7,$7::timestamptz+interval '90 days') on conflict(entity_id,provider) do update set fields=excluded.fields,raw=excluded.raw,description=excluded.description,retrieved_at=excluded.retrieved_at,refresh_after=excluded.refresh_after,refresh_status='current'`, id, e.Provider, fields, raw, e.Description, e.License, e.RetrievedAt)
		if err != nil {
			return err
		}
	}
	// Replace outgoing source assertions only for expanded entities in this response.
	froms := map[string]bool{}
	for _, ref := range g.Expanded {
		froms[ref.Key()] = true
	}
	for _, edge := range g.Edges {
		froms[edge.From.Key()] = true
	}
	for key := range froms {
		_, err = tx.Exec(ctx, `delete from canonical_relationships where from_id=$1 and provider=$2`, ids[key], g.Root.Provider)
		if err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		from, to := ids[e.From.Key()], ids[e.To.Key()]
		if from == "" || to == "" {
			return fmt.Errorf("edge endpoint missing")
		}
		_, err = tx.Exec(ctx, `insert into canonical_relationships(from_id,to_id,relation,provider,evidence) values($1,$2,$3,$4,$5) on conflict do nothing`, from, to, e.Relation, e.From.Provider, e.Evidence)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) CachedGraph(ctx context.Context, ref authority.Ref) (authority.Graph, bool, error) {
	var id string
	var fresh bool
	err := s.pool.QueryRow(ctx, `select i.entity_id,m.refresh_after>now() from canonical_identifiers i join canonical_metadata m on m.entity_id=i.entity_id and m.provider=i.provider where i.provider=$1 and i.kind=$2 and i.external_id=$3`, ref.Provider, ref.Kind, ref.ID).Scan(&id, &fresh)
	if errors.Is(err, pgx.ErrNoRows) {
		return authority.Graph{}, false, nil
	}
	if err != nil {
		return authority.Graph{}, false, err
	}
	doc, err := s.EntityDetail(ctx, id)
	if err != nil {
		return authority.Graph{}, false, err
	}
	return doc.Graph, fresh, nil
}

type EntityDocument struct {
	ID     string          `json:"id"`
	Graph  authority.Graph `json:"graph"`
	Owners []Item          `json:"owners"`
}

const reachSQL = `with recursive reach(id,depth,path) as (
 select $1::uuid,0,array[$1::uuid]
 union all select e.to_id,r.depth+1,r.path||e.to_id from reach r join canonical_relationships e on e.from_id=r.id
 where r.depth<4 and not e.to_id=any(r.path)
) `

func (s *Store) EntityDetail(ctx context.Context, id string) (EntityDocument, error) {
	d := EntityDocument{ID: id, Graph: authority.Graph{Entities: []authority.Entity{}, Edges: []authority.Edge{}}}
	rows, err := s.pool.Query(ctx, reachSQL+`select distinct c.id,i.provider,i.kind,i.external_id,c.label,i.uri,m.description,m.fields,m.license,m.retrieved_at from reach r join canonical_entities c on c.id=r.id join canonical_identifiers i on i.entity_id=c.id join canonical_metadata m on m.entity_id=c.id and m.provider=i.provider limit 300`, id)
	if err != nil {
		return d, err
	}
	refs := map[string]authority.Ref{}
	for rows.Next() {
		var cid string
		var e authority.Entity
		if err = rows.Scan(&cid, &e.Provider, &e.Kind, &e.ID, &e.Label, &e.URI, &e.Description, &e.Fields, &e.License, &e.RetrievedAt); err != nil {
			rows.Close()
			return d, err
		}
		refs[cid] = e.Ref
		if cid == id {
			d.Graph.Root = e.Ref
			d.Graph.Association = authority.Association(e)
		}
		d.Graph.Entities = append(d.Graph.Entities, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	if len(d.Graph.Entities) == 0 {
		return d, ErrNotFound
	}
	rows, err = s.pool.Query(ctx, reachSQL+`select distinct e.from_id,e.to_id,e.relation,e.evidence from canonical_relationships e where e.from_id in(select id from reach) and e.to_id in(select id from reach)`, id)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var from, to, rel, ev string
		if err = rows.Scan(&from, &to, &rel, &ev); err != nil {
			rows.Close()
			return d, err
		}
		if _, ok := refs[from]; ok {
			if _, ok := refs[to]; ok {
				d.Graph.Edges = append(d.Graph.Edges, authority.Edge{From: refs[from], To: refs[to], Relation: rel, Evidence: ev})
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	d.Owners, err = s.GraphSearch(ctx, "", "", "", id)
	return d, err
}

// Ownership paths follow only semantic containment/version edges. Shared artists,
// directors and platforms must never imply ownership of their other works.
const ownershipRelations = `('contains','release_of','version_of','performance_of')`

func (s *Store) GraphSearch(ctx context.Context, text, kind, year, entityID string) ([]Item, error) {
	if year != "" && !regexp.MustCompile(`^[0-9]{4}$`).MatchString(year) {
		return nil, fmt.Errorf("year must have four digits")
	}
	rows, err := s.pool.Query(ctx, `with recursive owned(item_id,entity_id,depth,path) as (
 select item_id,entity_id,0,array[entity_id] from item_canonical_links
 union all select o.item_id,e.to_id,o.depth+1,o.path||e.to_id from owned o join canonical_relationships e on e.from_id=o.entity_id where o.depth<6 and not e.to_id=any(o.path) and e.relation in `+ownershipRelations+`
 ) select distinct i.id,i.user_id,coalesce(i.category_id::text,'') as category_id,i.title,i.attributes_jsonb as attributes,i.quantity,i.created_at,i.updated_at,i.tags,i.location from items i join owned o on o.item_id=i.id join canonical_entities c on c.id=o.entity_id join canonical_metadata m on m.entity_id=c.id
 where i.user_id=$5 and ($1='' or lower(c.label||' '||m.description||' '||m.fields::text) like '%'||lower($1)||'%' or exists(select 1 from canonical_relationships e join canonical_entities target on target.id=e.to_id where e.from_id=c.id and e.relation in ('by_artist','by_author','directed_by','part_of_series','part_of','platform','compatible_with') and lower(target.label) like '%'||lower($1)||'%'))
 and ($2='' or c.kind=$2 or lower(m.description) like '%'||lower($2)||'%')
 and ($3='' or coalesce(nullif(m.fields->>'original_date',''),(select min(ltrim(date_value->>'time','+')) from jsonb_array_elements(coalesce(m.fields->'original_dates','[]'::jsonb)) date_value)) like $3||'%')
 and ($4='' or c.id=nullif($4,'')::uuid) order by i.title limit 100`, text, kind, year, entityID, DefaultUserID)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Item])
	if err != nil {
		return nil, err
	}
	return items, s.HydrateItemImages(ctx, items)
}
func (s *Store) ItemCanonical(ctx context.Context, itemID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `select c.id,c.label,c.kind,l.relation,l.confidence,i.uri,i.provider,i.external_id,l.approved_at from item_canonical_links l join canonical_entities c on c.id=l.entity_id join canonical_identifiers i on i.entity_id=c.id where l.item_id=$1 order by c.label`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, label, kind, rel, conf, uri, provider, external string
		var at time.Time
		if err = rows.Scan(&id, &label, &kind, &rel, &conf, &uri, &provider, &external, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "label": label, "kind": kind, "relation": rel, "confidence": conf, "uri": uri, "provider": provider, "external_id": external, "approved_at": at})
	}
	return out, rows.Err()
}

func (s *Store) CanonicalID(ctx context.Context, ref authority.Ref) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `select entity_id from canonical_identifiers where provider=$1 and kind=$2 and external_id=$3`, ref.Provider, ref.Kind, ref.ID).Scan(&id)
	return id, err
}
