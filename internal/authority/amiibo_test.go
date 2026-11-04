package authority

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAmiiboVariantIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"amiibo":[{"head":"00000000","tail":"00340102","name":"Mario","character":"Mario","amiiboSeries":"Super Mario Bros.","gameSeries":"Super Mario","type":"Figure","release":{"na":"2015-03-20"}},{"head":"00000000","tail":"00000002","name":"Mario","character":"Mario","amiiboSeries":"Super Smash Bros.","gameSeries":"Super Mario","type":"Figure","release":{"na":"2014-11-21"}}]}`))
	}))
	defer server.Close()
	p := Amiibo{BaseURL: server.URL, HTTP: NewClient()}
	p.HTTP.Wait = func(context.Context, string) error { return nil }
	c, err := p.Search(context.Background(), Query{Text: "Mario"})
	if err != nil || len(c) != 2 || c[0].ID == c[1].ID {
		t.Fatal("figure editions conflated", c, err)
	}
	g, err := p.Resolve(context.Background(), c[1].Ref)
	if err != nil {
		t.Fatal(err)
	}
	e := RootEntity(g)
	if e.ID != "0000000000000002" || e.Fields["amiibo_series"] != "Super Smash Bros." {
		t.Fatal("wrong figure identity", g)
	}
	if _, err = p.Resolve(context.Background(), Ref{"amiibo", "figure", "made-up"}); err == nil {
		t.Fatal("invented identifier accepted")
	}
}
func TestDisplayPlanCannotInventFacts(t *testing.T) {
	g := Graph{Root: Ref{"amiibo", "figure", "0000000000000002"}, Entities: []Entity{{Ref: Ref{"amiibo", "figure", "0000000000000002"}, Label: "Mario", Fields: map[string]any{"character": "Mario", "amiibo_series": "Super Smash Bros.", "release_dates": map[string]any{"na": "2014-11-21"}}}}}
	p := CuratePlan(g, DisplayPlan{Layout: "<script>", Fields: []string{"price", "amiibo_series", "release_dates.na", "amiibo_series"}})
	c := RenderCard(g, p, true)
	if c.Title != "Mario" || c.Subtitle != "Super Smash Bros." || len(c.Facts) != 1 || c.Facts[0].Value != "2014-11-21" || c.Layout != "identity" {
		t.Fatal("presentation introduced unverified data", c)
	}
	original := DisplayFingerprint(g)
	g.Entities[0].Fields["amiibo_series"] = "Super Mario Bros."
	if DisplayFingerprint(g) == original {
		t.Fatal("source edit did not invalidate presentation")
	}
}
func TestLiveAmiibo(t *testing.T) {
	if os.Getenv("REMVENTORY_LIVE_PROVIDERS") != "1" {
		t.Skip("opt-in live AmiiboAPI integration")
	}
	p := Amiibo{HTTP: NewClient()}
	c, err := p.Search(context.Background(), Query{Text: "Mario"})
	if err != nil || len(c) < 2 {
		t.Fatal("expected distinct Mario figures", err)
	}
	g, err := p.Resolve(context.Background(), Ref{"amiibo", "figure", "0000000000000002"})
	if err != nil {
		t.Fatal(err)
	}
	root := RootEntity(g)
	if root.Fields["character"] != "Mario" || root.Fields["amiibo_series"] != "Super Smash Bros." {
		t.Fatal("unexpected live variant", root)
	}
	t.Log("Verified", root.Label, root.Fields["amiibo_series"], root.ID)
}
