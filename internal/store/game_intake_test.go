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
