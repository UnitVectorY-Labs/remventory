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

// AmiiboAPI's maintained community catalogue identifies figure/card variants,
// rather than conflating an owned figure with its fictional character.
type Amiibo struct {
	HTTP    *Client
	BaseURL string
}

func (p *Amiibo) base() string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return "https://amiiboapi.org/api/amiibo/"
}
func (*Amiibo) Capabilities() Capabilities {
	return Capabilities{"AmiiboAPI", []string{"amiibo", "collectible figures", "cards"}, true, true, []string{"16 hexadecimal head+tail identifier"}}
}

type amiiboRecord struct {
	Image      string         `json:"image"`
	Head       string         `json:"head"`
	Tail       string         `json:"tail"`
	Name       string         `json:"name"`
	Character  string         `json:"character"`
	Series     string         `json:"amiiboSeries"`
	GameSeries string         `json:"gameSeries"`
	Type       string         `json:"type"`
	Release    map[string]any `json:"release"`
}

var amiiboID = regexp.MustCompile(`^[0-9a-f]{16}$`)

func (p *Amiibo) Search(ctx context.Context, q Query) ([]Candidate, error) {
	name := strings.TrimSpace(q.Title)
	if name == "" {
		name = strings.TrimSpace(q.Text)
	}
	if name == "" {
		return nil, fmt.Errorf("enter a character or figure name")
	}
	var result amiiboResponse
	err := p.HTTP.Get(ctx, "amiibo", p.base()+"?name="+url.QueryEscape(name), &result)
	out := []Candidate{}
	if err != nil && strings.Contains(err.Error(), "404") {
		return out, nil
	}
	for _, r := range result.Amiibo {
		id := strings.ToLower(r.Head + r.Tail)
		if !amiiboID.MatchString(id) {
			continue
		}
		out = append(out, Candidate{Ref: Ref{"amiibo", "figure", id}, Label: r.Name, Description: r.Series + " · " + r.GameSeries + " · " + r.Type, URI: p.base() + "?id=" + id})
		if len(out) >= 20 {
			break
		}
	}
	return out, err
}
func (p *Amiibo) Resolve(ctx context.Context, ref Ref) (Graph, error) {
	id := strings.ToLower(strings.TrimPrefix(ref.ID, "0x"))
	if ref.Kind != "figure" || !amiiboID.MatchString(id) {
		return Graph{}, fmt.Errorf("invalid Amiibo head/tail identifier")
	}
	var result amiiboResponse
	if err := p.HTTP.Get(ctx, "amiibo", p.base()+"?id="+id, &result); err != nil {
		return Graph{}, err
	}
	for _, r := range result.Amiibo {
		if strings.ToLower(r.Head+r.Tail) != id {
			continue
		}
		root := Ref{"amiibo", "figure", id}
		fields := map[string]any{"character": r.Character, "amiibo_series": r.Series, "game_series": r.GameSeries, "type": r.Type, "release_dates": r.Release}
		raw, _ := json.Marshal(fields)
		return Graph{Association: "identified_as", Root: root, Entities: []Entity{{Ref: root, Label: r.Name, Description: r.Series + " amiibo " + strings.ToLower(r.Type), URI: p.base() + "?id=" + id, Fields: fields, Raw: raw, License: "MIT (catalogue metadata; artwork excluded)", RetrievedAt: time.Now().UTC()}}, Edges: []Edge{}, Expanded: []Ref{root}}, nil
	}
	return Graph{}, fmt.Errorf("Amiibo identifier was not returned by the catalogue")
}

// Search returns an array, while an exact identifier lookup returns one object.
type amiiboResponse struct{ Amiibo []amiiboRecord }

func (r *amiiboResponse) UnmarshalJSON(b []byte) error {
	var wire struct {
		Amiibo jsontext.Value `json:"amiibo"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		return err
	}
	if len(wire.Amiibo) == 0 || string(wire.Amiibo) == "null" {
		r.Amiibo = nil
		return nil
	}
	if wire.Amiibo[0] == '[' {
		return json.Unmarshal(wire.Amiibo, &r.Amiibo)
	}
	var one amiiboRecord
	if err := json.Unmarshal(wire.Amiibo, &one); err != nil {
		return err
	}
	r.Amiibo = []amiiboRecord{one}
	return nil
}
