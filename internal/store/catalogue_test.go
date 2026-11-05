package store

import (
	"context"
	"encoding/json"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"testing"
	"time"
)

func TestManualCatalogueLifecyclePreservesOwnerData(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	var id string
	if err := s.pool.QueryRow(ctx, `insert into items(user_id,title,tags,attributes_jsonb) values($1,'My copy',array['Movies'],'{"title":"My copy","year":1999,"condition":"Mine"}') returning id`, DefaultUserID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	ref := authority.Ref{Provider: "wikidata", Kind: "entity", ID: "Q8337"}
	draft := CatalogueProposal{Operation: "link", ItemID: id, Ref: ref}
	if _, err := s.ProposeCatalogue(ctx, draft); err == nil {
		t.Fatal("unresolved identifier accepted")
	}
	g := authority.Graph{Root: ref, Entities: []authority.Entity{{Ref: ref, Label: "A verified work", Fields: map[string]any{"original_date": "1999"}, License: "CC0", RetrievedAt: time.Now()}}}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	p, err := s.ProposeCatalogue(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	links, _ := s.ItemCanonical(ctx, id)
	if len(links) != 0 {
		t.Fatal("proposal linked before confirmation")
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	links, _ = s.ItemCanonical(ctx, id)
	if len(links) != 1 {
		t.Fatal("manual link missing")
	}
	entityID := links[0]["id"].(string)
	unlink, err := s.ProposeCatalogue(ctx, CatalogueProposal{Operation: "unlink", ItemID: id, EntityID: entityID, Relation: "identified_as"})
	if err != nil {
		t.Fatal(err)
	}
	links, _ = s.ItemCanonical(ctx, id)
	if len(links) != 1 {
		t.Fatal("unlinked before confirmation")
	}
	if _, err = s.DecideProposal(ctx, unlink.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	links, _ = s.ItemCanonical(ctx, id)
	if len(links) != 0 {
		t.Fatal("unlink failed")
	}
	item, _ := s.GetItem(ctx, id)
	var attrs map[string]any
	json.Unmarshal(item.Attributes, &attrs)
	if attrs["year"] != float64(1999) || attrs["condition"] != "Mine" || item.Title != "My copy" {
		t.Fatal("manual management changed owner data")
	}
	if _, _, err = s.CachedGraph(ctx, ref); err != nil {
		t.Fatal("unlink removed cached source graph")
	}
}
func TestRuntimeTypePolicySnapshotsAndDefaults(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	policy := IntakeConfig{Providers: []string{"wikidata"}, Prompt: "Use the title visible on the spine", DisplayFields: map[string][]string{"wikidata": {"original_dates"}}}
	proposal, err := s.CreateCategoryProposal(ctx, CategoryProposalPayload{Name: "LaserDisc", DefaultTags: []string{"LaserDisc"}, DefaultValues: json.RawMessage(`{"format":"LaserDisc"}`), IntakeConfig: &policy, Attributes: []AttributeDraft{{Key: "format"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, proposal.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	profiles, _ := s.ListCategories(ctx, 200, 0)
	var profile Category
	for _, p := range profiles {
		if p.Name == "LaserDisc" {
			profile = p
		}
	}
	if profile.IntakeConfig.Prompt != policy.Prompt {
		t.Fatal("policy did not persist")
	}
	j, err := s.CreateIdentification(ctx, "ignored", "", IdentificationInput{TemplateID: profile.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if j.Category != "LaserDisc" || j.Input.Policy == nil || j.Input.Policy.Allows("musicbrainz") || !j.Input.Policy.Allows("wikidata") {
		t.Fatal("job did not use runtime policy", j)
	}
	edit, err := s.CreateCategoryProposal(ctx, CategoryProposalPayload{Operation: "update", CategoryID: profile.ID, DefaultValues: json.RawMessage(`{"format":"Changed"}`), DefaultTags: []string{"Changed"}, IntakeConfig: &IntakeConfig{Providers: []string{}, Prompt: "Changed"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, edit.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.Identification(ctx, j.ID)
	if saved.Input.Policy.Prompt != policy.Prompt {
		t.Fatal("queued job policy changed retroactively")
	}
	claimed, _ := s.ClaimIdentification(ctx)
	if err = s.FinishIdentification(ctx, claimed, IdentificationResult{Title: "Owner copy"}, nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.ProposeIdentification(ctx, IdentificationProposal{JobID: j.ID, Title: "Owner copy", Attributes: json.RawMessage(`{"title":"Owner copy"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	done, _ := s.Identification(ctx, j.ID)
	item, _ := s.GetItem(ctx, done.ItemID)
	var attrs map[string]any
	json.Unmarshal(item.Attributes, &attrs)
	if attrs["format"] != "LaserDisc" || len(item.Tags) != 1 || item.Tags[0] != "LaserDisc" || item.CategoryID != "" {
		t.Fatal("capture defaults diverged from manual template defaults", item)
	}
}
