package store

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/jackc/pgx/v5"
)

type CatalogueProposal struct {
	Operation string        `json:"operation"`
	ItemID    string        `json:"item_id"`
	EntityID  string        `json:"entity_id"`
	Relation  string        `json:"relation"`
	Label     string        `json:"label"`
	Ref       authority.Ref `json:"ref"`
}

func (s *Store) ProposeCatalogue(ctx context.Context, p CatalogueProposal) (Proposal, error) {
	if _, err := s.GetItem(ctx, p.ItemID); err != nil {
		return Proposal{}, err
	}
	if p.Operation != "link" && p.Operation != "unlink" {
		return Proposal{}, fmt.Errorf("choose link or unlink")
	}
	if p.Relation == "" {
		p.Relation = "identified_as"
	}
	if p.Relation != "identified_as" && p.Relation != "contains" && p.Relation != "associated_with" {
		return Proposal{}, fmt.Errorf("invalid catalogue relationship")
	}
	if p.Operation == "link" {
		id, err := s.CanonicalID(ctx, p.Ref)
		if err != nil {
			return Proposal{}, fmt.Errorf("resolve and validate the source record first: %w", err)
		}
		p.EntityID = id
	}
	if err := s.pool.QueryRow(ctx, `select label from canonical_entities where id=$1`, p.EntityID).Scan(&p.Label); err != nil {
		return Proposal{}, err
	}
	if p.Operation == "unlink" {
		var exists bool
		if err := s.pool.QueryRow(ctx, `select exists(select 1 from item_canonical_links where item_id=$1 and entity_id=$2 and relation=$3)`, p.ItemID, p.EntityID, p.Relation).Scan(&exists); err != nil {
			return Proposal{}, err
		}
		if !exists {
			return Proposal{}, fmt.Errorf("association no longer exists")
		}
	}
	return s.createProposal(ctx, DefaultUserID, "catalogue_change", p)
}
func applyCatalogue(ctx context.Context, tx pgx.Tx, proposal *Proposal) error {
	var p CatalogueProposal
	if err := json.Unmarshal(proposal.ProposedPayload, &p); err != nil {
		return err
	}
	var owner string
	if err := tx.QueryRow(ctx, `select user_id::text from items where id=$1 for update`, p.ItemID).Scan(&owner); err != nil {
		return err
	}
	if owner != proposal.UserID {
		return ErrNotFound
	}
	switch p.Operation {
	case "link":
		var id string
		if err := tx.QueryRow(ctx, `select entity_id::text from canonical_identifiers where provider=$1 and kind=$2 and external_id=$3`, p.Ref.Provider, p.Ref.Kind, p.Ref.ID).Scan(&id); err != nil || id != p.EntityID {
			return fmt.Errorf("source identity changed; review a new proposal")
		}
		_, err := tx.Exec(ctx, `insert into item_canonical_links(item_id,entity_id,relation,proposal_id,confidence) values($1,$2,$3,$4,'user_confirmed') on conflict do nothing`, p.ItemID, p.EntityID, p.Relation, proposal.ID)
		return err
	case "unlink":
		tag, err := tx.Exec(ctx, `delete from item_canonical_links where item_id=$1 and entity_id=$2 and relation=$3`, p.ItemID, p.EntityID, p.Relation)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("association changed; review again")
		}
		return nil
	}
	return fmt.Errorf("invalid catalogue operation")
}
