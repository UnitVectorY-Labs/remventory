package store

import (
	"context"
	"encoding/json"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"strings"
	"testing"
	"time"
)

func TestGameIntakeRequiresOwnedConsole(t *testing.T) {
	job := IdentificationJob{Input: IdentificationInput{FieldKeys: []string{"title", "console"}}}
	for _, raw := range []string{`{}`, `{"console":""}`, `{"console":["PS5","Nintendo Switch"]}`} {
		if validateIdentificationConsole(job, json.RawMessage(raw)) == nil {
			t.Errorf("accepted missing or ambiguous console: %s", raw)
		}
	}
	if err := validateIdentificationConsole(job, json.RawMessage(`{"console":"Nintendo Switch 2"}`)); err != nil {
		t.Fatal(err)
	}
	if err := validateIdentificationConsole(IdentificationJob{}, json.RawMessage(`{}`)); err != nil {
		t.Fatal("unrelated intake must remain optional")
	}
}

func TestExtendEnumRequiresApprovalAndPreservesChoices(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	p, err := s.CreateGraphProposal(ctx, GraphProposalPayload{Operation: "define_field", Key: "console", Label: "Console", DataType: "enum", Config: json.RawMessage(`{"options":["PS5"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	var pending []Proposal
	for _, option := range []string{"Nintendo Switch", "Nintendo Switch 2"} {
		p, err = s.CreateGraphProposal(ctx, GraphProposalPayload{Operation: "extend_enum", Key: "console", Config: json.RawMessage(`{"options":["` + option + `"]}`)})
		if err != nil {
			t.Fatal(err)
		}
		var before string
		if err = s.pool.QueryRow(ctx, `select config::text from field_definitions where key='console'`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(before, option) {
			t.Fatal("pending proposal changed choices")
		}
		pending = append(pending, p)
	}
	for _, proposal := range pending {
		if _, err = s.DecideProposal(ctx, proposal.ID, ProposalDecision{Approve: true}); err != nil {
			t.Fatal(err)
		}
	}
	var config struct {
		Options []string `json:"options"`
	}
	var raw []byte
	if err = s.pool.QueryRow(ctx, `select config from field_definitions where key='console'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Options) != 3 || config.Options[0] != "PS5" {
		t.Fatalf("choices overwritten: %s", raw)
	}
}

func TestBlankCachedTitleNeedsRefresh(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	ref := authority.Ref{Provider: "wikidata", Kind: "entity", ID: "Q96417649"}
	g := authority.Graph{Root: ref, Entities: []authority.Entity{{Ref: ref, URI: "https://www.wikidata.org/wiki/Q96417649", Fields: map[string]any{}, License: "CC0-1.0", RetrievedAt: time.Now()}}}
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := s.CachedGraph(ctx, ref); err != nil || fresh {
		t.Fatalf("blank cached title marked fresh: %v, %v", fresh, err)
	}
	g.Entities[0].Label = "Among Us"
	if err := s.CacheGraph(ctx, g); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := s.CachedGraph(ctx, ref); err != nil || !fresh {
		t.Fatalf("repaired title not fresh: %v, %v", fresh, err)
	}
}

func TestConfirmedGameTagsRepairLegacyDefaults(t *testing.T) {
	job := IdentificationJob{Input: IdentificationInput{FieldKeys: []string{"console"}}}
	got := identificationTags(job, []string{"Games", "Consoles", " Games ", "consoles", "PS3", "ps3", "Favorites"}, json.RawMessage(`{"console":"PS3"}`))
	if strings.Join(got, ",") != "Games,PS3,Favorites" {
		t.Fatalf("tags: %v", got)
	}
	got = identificationTags(job, []string{"Games", "Consoles", "Games"}, json.RawMessage(`{"console":"Nintendo Switch 2"}`))
	if strings.Join(got, ",") != "Games,Nintendo Switch 2" {
		t.Fatalf("tags: %v", got)
	}
	got = identificationTags(IdentificationJob{}, []string{"Consoles", "Hardware"}, json.RawMessage(`{}`))
	if strings.Join(got, ",") != "Consoles,Hardware" {
		t.Fatalf("unrelated tags changed: %v", got)
	}
}

func TestLegacyPendingGameGetsConsoleTagOnlyOnApproval(t *testing.T) {
	s := canonicalTestStore(t)
	ctx := context.Background()
	p, err := s.CreateGraphProposal(ctx, GraphProposalPayload{Operation: "define_field", Key: "console", Label: "Console", DataType: "enum", Config: json.RawMessage(`{"options":["PS3"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	job, err := s.CreateIdentification(ctx, "Video Game", "", IdentificationInput{Policy: &IntakeConfig{Providers: []string{"wikidata"}}, FieldKeys: []string{"title", "console"}, DefaultTags: []string{"Games", "Consoles"}, DefaultValues: json.RawMessage(`{}`)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `update identification_jobs set state='review' where id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	p, err = s.ProposeIdentification(ctx, IdentificationProposal{JobID: job.ID, Title: "Skyrim — Legendary Edition", Tags: []string{"Games", "Consoles"}, Attributes: json.RawMessage(`{"title":"Skyrim — Legendary Edition","console":"PS3"}`)})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Identification(ctx, job.ID)
	if err != nil || before.State != "review" || before.ItemID != "" {
		t.Fatalf("proposal changed pending state: %v, %v", before, err)
	}
	if _, err = s.DecideProposal(ctx, p.ID, ProposalDecision{Approve: true}); err != nil {
		t.Fatal(err)
	}
	after, err := s.Identification(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.GetItem(ctx, after.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(item.Tags, ",") != "Games,PS3" || item.Title != "Skyrim — Legendary Edition" {
		t.Fatalf("saved game: %+v", item)
	}
}
