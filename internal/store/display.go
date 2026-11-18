package store

import (
	"context"
	"encoding/json/v2"
	"errors"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/jackc/pgx/v5"
)

func (s *Store) Display(ctx context.Context, g authority.Graph) (authority.DisplayPlan, bool, error) {
	var p authority.DisplayPlan
	err := s.pool.QueryRow(ctx, `select p.plan from canonical_presentations p join canonical_identifiers i on i.entity_id=p.entity_id and i.provider=p.provider where i.provider=$1 and i.kind=$2 and i.external_id=$3 and p.source_fingerprint=$4`, g.Root.Provider, g.Root.Kind, g.Root.ID, authority.DisplayFingerprint(g)).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, false, nil
	}
	return p, err == nil, err
}
func (s *Store) SaveDisplay(ctx context.Context, g authority.Graph, p authority.DisplayPlan, model string) error {
	b, _ := json.Marshal(authority.CuratePlan(g, p))
	_, err := s.pool.Exec(ctx, `insert into canonical_presentations(entity_id,provider,source_fingerprint,plan,model) select entity_id,provider,$4,$5,$6 from canonical_identifiers where provider=$1 and kind=$2 and external_id=$3 on conflict(entity_id,provider) do update set source_fingerprint=excluded.source_fingerprint,plan=excluded.plan,model=excluded.model,created_at=now() where canonical_presentations.source_fingerprint<>excluded.source_fingerprint`, g.Root.Provider, g.Root.Kind, g.Root.ID, authority.DisplayFingerprint(g), b, model)
	return err
}
func (s *Store) ActiveIdentification(ctx context.Context, itemID string) (IdentificationJob, error) {
	return scanJob(s.pool.QueryRow(ctx, `select `+jobColumns+` from identification_jobs where item_id=$1 and state in ('queued','processing','review') order by created_at desc limit 1`, itemID))
}
