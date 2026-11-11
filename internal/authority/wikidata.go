package authority

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Wikidata struct {
	HTTP    *Client
	BaseURL string
}

func (p *Wikidata) base() string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return "https://www.wikidata.org/w/api.php"
}
func (*Wikidata) Capabilities() Capabilities {
	return Capabilities{"Wikidata", []string{"movies", "television", "games", "hardware", "accessories", "books", "other"}, true, true, []string{"QID"}}
}
func (p *Wikidata) Search(ctx context.Context, q Query) ([]Candidate, error) {
	text := q.Text
	if text == "" {
		text = q.Title
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("search text required")
	}
	var res struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"search"`
		Error any `json:"error"`
	}
	err := p.HTTP.Get(ctx, "wikidata", p.base()+"?action=wbsearchentities&language=en&uselang=en&type=item&limit=8&format=json&maxlag=5&search="+url.QueryEscape(text), &res)
	if err == nil && res.Error != nil {
		err = fmt.Errorf("Wikidata temporarily unavailable: %v", res.Error)
	}
	out := []Candidate{}
	for _, s := range res.Search {
		out = append(out, Candidate{Ref: Ref{"wikidata", "entity", s.ID}, Label: s.Label, Description: s.Description, URI: "https://www.wikidata.org/wiki/" + s.ID})
	}
	return out, err
}

var qid = regexp.MustCompile(`^Q[1-9][0-9]*$`)

type wdEntity struct {
	ID      string `json:"id"`
	Missing string `json:"missing"`
	Labels  map[string]struct {
		Value string `json:"value"`
	} `json:"labels"`
	Descriptions map[string]struct {
		Value string `json:"value"`
	} `json:"descriptions"`
	Claims map[string][]wdClaim `json:"claims"`
}
type wdClaim struct {
	Rank     string `json:"rank"`
	Mainsnak struct {
		DataType  string `json:"datatype"`
		DataValue struct {
			Value jsontext.Value `json:"value"`
		} `json:"datavalue"`
	} `json:"mainsnak"`
}

var wdRelations = map[string]string{"P179": "part_of_series", "P361": "part_of", "P527": "contains", "P144": "based_on", "P4969": "derivative_work", "P57": "directed_by", "P161": "cast", "P50": "by_author", "P175": "by_artist", "P400": "platform", "P178": "developed_by", "P123": "published_by", "P750": "distributed_by", "P31": "instance_of", "P138": "named_after", "P8956": "compatible_with"}

func (p *Wikidata) fetch(ctx context.Context, ids []string) (map[string]wdEntity, error) {
	if len(ids) > 50 {
		combined := map[string]wdEntity{}
		for start := 0; start < len(ids); start += 50 {
			batch, err := p.fetch(ctx, ids[start:min(start+50, len(ids))])
			if err != nil {
				return nil, err
			}
			for id, e := range batch {
				combined[id] = e
			}
		}
		return combined, nil
	}
	var res struct {
		Entities map[string]wdEntity `json:"entities"`
		Error    any                 `json:"error"`
	}
	err := p.HTTP.Get(ctx, "wikidata", p.base()+"?action=wbgetentities&format=json&languages=en%7Cmul&languagefallback=1&props=labels|descriptions|claims&maxlag=5&ids="+url.QueryEscape(strings.Join(ids, "|")), &res)
	if err == nil && res.Error != nil {
		err = fmt.Errorf("Wikidata unavailable: %v", res.Error)
	}
	return res.Entities, err
}
func (p *Wikidata) Resolve(ctx context.Context, r Ref) (Graph, error) {
	if r.Kind != "entity" || !qid.MatchString(r.ID) {
		return Graph{}, fmt.Errorf("invalid Wikidata QID")
	}
	entities, err := p.fetch(ctx, []string{r.ID})
	if err != nil {
		return Graph{}, err
	}
	root, ok := entities[r.ID]
	if !ok || root.ID == "" {
		return Graph{}, fmt.Errorf("Wikidata entity not found")
	}
	association := Association(Entity{Ref: Ref{Kind: "entity"}, Description: root.Descriptions["en"].Value})
	g := Graph{Association: association, Root: Ref{"wikidata", "entity", root.ID}, Entities: []Entity{}, Edges: []Edge{}}
	seen := map[string]bool{}
	front := []string{root.ID}
	seen[root.ID] = true
	// Expand containment/series hierarchy to depth 3, at most 80 entities.
	// Credits/platforms are cached one hop and are never recursively expanded.
	childQueries := 0
	for depth := 0; depth < 4 && len(front) > 0; depth++ {
		next := []string{}
		for _, id := range front {
			g.Expanded = append(g.Expanded, Ref{"wikidata", "entity", id})
			e := entities[id]
			description := strings.ToLower(e.Descriptions["en"].Value)
			if depth < 3 && len(seen) < 70 && childQueries < 4 && (strings.Contains(description, "television series") || strings.Contains(description, "television season") || strings.Contains(description, "season of")) {
				childQueries++
				children, childErr := p.children(ctx, id)
				if childErr != nil {
					g.Warnings = append(g.Warnings, "Child lookup incomplete: "+childErr.Error())
				} else {
					for _, child := range children {
						if !seen[child] {
							if len(seen) >= 80 {
								g.Truncated = true
								continue
							}
							seen[child] = true
							next = append(next, child)
						}
						g.Edges = append(g.Edges, Edge{Ref{"wikidata", "entity", id}, Ref{"wikidata", "entity", child}, "contains", "Wikidata inverse P361/P179; bounded child query"})
					}
					if len(children) >= 24 {
						g.Truncated = true
					}
				}
			}
			for _, prop := range []string{"P527", "P361", "P179", "P144", "P400", "P178", "P123", "P57", "P175", "P50", "P31", "P161", "P750", "P138", "P8956", "P4969"} {
				rel := wdRelations[prop]
				for claimIndex, c := range e.Claims[prop] {
					if claimIndex >= 8 && prop != "P527" {
						g.Truncated = true
						break
					}
					if c.Rank == "deprecated" {
						continue
					}
					var v struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(c.Mainsnak.DataValue.Value, &v)
					if !qid.MatchString(v.ID) {
						continue
					}
					if !seen[v.ID] {
						if len(seen) >= 80 {
							g.Truncated = true
							continue
						}
						seen[v.ID] = true
						next = append(next, v.ID)
					}
					g.Edges = append(g.Edges, Edge{Ref{"wikidata", "entity", id}, Ref{"wikidata", "entity", v.ID}, rel, "Wikidata " + prop})
				}
			}
		}
		if len(next) == 0 {
			break
		}
		loaded, err := p.fetch(ctx, next)
		if err != nil {
			return Graph{}, err
		}
		for id, e := range loaded {
			if e.ID != "" {
				entities[id] = e
			}
		}
		front = nil
		if depth < 2 {
			for _, id := range next {
				for _, edge := range g.Edges {
					if edge.To.ID == id && (edge.Relation == "contains" || edge.Relation == "part_of" || edge.Relation == "part_of_series") {
						front = append(front, id)
						break
					}
				}
			}
		}
	}
	for _, e := range entities {
		if e.ID == "" {
			continue
		}
		fields := map[string]any{}
		for prop, key := range map[string]string{"P577": "original_dates", "P580": "start_dates", "P582": "end_dates", "P345": "imdb_ids", "P436": "musicbrainz_release_group_ids", "P437": "distribution_formats", "P528": "catalog_codes"} {
			vals := []any{}
			for _, c := range e.Claims[prop] {
				if c.Rank == "deprecated" {
					continue
				}
				var v any
				_ = json.Unmarshal(c.Mainsnak.DataValue.Value, &v)
				vals = append(vals, v)
			}
			if len(vals) > 0 {
				fields[key] = vals
			}
		}
		raw, _ := json.Marshal(e)
		g.Entities = append(g.Entities, Entity{Ref: Ref{"wikidata", "entity", e.ID}, Label: wikidataLabel(e), Description: e.Descriptions["en"].Value, URI: "https://www.wikidata.org/wiki/" + e.ID, Fields: fields, Raw: raw, License: "CC0-1.0", RetrievedAt: time.Now().UTC()})
	}
	valid := map[string]bool{}
	for _, e := range g.Entities {
		valid[e.Key()] = true
	}
	edges := []Edge{}
	for _, e := range g.Edges {
		if valid[e.From.Key()] && valid[e.To.Key()] {
			edges = append(edges, e)
		}
	}
	g.Edges = edges
	return g, nil
}

// Only TV hierarchies need inverse lookup; never expand every linked person/class.
func (p *Wikidata) children(ctx context.Context, id string) ([]string, error) {
	query := `SELECT DISTINCT ?child WHERE { { ?child wdt:P361 wd:` + id + ` } UNION { ?child wdt:P179 wd:` + id + ` } } LIMIT 24`
	var response struct {
		Results struct {
			Bindings []struct {
				Child struct {
					Value string `json:"value"`
				} `json:"child"`
			} `json:"bindings"`
		} `json:"results"`
	}
	err := p.HTTP.Get(ctx, "wikidata", "https://query.wikidata.org/sparql?format=json&query="+url.QueryEscape(query), &response)
	out := []string{}
	for _, b := range response.Results.Bindings {
		id := b.Child.Value[strings.LastIndex(b.Child.Value, "/")+1:]
		if qid.MatchString(id) {
			out = append(out, id)
		}
	}
	return out, err
}

// Many proper names use Wikidata's language-neutral label rather than English.
func wikidataLabel(e wdEntity) string {
	if label := e.Labels["en"].Value; strings.TrimSpace(label) != "" {
		return label
	}
	return e.Labels["mul"].Value
}
