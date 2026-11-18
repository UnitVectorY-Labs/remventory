package authority

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMusicReleaseLayers(t *testing.T) {
	raw := jsontext.Value(`{"id":"11111111-1111-4111-8111-111111111111","title":"Album pressing","date":"2015","barcode":"123","release-group":{"id":"22222222-2222-4222-8222-222222222222","title":"Album","first-release-date":"1999"},"artist-credit":[{"artist":{"id":"33333333-3333-4333-8333-333333333333","name":"Artist"}}],"media":[{"format":"12\" Vinyl","tracks":[{"position":1,"recording":{"id":"44444444-4444-4444-8444-444444444444","title":"Recording","relations":[{"type":"performance","work":{"id":"55555555-5555-4555-8555-555555555555","title":"Composition"}}]}}]}]}`)
	g, err := normalizeMB(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Entities) != 5 || len(g.Edges) != 5 {
		t.Fatalf("lost release/work layers: %+v", g)
	}
	if g.Entities[0].Fields["release_date"] != "2015" || g.Entities[1].Fields["original_date"] != "1999" {
		t.Fatal("conflated original and pressing dates")
	}
	for _, e := range g.Entities {
		if e.License != "CC0-1.0" {
			t.Fatal("license missing")
		}
	}
}
func TestWikidataHierarchyAndDeprecatedClaims(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query().Get("ids")
		if ids == "Q1" {
			w.Write([]byte(`{"entities":{"Q1":{"id":"Q1","labels":{"en":{"value":"Box"}},"claims":{"P527":[{"rank":"normal","mainsnak":{"datavalue":{"value":{"id":"Q2"}}}},{"rank":"deprecated","mainsnak":{"datavalue":{"value":{"id":"Q3"}}}}]}}}}`))
		} else {
			w.Write([]byte(`{"entities":{"Q2":{"id":"Q2","labels":{"en":{"value":"Film"}},"claims":{"P577":[{"mainsnak":{"datavalue":{"value":{"time":"+1999-01-01T00:00:00Z","precision":9}}}}]}}}}`))
		}
	}))
	defer srv.Close()
	p := Wikidata{BaseURL: srv.URL, HTTP: NewClient()}
	p.HTTP.Wait = func(context.Context, string) error { return nil }
	g, err := p.Resolve(context.Background(), Ref{"wikidata", "entity", "Q1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Entities) != 2 || len(g.Edges) != 1 || g.Edges[0].Relation != "contains" {
		t.Fatalf("bad hierarchy: %+v", g)
	}
}
func TestRejectUnvalidatedIdentifiers(t *testing.T) {
	ctx := context.Background()
	if _, err := (&MusicBrainz{}).Resolve(ctx, Ref{"musicbrainz", "release", "imaginary"}); err == nil {
		t.Fatal("invalid MBID accepted")
	}
	if _, err := (&Wikidata{}).Resolve(ctx, Ref{"wikidata", "entity", "https://evil.invalid"}); err == nil {
		t.Fatal("non-QID accepted")
	}
}
func TestLiveAuthorities(t *testing.T) {
	if os.Getenv("REMVENTORY_LIVE_PROVIDERS") != "1" {
		t.Skip("set REMVENTORY_LIVE_PROVIDERS=1 for rate-limited live API tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	r := New()
	mb := r["musicbrainz"]
	for _, q := range []Query{{Title: "Thriller", Artist: "Michael Jackson", Format: "Vinyl"}, {Title: "Kind of Blue", Artist: "Miles Davis", Format: "Vinyl"}} {
		c, err := mb.Search(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(c) < 2 {
			t.Fatalf("expected multiple pressings for %s, got %d", q.Title, len(c))
		}
		g, err := mb.Resolve(ctx, c[0].Ref)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.Entities) < 3 {
			t.Fatal("missing release graph")
		}
		t.Log(q.Title, c[0].ID, len(g.Entities), "entities")
	}
	wd := r["wikidata"]
	for _, q := range []string{"The Matrix", "The Lord of the Rings film trilogy", "Friends", "The Legend of Zelda: Breath of the Wild", "Nintendo Switch Pro Controller"} {
		c, err := wd.Search(ctx, Query{Text: q})
		if err != nil {
			t.Fatal(err)
		}
		if len(c) == 0 {
			t.Fatalf("no results: %s", q)
		}
		selected := c[0]
		for _, candidate := range c {
			if q == "The Matrix" && candidate.ID == "Q83495" {
				selected = candidate
			}
			if q == "Friends" && candidate.ID == "Q79784" {
				selected = candidate
			}
		}
		g, err := wd.Resolve(ctx, selected.Ref)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.Entities) == 0 {
			t.Fatal("empty graph")
		}
		t.Log(q, selected.ID, len(g.Entities), "entities", len(g.Edges), "edges; candidate relevance requires review")
	}
	c, err := wd.Search(ctx, Query{Text: "remventory-no-such-object-9d2d0f2b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 0 {
		t.Fatal("unexpected match for unknown object")
	}
}

func TestWikidataBatchesAnonymousEntityReads(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := strings.Split(r.URL.Query().Get("ids"), "|")
		if len(ids) > 50 {
			t.Errorf("anonymous entity batch exceeded 50: %d", len(ids))
		}
		calls++
		entities := map[string]any{}
		for _, id := range ids {
			entities[id] = map[string]any{"id": id}
		}
		json.MarshalWrite(w, map[string]any{"entities": entities})
	}))
	defer srv.Close()
	p := Wikidata{BaseURL: srv.URL, HTTP: NewClient()}
	p.HTTP.Wait = func(context.Context, string) error { return nil }
	ids := []string{}
	for i := 1; i <= 80; i++ {
		ids = append(ids, fmt.Sprintf("Q%d", i))
	}
	got, err := p.fetch(context.Background(), ids)
	if err != nil || len(got) != 80 || calls != 2 {
		t.Fatalf("batch read failed: %d entities, %d calls, %v", len(got), calls, err)
	}
}

func TestWikidataLanguageNeutralGameTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("languagefallback") != "1" || !strings.Contains(r.URL.Query().Get("languages"), "mul") {
			t.Error("entity requests must enable language fallback")
		}
		w.Write([]byte(`{"entities":{"Q96417649":{"id":"Q96417649","labels":{"mul":{"value":"Among Us"}},"descriptions":{"en":{"value":"2018 video game"}}}}}`))
	}))
	defer srv.Close()
	p := Wikidata{BaseURL: srv.URL, HTTP: NewClient()}
	g, err := p.Resolve(context.Background(), Ref{"wikidata", "entity", "Q96417649"})
	if err != nil {
		t.Fatal(err)
	}
	if got := RenderCard(g, DisplayPlan{}, false).Title; got != "Among Us" {
		t.Fatalf("title = %q", got)
	}
}
