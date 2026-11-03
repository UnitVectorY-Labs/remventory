// Package authority isolates external databases from inventory and identification.
package authority

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Ref struct {
	Provider string `json:"provider"`
	Kind     string `json:"kind"`
	ID       string `json:"id"`
}

func (r Ref) Key() string { return r.Provider + ":" + r.Kind + ":" + r.ID }

type Entity struct {
	Ref
	Label       string          `json:"label"`
	URI         string          `json:"uri"`
	Description string          `json:"description,omitempty"`
	Fields      map[string]any  `json:"fields"`
	Raw         json.RawMessage `json:"raw,omitempty"`
	License     string          `json:"license"`
	RetrievedAt time.Time       `json:"retrieved_at"`
}
type Edge struct {
	From     Ref    `json:"from"`
	To       Ref    `json:"to"`
	Relation string `json:"relation"`
	Evidence string `json:"evidence"`
}
type Graph struct {
	Association string   `json:"association,omitempty"`
	Expanded    []Ref    `json:"expanded,omitempty"`
	Root        Ref      `json:"root"`
	Entities    []Entity `json:"entities"`
	Edges       []Edge   `json:"edges"`
	Truncated   bool     `json:"truncated"`
	Warnings    []string `json:"warnings,omitempty"`
}
type Query struct {
	Text    string `json:"text"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Barcode string `json:"barcode"`
	Catalog string `json:"catalog"`
	Format  string `json:"format"`
}
type Candidate struct {
	Ref
	Label       string `json:"label"`
	Description string `json:"description"`
	Score       int    `json:"score"`
	URI         string `json:"uri"`
}
type Capabilities struct {
	Name        string   `json:"name"`
	Domains     []string `json:"domains"`
	Search      bool     `json:"search"`
	Resolve     bool     `json:"resolve"`
	Identifiers []string `json:"identifiers"`
}
type Provider interface {
	Capabilities() Capabilities
	Search(context.Context, Query) ([]Candidate, error)
	Resolve(context.Context, Ref) (Graph, error)
}
type Registry map[string]Provider

func New() Registry {
	return Registry{"musicbrainz": &MusicBrainz{HTTP: NewClient()}, "wikidata": &Wikidata{HTTP: NewClient()}}
}
func (r Registry) Provider(name string) (Provider, error) {
	p, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("provider %q is not enabled", name)
	}
	return p, nil
}

// Wait is supplied by the application for a cross-replica PostgreSQL rate gate.
// Standalone live tests use the conservative process-local limiter.
type Client struct {
	HTTP *http.Client
	Wait func(context.Context, string) error
	mu   sync.Mutex
	next time.Time
}

func NewClient() *Client { return &Client{HTTP: &http.Client{Timeout: 40 * time.Second}} }
func (c *Client) getOnce(ctx context.Context, provider, url string) (*http.Response, error) {
	if c.Wait != nil {
		if err := c.Wait(ctx, provider); err != nil {
			return nil, err
		}
	} else {
		c.mu.Lock()
		delay := time.Until(c.next)
		if delay < 0 {
			delay = 0
		}
		c.next = time.Now().Add(delay + 1200*time.Millisecond)
		c.mu.Unlock()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Remventory/0.1 (https://github.com/UnitVectorY-Labs/remventory)")
	req.Header.Set("Accept", "application/json")
	return c.HTTP.Do(req)
}
func (c *Client) Get(ctx context.Context, provider, url string, dest any) error {
	for attempt := 0; attempt < 3; attempt++ {
		res, err := c.getOnce(ctx, provider, url)
		if err != nil {
			return err
		}
		if res.StatusCode == 429 || res.StatusCode == 503 {
			delay := time.Duration(3*(attempt+1)) * time.Second
			if seconds, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && seconds > 0 {
				delay = time.Duration(seconds) * time.Second
			}
			res.Body.Close()
			if attempt == 2 {
				return fmt.Errorf("%s temporarily unavailable (HTTP %d); retry later", provider, res.StatusCode)
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return fmt.Errorf("%s returned HTTP %d", provider, res.StatusCode)
		}
		var raw json.RawMessage
		err = json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&raw)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("%s response: %w", provider, err)
		}
		var apiError struct {
			Error json.RawMessage `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiError)
		if len(apiError.Error) > 0 && string(apiError.Error) != "null" {
			if attempt < 2 {
				timer := time.NewTimer(time.Duration(3*(attempt+1)) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
				continue
			}
			return fmt.Errorf("%s API unavailable: %s", provider, string(apiError.Error))
		}
		return json.Unmarshal(raw, dest)
	}
	return fmt.Errorf("%s unavailable", provider)
}

// Association describes the physical item's relationship to an authority entity.
// A product model/release identifies it; a creative work is contained on it.
func Association(e Entity) string {
	if e.Kind == "release" {
		return "identified_as"
	}
	for _, kind := range []string{"work", "recording", "release-group"} {
		if e.Kind == kind {
			return "contains"
		}
	}
	description := strings.ToLower(e.Description)
	for _, kind := range []string{"film", "television", "video game", "album", "recording"} {
		if strings.Contains(description, kind) {
			return "contains"
		}
	}
	return "identified_as"
}
