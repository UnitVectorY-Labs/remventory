package store

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
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
	p, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: j.ID, Selected: []int{0}, Title: "My physical box", Tags: []string{"Blu-ray"}, Attributes: jsontext.Value(`{"title":"My physical box"}`)})
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
	p, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, Tags: []string{"Movies", "Box set"}, Attributes: jsontext.Value(`{}`)})
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
	stale, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, Attributes: jsontext.Value(`{"year":2000}`)})
	if err != nil {
		t.Fatal(err)
	}
	removal, err := s.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: id, RemoveFields: []string{"condition"}, Attributes: jsontext.Value(`{}`)})
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

func TestCuratedPresentationCache(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	ref := authority.Ref{Provider: "amiibo", Kind: "figure", ID: "0000000000000002"}
	g := authority.Graph{Root: ref, Association: "identified_as", Entities: []authority.Entity{{Ref: ref, Label: "Mario", Fields: map[string]any{"character": "Mario", "amiibo_series": "Super Smash Bros."}, License: "MIT", RetrievedAt: time.Now()}}}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	p := authority.DisplayPlan{Layout: "identity", Fields: []string{"amiibo_series", "character", "invented"}}
	if err := s.SaveDisplay(ctx, g, p, "test-model"); err != nil {
		t.Fatal(err)
	}
	saved, found, err := s.Display(ctx, g)
	if err != nil || !found || len(saved.Fields) != 2 {
		t.Fatal("curation not cached/validated", saved, found, err)
	}
	if err := s.SaveDisplay(ctx, g, authority.DisplayPlan{Fields: []string{"character"}}, "second-model"); err != nil {
		t.Fatal(err)
	}
	saved, _, _ = s.Display(ctx, g)
	if len(saved.Fields) != 2 {
		t.Fatal("unchanged source unnecessarily changed the cached presentation")
	}
	g.Entities[0].Fields["amiibo_series"] = "Super Mario Bros."
	if _, found, err := s.Display(ctx, g); err != nil || found {
		t.Fatal("changed source must invalidate presentation", err)
	}
}

func TestHumanCanConfirmSourceValidatedAlternative(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	ref := authority.Ref{Provider: "amiibo", Kind: "figure", ID: "0000000000340102"}
	g := authority.Graph{Root: ref, Association: "identified_as", Entities: []authority.Entity{{Ref: ref, Label: "Mario", Fields: map[string]any{"amiibo_series": "Super Mario Bros."}, License: "MIT", RetrievedAt: time.Now()}}}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	j, err := s.CreateIdentification(ctx, "Amiibo", "", IdentificationInput{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimIdentification(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Matches: []IdentificationMatch{{Graph: g, Confidence: "rejected"}}}, nil); err != nil {
		t.Fatal(err)
	}
	draft := IdentificationProposal{JobID: j.ID, Title: "Mario — Super Mario Bros. amiibo", Selected: []int{0}, Attributes: jsontext.Value(`{"title":"Mario — Super Mario Bros. amiibo"}`), Tags: []string{"Amiibo"}}
	if _, err = s.ProposeIdentification(ctx, draft); err == nil {
		t.Fatal("automated rejection silently bypassed")
	}
	draft.ConfirmAlternative = true
	p, err := s.ProposeIdentification(ctx, draft)
	if err != nil {
		t.Fatal("explicit human correction was blocked", err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	var confidence string
	if err = s.pool.QueryRow(ctx, `select confidence from item_canonical_links where proposal_id=$1`, p.ID).Scan(&confidence); err != nil || confidence != "user_confirmed" {
		t.Fatal("human correction provenance missing", confidence, err)
	}
}

func TestReplacementRequiresApprovalAndPreservesOwnedFields(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	var id string
	if err := s.pool.QueryRow(ctx, `insert into items(user_id,title,tags,attributes_jsonb) values($1,'My Mario',array['Amiibo'],'{"title":"My Mario","condition":"Mine"}') returning id`, DefaultUserID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	makeProposal := func(externalID string, replace bool) Proposal {
		t.Helper()
		ref := authority.Ref{Provider: "amiibo", Kind: "figure", ID: externalID}
		g := authority.Graph{Root: ref, Association: "identified_as", Entities: []authority.Entity{{Ref: ref, Label: externalID, Fields: map[string]any{}, License: "MIT", RetrievedAt: time.Now()}}}
		if err := s.CacheGraph(ctx, g); err != nil {
			t.Fatal(err)
		}
		j, err := s.CreateIdentification(ctx, "Amiibo", id, IdentificationInput{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := s.ClaimIdentification(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Matches: []IdentificationMatch{{Graph: g, Confidence: "confident"}}}, nil); err != nil {
			t.Fatal(err)
		}
		p, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: j.ID, Selected: []int{0}, ReplaceExisting: replace})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := makeProposal("original", false)
	statuses, err := s.IdentificationStatuses(ctx, []Item{{ID: id}})
	if err != nil || statuses[id] != "review" {
		t.Fatal(statuses, err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	replacement := makeProposal("corrected", true)
	var count int
	s.pool.QueryRow(ctx, `select count(*) from item_canonical_links where item_id=$1`, id).Scan(&count)
	if count != 1 {
		t.Fatal("proposal mutated links")
	}
	if _, err = s.DecideProposal(ctx, replacement.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	var externalID string
	if err = s.pool.QueryRow(ctx, `select external_id from canonical_identifiers c join item_canonical_links l on l.entity_id=c.entity_id where l.item_id=$1`, id).Scan(&externalID); err != nil || externalID != "corrected" {
		t.Fatal(externalID, err)
	}
	item, _ := s.GetItem(ctx, id)
	var attrs map[string]any
	json.Unmarshal(item.Attributes, &attrs)
	if item.Title != "My Mario" || attrs["condition"] != "Mine" {
		t.Fatal("replacement changed owned fields")
	}
	statuses, err = s.IdentificationStatuses(ctx, []Item{{ID: id}})
	if err != nil || statuses[id] != "identified" {
		t.Fatal(statuses, err)
	}
	// A different confirmation between proposing and approving must not be discarded.
	stale := makeProposal("stale", true)
	_, err = s.pool.Exec(ctx, `insert into item_canonical_links(item_id,entity_id,relation,proposal_id,confidence) select $1,entity_id,'contains',$2,'confident' from canonical_identifiers where external_id='original'`, id, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, stale.ID, ProposalDecision{Approve: true}); err == nil {
		t.Fatal("stale replacement removed newer links")
	}
}
