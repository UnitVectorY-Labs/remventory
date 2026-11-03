package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func canonicalTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for isolated Postgres integration")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		t.Fatal("canonical tests require local Postgres")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "canonical_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = db.Exec(ctx, `create schema `+schema); err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(); db.Exec(ctx, `drop schema `+schema+` cascade`); db.Close(ctx) })
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.EnsureDefaultUser(ctx, "Tests"); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCanonicalApprovalDatesAndOwnership(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	root := authority.Ref{Provider: "test", Kind: "release", ID: "retail-2015"}
	film := authority.Ref{Provider: "test", Kind: "film", ID: "film-1999"}
	director := authority.Ref{Provider: "test", Kind: "person", ID: "director"}
	other := authority.Ref{Provider: "test", Kind: "film", ID: "unowned"}
	entity := func(ref authority.Ref, label string, fields map[string]any) authority.Entity {
		return authority.Entity{Ref: ref, Label: label, Fields: fields, License: "test", RetrievedAt: time.Now()}
	}
	g := authority.Graph{Root: root, Entities: []authority.Entity{entity(root, "2015 Box", map[string]any{"release_date": "2015"}), entity(film, "1999 Movie", map[string]any{"original_dates": []any{map[string]any{"time": "+1999-01-01T00:00:00Z"}, map[string]any{"time": "+2015-01-01T00:00:00Z"}}}), entity(director, "Director", map[string]any{}), entity(other, "Unowned Film", map[string]any{"original_date": "1999"})}, Edges: []authority.Edge{{From: root, To: film, Relation: "contains"}, {From: film, To: director, Relation: "directed_by"}, {From: other, To: director, Relation: "directed_by"}}}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal("idempotent cache", err)
	}
	var count int
	s.pool.QueryRow(ctx, `select count(*) from canonical_entities`).Scan(&count)
	if count != 4 {
		t.Fatal("duplicate canonical identities")
	}
	j, err := s.CreateIdentification(ctx, "Blu-ray", "", IdentificationInput{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimIdentification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimIdentification(ctx); err == nil {
		t.Fatal("claimed job twice")
	}
	result := IdentificationResult{Title: "My box", Matches: []IdentificationMatch{{Graph: g, Confidence: "partial"}}}
	if err = s.FinishIdentification(ctx, claimed, result, nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: j.ID, Selected: []int{0}, Title: "My physical box", Tags: []string{"Blu-ray"}, Attributes: json.RawMessage(`{"title":"My physical box"}`)})
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.GraphSearch(ctx, "", "film", "1999", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatal("unapproved source cache created ownership")
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	items, err = s.GraphSearch(ctx, "", "film", "1999", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "My physical box" {
		t.Fatalf("original-year traversal failed: %+v", items)
	}
	facts, e := s.CanonicalEvidence(ctx, items)
	if e != nil {
		t.Fatal(e)
	}
	explained := false
	for _, fact := range facts {
		if fact["entity"] == "1999 Movie" && strings.HasPrefix(fmt.Sprint(fact["original_date"]), "1999") {
			explained = true
		}
	}
	if !explained {
		t.Fatal("owned item search omitted supporting work/date evidence", facts)
	}
	items, err = s.GraphSearch(ctx, "", "film", "2015", "")
	if err != nil || len(items) != 0 {
		t.Fatal("later publication date incorrectly treated as original year", err)
	}
	for _, filters := range [][2]string{{"movies", ""}, {"", "movie"}, {"1999", "film"}} {
		found, e := s.GraphSearch(ctx, filters[0], filters[1], "1999", "")
		if e != nil || len(found) != 1 {
			t.Fatalf("natural movie/year filters lost owned work: %v, %v", filters, e)
		}
	}
	items, err = s.GraphSearch(ctx, "Unowned Film", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatal("shared director incorrectly implies ownership")
	}
	items, err = s.GraphSearch(ctx, "Director", "", "", "")
	if err != nil || len(items) != 1 {
		t.Fatal("creator lookup failed", err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err == nil {
		t.Fatal("replayed approval")
	}
	_, err = s.GraphSearch(ctx, "", "", "1999\" bad", "")
	if err == nil {
		t.Fatal("invalid year accepted")
	}
}
func TestJobLeaseFencingAndOriginalFields(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	var itemID string
	if err := s.pool.QueryRow(ctx, `insert into items(user_id,title,tags,attributes_jsonb) values($1,'Original',array['Vinyl'],'{"title":"Original","condition":"Mine"}') returning id`, DefaultUserID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	j, err := s.CreateIdentification(ctx, "Vinyl", itemID, IdentificationInput{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.ClaimIdentification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.pool.Exec(ctx, `update identification_jobs set lease_until=now()-interval '1 second' where id=$1`, j.ID)
	second, err := s.ClaimIdentification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishIdentification(ctx, first, IdentificationResult{}, nil); err == nil {
		t.Fatal("stale worker overwrote new lease")
	}
	if err = s.FinishIdentification(ctx, second, IdentificationResult{Title: "External title"}, nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: j.ID, Selected: []int{}, Title: "Replacement"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Original" || string(item.Attributes) != `{"title": "Original", "condition": "Mine"}` {
		var v map[string]any
		json.Unmarshal(item.Attributes, &v)
		if item.Title != "Original" || v["condition"] != "Mine" {
			t.Fatal("overwrote original item", item)
		}
	}
}

func TestPartialItemUpdatesPreserveOmittedFields(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	var id string
	err := s.pool.QueryRow(ctx, `insert into items(user_id,title,tags,attributes_jsonb) values($1,'Original',array['Movies'],'{"title":"Original","condition":"Mine","year":1999}') returning id`, DefaultUserID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, Tags: []string{"Movies", "Box set"}, Attributes: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	item, err := s.GetItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	json.Unmarshal(item.Attributes, &fields)
	if item.Title != "Original" || fields["condition"] != "Mine" || fields["year"] != float64(1999) {
		t.Fatal("tag-only update replaced personal fields", item)
	}
	stale, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, Attributes: json.RawMessage(`{"year":2000}`)})
	if err != nil {
		t.Fatal(err)
	}
	removal, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, RemoveFields: []string{"condition"}, Attributes: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, removal.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	item, err = s.GetItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(item.Attributes, &fields)
	var remaining map[string]any
	json.Unmarshal(item.Attributes, &remaining)
	if _, ok := remaining["condition"]; ok {
		t.Fatal("explicit field removal ignored")
	}
	if remaining["title"] != "Original" {
		t.Fatal("explicit removal affected another field")
	}
	if _, err = s.DecideProposal(ctx, stale.ID, ProposalDecision{Approve: true}); err == nil {
		t.Fatal("stale proposal overwrote a newer edit")
	}
}
