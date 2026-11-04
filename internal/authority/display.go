package authority

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A model selects a small presentation from verified fields. It never supplies
// facts, external identifiers, URLs, HTML, or executable UI instructions.
type DisplayPlan struct {
	Layout        string   `json:"layout"`
	SubtitleField string   `json:"subtitle_field"`
	Fields        []string `json:"fields"`
}
type DisplayFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type DisplayCard struct {
	Title    string        `json:"title"`
	Subtitle string        `json:"subtitle"`
	Facts    []DisplayFact `json:"facts"`
	Source   string        `json:"source"`
	URL      string        `json:"url"`
	EntityID string        `json:"entity_id,omitempty"`
	Curated  bool          `json:"curated"`
	Layout   string        `json:"layout"`
}

func RootEntity(g Graph) Entity {
	for _, e := range g.Entities {
		if e.Ref == g.Root {
			return e
		}
	}
	return Entity{}
}
func DisplayFields(g Graph) map[string]any {
	root := RootEntity(g)
	values := map[string]any{}
	for k, v := range root.Fields {
		values[k] = v
	}
	for _, edge := range g.Edges {
		if edge.From != g.Root {
			continue
		}
		if edge.Relation == "release_of" {
			for _, e := range g.Entities {
				if e.Ref == edge.To {
					if v, ok := e.Fields["original_date"]; ok {
						values["original_date"] = v
					}
				}
			}
		}
		switch edge.Relation {
		case "by_artist", "directed_by", "platform", "published_by", "contains":
			for _, e := range g.Entities {
				if e.Ref == edge.To && e.Label != "" {
					names, _ := values[edge.Relation].([]string)
					names = append(names, e.Label)
					sort.Strings(names)
					values[edge.Relation] = names
				}
			}
		}
	}
	if _, ok := values["original_date"]; !ok {
		if dates, ok := values["original_dates"].([]any); ok {
			best := ""
			for _, raw := range dates {
				d, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				t, _ := d["time"].(string)
				t = strings.TrimPrefix(t, "+")
				if t != "" && (best == "" || t < best) {
					best = t
				}
			}
			if best != "" {
				values["original_date"] = strings.Split(best, "T")[0]
			}
		}
	}
	return values
}
func displayValue(values map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var value any = values
	for _, p := range parts {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[p]
	}
	return value
}
func FactText(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		if len(v) > 10 && strings.Contains(v, "T00:") {
			v = strings.Split(strings.TrimPrefix(v, "+"), "T")[0]
		}
		v = strings.TrimSuffix(v, "-00-00")
		if len(v) > 180 {
			return v[:177] + "…"
		}
		return v
	case []string:
		if len(v) > 4 {
			return strings.Join(v[:4], ", ") + "…"
		}
		return strings.Join(v, ", ")
	case []any:
		out := []string{}
		for _, x := range v {
			if t := FactText(x); t != "" {
				out = append(out, t)
			}
			if len(out) >= 4 {
				break
			}
		}
		return strings.Join(out, ", ")
	case map[string]any:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
func CuratePlan(g Graph, p DisplayPlan) DisplayPlan {
	values := DisplayFields(g)
	switch p.Layout {
	case "identity", "release", "work":
	default:
		p.Layout = "identity"
	}
	if FactText(displayValue(values, p.SubtitleField)) == "" {
		p.SubtitleField = ""
	}
	if p.SubtitleField == "" {
		for _, key := range []string{"amiibo_series", "by_artist"} {
			if FactText(displayValue(values, key)) != "" {
				p.SubtitleField = key
				break
			}
		}
	}
	selected := []string{}
	seen := map[string]bool{}
	for _, key := range p.Fields {
		if !seen[key] && FactText(displayValue(values, key)) != "" {
			selected = append(selected, key)
			seen[key] = true
		}
		if len(selected) == 4 {
			break
		}
	}
	if len(selected) == 0 {
		for _, key := range []string{"character", "amiibo_series", "game_series", "by_artist", "original_date", "formats", "catalog_numbers", "release_date", "directed_by", "platform", "release_dates.na", "country"} {
			if FactText(displayValue(values, key)) != "" {
				selected = append(selected, key)
			}
			if len(selected) == 4 {
				break
			}
		}
	}
	p.Fields = selected
	return p
}
func DisplayFingerprint(g Graph) string {
	root := RootEntity(g)
	b, _ := json.Marshal([]any{root.Label, root.Description, DisplayFields(g)})
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func RenderCard(g Graph, p DisplayPlan, curated bool) DisplayCard {
	p = CuratePlan(g, p)
	root := RootEntity(g)
	values := DisplayFields(g)
	source := map[string]string{"amiibo": "AmiiboAPI", "musicbrainz": "MusicBrainz", "wikidata": "Wikidata"}[root.Provider]
	if source == "" {
		source = root.Provider
	}
	card := DisplayCard{Title: root.Label, Subtitle: FactText(displayValue(values, p.SubtitleField)), Source: source, URL: root.URI, Layout: p.Layout, Curated: curated, Facts: []DisplayFact{}}
	labels := map[string]string{"character": "Character", "amiibo_series": "Amiibo series", "game_series": "Game series", "by_artist": "Artist", "original_date": "Original release", "formats": "Format", "catalog_numbers": "Catalogue number", "release_date": "Edition release", "directed_by": "Director", "platform": "Platform", "release_dates.na": "US release", "release_dates.eu": "Europe release", "country": "Country", "type": "Type"}
	seen := map[string]bool{card.Title: true}
	if card.Subtitle != "" {
		seen[card.Subtitle] = true
	}
	for _, key := range p.Fields {
		value := FactText(displayValue(values, key))
		if seen[value] {
			continue
		}
		seen[value] = true
		label := labels[key]
		if label == "" {
			label = strings.ReplaceAll(key, "_", " ")
		}
		card.Facts = append(card.Facts, DisplayFact{label, value})
	}
	return card
}
