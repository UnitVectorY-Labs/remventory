package store

import (
	"context"
	"fmt"
	"strings"
)

// IntakeConfig belongs to the existing template; jobs copy a snapshot of it.
type IntakeConfig struct {
	Providers     []string            `json:"providers"`
	Prompt        string              `json:"prompt"`
	DisplayFields map[string][]string `json:"display_fields"`
}

func (c IntakeConfig) Validate() error {
	if len(c.Prompt) > 12000 {
		return fmt.Errorf("type instructions must be under 12,000 characters")
	}
	for _, p := range c.Providers {
		if p != "amiibo" && p != "musicbrainz" && p != "wikidata" {
			return fmt.Errorf("unknown provider %q", p)
		}
	}
	for p, fields := range c.DisplayFields {
		if p != "amiibo" && p != "musicbrainz" && p != "wikidata" {
			return fmt.Errorf("unknown display provider %q", p)
		}
		if len(fields) > 4 {
			return fmt.Errorf("choose up to four catalogue fields per source")
		}
		for _, f := range fields {
			if len(f) > 100 || strings.ContainsAny(f, "<>\n\r") {
				return fmt.Errorf("invalid source field path")
			}
		}
	}
	return nil
}
func (c IntakeConfig) Allows(provider string) bool {
	if c.Providers == nil {
		return true
	}
	for _, p := range c.Providers {
		if p == provider {
			return true
		}
	}
	return false
}
func (s *Store) ProfileForItem(ctx context.Context, item Item) (Category, error) {
	profiles, err := s.ListCategories(ctx, 200, 0)
	if err != nil {
		return Category{}, err
	}
	best := Category{}
	score := 0
	for _, p := range profiles {
		n := 0
		for _, tag := range p.DefaultTags {
			for _, owned := range item.Tags {
				if strings.EqualFold(tag, owned) {
					n++
				}
			}
		}
		if n > score {
			best = p
			score = n
		}
	}
	return best, nil
}

// Legacy jobs have no policy snapshot. Resolve their existing template or tags;
// new jobs continue to use their immutable captured policy.
func (s *Store) ProfileForIdentification(ctx context.Context, j IdentificationJob) (Category, error) {
	if j.Input.TemplateID != "" {
		p, err := s.GetCategoryDefinition(ctx, j.Input.TemplateID)
		if err == nil {
			return p, nil
		}
		if err != ErrNotFound {
			return Category{}, err
		}
	}
	if j.ItemID != "" {
		item, err := s.GetItem(ctx, j.ItemID)
		if err != nil {
			return Category{}, err
		}
		return s.ProfileForItem(ctx, item)
	}
	profiles, err := s.ListCategories(ctx, 200, 0)
	if err != nil {
		return Category{}, err
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Name, j.Category) {
			return p, nil
		}
	}
	return Category{}, nil
}
