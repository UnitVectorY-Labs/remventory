package authority

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"strings"
	"time"
)

type MusicBrainz struct {
	HTTP    *Client
	BaseURL string
}

func (p *MusicBrainz) base() string {
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return "https://musicbrainz.org/ws/2"
}
func (*MusicBrainz) Capabilities() Capabilities {
	return Capabilities{"MusicBrainz", []string{"music", "vinyl", "CD", "cassette"}, true, true, []string{"MBID", "barcode", "catalog number"}}
}
func quote(s string) string { return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"` }
func (p *MusicBrainz) Search(ctx context.Context, q Query) ([]Candidate, error) {
	query := q.Text
	parts := []string{}
	if q.Title != "" {
		parts = append(parts, "release:"+quote(q.Title))
	}
	if q.Artist != "" {
		parts = append(parts, "artist:"+quote(q.Artist))
	}
	if q.Barcode != "" {
		parts = append(parts, "barcode:"+quote(q.Barcode))
	}
	if q.Catalog != "" {
		parts = append(parts, "catno:"+quote(q.Catalog))
	}
	if q.Format != "" {
		if strings.EqualFold(q.Format, "vinyl") {
			parts = append(parts, `format:("12\" Vinyl" OR "7\" Vinyl" OR "10\" Vinyl" OR "Vinyl")`)
		} else {
			parts = append(parts, "format:"+quote(q.Format))
		}
	}
	if len(parts) > 0 {
		query = strings.Join(parts, " AND ")
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search text or identifiers required")
	}
	var result struct {
		Releases []mbRelease `json:"releases"`
	}
	err := p.HTTP.Get(ctx, "musicbrainz", p.base()+"/release/?fmt=json&limit=8&query="+url.QueryEscape(query), &result)
	out := []Candidate{}
	for _, r := range result.Releases {
		desc := r.Date + " · " + r.Country + " · " + r.ReleaseGroup.Type + " · " + r.Disambiguation
		for _, c := range r.ArtistCredit {
			desc += " · " + c.Name
		}
		for _, m := range r.Media {
			desc += " · " + m.Format
		}
		for _, l := range r.LabelInfo {
			desc += " · " + l.Catalog
		}
		out = append(out, Candidate{Ref: Ref{"musicbrainz", "release", r.ID}, Label: r.Title, Description: desc, Score: r.Score, URI: "https://musicbrainz.org/release/" + r.ID})
	}
	return out, err
}

type mbCredit struct {
	Name   string   `json:"name"`
	Artist mbArtist `json:"artist"`
}
type mbArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type mbRelease struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Date           string     `json:"date"`
	Country        string     `json:"country"`
	Barcode        string     `json:"barcode"`
	Disambiguation string     `json:"disambiguation"`
	Score          int        `json:"score"`
	ArtistCredit   []mbCredit `json:"artist-credit"`
	ReleaseGroup   struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Date  string `json:"first-release-date"`
		Type  string `json:"primary-type"`
	} `json:"release-group"`
	LabelInfo []struct {
		Catalog string   `json:"catalog-number"`
		Label   mbArtist `json:"label"`
	} `json:"label-info"`
	Media []struct {
		Position int    `json:"position"`
		Format   string `json:"format"`
		Tracks   []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Position  int    `json:"position"`
			Recording struct {
				ID           string     `json:"id"`
				Title        string     `json:"title"`
				ArtistCredit []mbCredit `json:"artist-credit"`
				Relations    []struct {
					Type string `json:"type"`
					Work struct {
						ID    string `json:"id"`
						Title string `json:"title"`
					} `json:"work"`
				} `json:"relations"`
			} `json:"recording"`
		} `json:"tracks"`
	} `json:"media"`
}

