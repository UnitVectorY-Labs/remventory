package intake

import (
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

func consoleGraph(platforms ...string) authority.Graph {
	root := authority.Ref{Provider: "wikidata", Kind: "entity", ID: "game"}
	g := authority.Graph{Root: root}
	for _, platform := range platforms {
		ref := authority.Ref{Provider: "wikidata", Kind: "entity", ID: platform}
		g.Entities = append(g.Entities, authority.Entity{Ref: ref, Label: platform})
		g.Edges = append(g.Edges, authority.Edge{From: root, To: ref, Relation: "platform"})
	}
	return g
}

func TestSuggestConsole(t *testing.T) {
	options := []string{"PS3", "PS5", "Nintendo Switch", "Nintendo Switch 2", "Wii", "Xbox", "Xbox360", "XboxOne"}
	for _, tc := range []struct {
		name, console, format string
		platforms             []string
		want                  string
		warning               bool
	}{
		{name: "photo picks owned platform", console: "PlayStation 3", platforms: []string{"PlayStation 3", "Microsoft Windows", "Xbox 360"}, want: "PS3"},
		{name: "old scan format", format: "Nintendo Switch physical game", platforms: []string{"Nintendo Switch"}, want: "Nintendo Switch"},
		{name: "disc uncertainty is separate", format: "PlayStation 3 game case; disc not visible", platforms: []string{"PlayStation 3", "PlayStation 5"}, want: "PS3"},
		{name: "switch generations stay separate", console: "Nintendo Switch 2", platforms: []string{"Nintendo Switch", "Nintendo Switch 2"}, want: "Nintendo Switch 2"},
		{name: "catalogue omission keeps photo", console: "PS3", platforms: []string{"Xbox 360"}, want: "PS3", warning: true},
		{name: "single catalogue platform", platforms: []string{"PlayStation 5"}, want: "PS5"},
		{name: "multi platform without photo", platforms: []string{"PlayStation 3", "Xbox 360"}},
		{name: "ambiguous photo", console: "PS3 or PS5", platforms: []string{"PlayStation 5"}},
		{name: "uncertain photo", console: "possibly Nintendo Switch", platforms: []string{"Nintendo Switch"}},
		{name: "negated photo", console: "not PS3", platforms: []string{"PlayStation 3"}},
		{name: "unknown console", console: "unknown", platforms: []string{"PlayStation 3", "PlayStation 5"}},
		{name: "missing console banner", format: "PS3 game; console branding not visible", platforms: []string{"PlayStation 3"}},
		{name: "wii u is not wii", console: "Wii U physical game", platforms: []string{"Wii U"}, want: "Wii U"},
		{name: "xbox generations", console: "Xbox Series X game", platforms: []string{"Xbox Series X and Series S"}, want: "Xbox Series X/S"},
		{name: "portable is not ps1", platforms: []string{"PlayStation Portable"}, want: "PlayStation Portable"},
		{name: "no evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := store.IdentificationResult{Inferred: map[string]any{"console": tc.console, "format": tc.format}}
			hint := SuggestConsole(result, consoleGraph(tc.platforms...), options)
			if hint.Value != tc.want || (hint.Warning != "") != tc.warning {
				t.Fatalf("hint: %+v, want %q warning=%v", hint, tc.want, tc.warning)
			}
			if hint.Reason == "" {
				t.Fatal("missing explanation")
			}
		})
	}
}

func TestConsoleSuggestionUsesApprovedNames(t *testing.T) {
	hint := SuggestConsole(store.IdentificationResult{Inferred: map[string]any{"console": "PS3"}}, consoleGraph("PlayStation 3"), []string{"PlayStation 3"})
	if hint.Value != "PlayStation 3" || strings.Join(hint.Platforms, ",") != "PlayStation 3" {
		t.Fatalf("approved spelling lost: %+v", hint)
	}
}