func (p *MusicBrainz) Resolve(ctx context.Context, ref Ref) (Graph, error) {
	if ref.Kind != "release" {
		return Graph{}, fmt.Errorf("resolve a MusicBrainz release; album identity alone cannot verify a pressing")
	}
	if _, err := uuid.Parse(ref.ID); err != nil {
		return Graph{}, fmt.Errorf("invalid MBID")
	}
	var raw jsontext.Value
	err := p.HTTP.Get(ctx, "musicbrainz", p.base()+"/release/"+ref.ID+"?fmt=json&inc=artists+release-groups+recordings+artist-credits+labels+recording-level-rels+work-rels", &raw)
	if err != nil {
		return Graph{}, err
	}
	return normalizeMB(raw)
}
func normalizeMB(raw jsontext.Value) (Graph, error) {
	var r mbRelease
	if err := json.Unmarshal(raw, &r); err != nil {
		return Graph{}, err
	}
	if _, err := uuid.Parse(r.ID); err != nil {
		return Graph{}, fmt.Errorf("source returned invalid MBID")
	}
	g := Graph{Association: "identified_as", Root: Ref{"musicbrainz", "release", r.ID}, Entities: []Entity{}, Edges: []Edge{}}
	seen := map[string]bool{}
	add := func(kind, id, label string, fields map[string]any) Ref {
		ref := Ref{"musicbrainz", kind, id}
		if !seen[ref.Key()] {
			seen[ref.Key()] = true
			g.Entities = append(g.Entities, Entity{Ref: ref, Label: label, URI: "https://musicbrainz.org/" + kind + "/" + id, Fields: fields, License: "CC0-1.0", RetrievedAt: time.Now().UTC()})
		}
		return ref
	}
	edge := func(from, to Ref, rel string) {
		g.Edges = append(g.Edges, Edge{from, to, rel, "MusicBrainz release response"})
	}
	root := add("release", r.ID, r.Title, map[string]any{"release_date": r.Date, "country": r.Country, "barcode": r.Barcode, "disambiguation": r.Disambiguation})
	// Keep only the selected core fields. Ratings/tags/annotations and artwork are not requested or retained.
	if r.ReleaseGroup.ID != "" {
		rg := add("release-group", r.ReleaseGroup.ID, r.ReleaseGroup.Title, map[string]any{"original_date": r.ReleaseGroup.Date, "type": r.ReleaseGroup.Type})
		edge(root, rg, "release_of")
		for _, c := range r.ArtistCredit {
			if c.Artist.ID != "" {
				a := add("artist", c.Artist.ID, c.Artist.Name, map[string]any{})
				edge(rg, a, "by_artist")
				edge(root, a, "by_artist")
			}
		}
	}
	catalogs := []string{}
	formats := []string{}
	for _, l := range r.LabelInfo {
		catalogs = append(catalogs, l.Catalog)
		if l.Label.ID != "" {
			edge(root, add("label", l.Label.ID, l.Label.Name, map[string]any{}), "published_by")
		}
	}
	mediaFields := []map[string]any{}
	trackCount := 0
	for _, m := range r.Media {
		formats = append(formats, m.Format)
		for _, t := range m.Tracks {
			trackCount++
			if trackCount > 250 {
				g.Truncated = true
				continue
			}
			if t.Recording.ID == "" {
				continue
			}
			rec := add("recording", t.Recording.ID, t.Recording.Title, map[string]any{})
			edge(root, rec, "contains")
			for _, c := range t.Recording.ArtistCredit {
				if c.Artist.ID != "" {
					edge(rec, add("artist", c.Artist.ID, c.Artist.Name, map[string]any{}), "by_artist")
				}
			}
			for _, rel := range t.Recording.Relations {
				if rel.Work.ID != "" {
					edge(rec, add("work", rel.Work.ID, rel.Work.Title, map[string]any{}), "performance_of")
				}
			}
		}
	}
	for _, m := range r.Media {
		tracks := []map[string]any{}
		for index, t := range m.Tracks {
			if index >= 250 {
				break
			}
			tracks = append(tracks, map[string]any{"position": t.Position, "title": t.Title, "recording_id": t.Recording.ID})
		}
		mediaFields = append(mediaFields, map[string]any{"position": m.Position, "format": m.Format, "tracks": tracks})
	}
	g.Entities[0].Fields["media"] = mediaFields
	g.Entities[0].Fields["catalog_numbers"] = catalogs
	g.Entities[0].Fields["formats"] = formats
	for _, e := range g.Entities {
		if e.Kind == "release" || e.Kind == "release-group" || e.Kind == "recording" {
			g.Expanded = append(g.Expanded, e.Ref)
		}
	}
	return g, nil
}
