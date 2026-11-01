package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

type FieldDefinition struct {
	Key      string          `json:"key" db:"key"`
	Label    string          `json:"label" db:"label"`
	DataType string          `json:"data_type" db:"data_type"`
	Config   json.RawMessage `json:"config" db:"config"`
}
type GraphVocabulary struct {
	Fields    []FieldDefinition   `json:"fields"`
	TagFields map[string][]string `json:"tag_fields"`
	Templates []Category          `json:"templates"`
}
type GraphProposalPayload struct {
	Operation string          `json:"operation"`
	Key       string          `json:"key,omitempty"`
	Label     string          `json:"label,omitempty"`
	DataType  string          `json:"data_type,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	Tag       string          `json:"tag,omitempty"`
	FieldKeys []string        `json:"field_keys,omitempty"`
}

type fieldQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readFields(ctx context.Context, q fieldQuerier) ([]FieldDefinition, error) {
	rows, err := q.Query(ctx, `select key,label,data_type,config from field_definitions order by lower(label),key`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[FieldDefinition])
}
func (s *Store) ListFields(ctx context.Context) ([]FieldDefinition, error) {
	return readFields(ctx, s.pool)
}
func (s *Store) Vocabulary(ctx context.Context) (GraphVocabulary, error) {
	result := GraphVocabulary{TagFields: map[string][]string{}}
	var err error
	result.Fields, err = s.ListFields(ctx)
	if err != nil {
		return result, err
	}
	rows, err := s.pool.Query(ctx, `select tag,field_key from tag_field_suggestions order by tag,field_key`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var tag, key string
		if err := rows.Scan(&tag, &key); err != nil {
			rows.Close()
			return result, err
		}
		result.TagFields[tag] = append(result.TagFields[tag], key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.Templates, err = s.ListCategories(ctx, 200, 0)
	return result, err
}
func fieldAttributes(fields []FieldDefinition) []Attribute {
	result := make([]Attribute, 0, len(fields))
	for _, f := range fields {
		result = append(result, Attribute{Key: f.Key, Label: f.Label, DataType: f.DataType, Config: f.Config})
	}
	return result
}
func validateGraphPayload(p *GraphProposalPayload) error {
	switch p.Operation {
	case "define_field":
		p.Key = strings.TrimSpace(p.Key)
		p.Label = strings.TrimSpace(p.Label)
		if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`).MatchString(p.Key) || p.Label == "" {
			return errors.New("use a stable field key (lowercase letters, digits, underscores) and a label")
		}
		switch p.DataType {
		case "text", "number", "boolean", "date", "links", "enum":
		default:
			return errors.New("field type must be text, number, boolean, date, links, or enum")
		}
		if len(p.Config) == 0 {
			p.Config = json.RawMessage(`{}`)
		}
		if !json.Valid(p.Config) {
			return errors.New("field configuration must be JSON")
		}
		if p.DataType == "enum" {
			candidate := CategoryProposalPayload{Operation: "create", Name: "validation", Attributes: []AttributeDraft{{Key: p.Key, Label: p.Label, DataType: "enum", Config: p.Config}}}
			return validateCategoryProposalPayload(&candidate)
		}
	case "suggest_fields":
		p.Tag = strings.TrimSpace(p.Tag)
		if p.Tag == "" || len(p.Tag) > 80 || strings.Contains(p.Tag, ",") {
			return errors.New("provide one tag, up to 80 characters, without commas")
		}
	default:
		return errors.New("graph operation must be define_field or suggest_fields")
	}
	return nil
}
func (s *Store) CreateGraphProposal(ctx context.Context, p GraphProposalPayload) (Proposal, error) {
	if err := validateGraphPayload(&p); err != nil {
		return Proposal{}, err
	}
	if p.Operation == "suggest_fields" {
		fields, err := s.ListFields(ctx)
		if err != nil {
			return Proposal{}, err
		}
		known := map[string]bool{}
		for _, f := range fields {
			known[f.Key] = true
		}
		for _, key := range p.FieldKeys {
			if !known[key] {
				return Proposal{}, fmt.Errorf("define and approve field %q before associating it with a tag", key)
			}
		}
	}
	if p.Operation == "define_field" {
		var exists bool
		if err := s.pool.QueryRow(ctx, `select exists(select 1 from field_definitions where key=$1)`, p.Key).Scan(&exists); err != nil {
			return Proposal{}, err
		}
		if exists {
			return Proposal{}, errors.New("this field is already defined; use the existing field")
		}
	}
	return s.createProposal(ctx, DefaultUserID, "graph_change", p)
}
func applyGraphChange(ctx context.Context, tx pgx.Tx, p GraphProposalPayload) error {
	if err := validateGraphPayload(&p); err != nil {
		return err
	}
	if p.Operation == "define_field" {
		_, err := tx.Exec(ctx, `insert into field_definitions(key,label,data_type,config) values($1,$2,$3,$4)`, p.Key, p.Label, p.DataType, p.Config)
		return err
	}
	if _, err := tx.Exec(ctx, `delete from tag_field_suggestions where lower(tag)=lower($1)`, p.Tag); err != nil {
		return err
	}
	for _, key := range p.FieldKeys {
		if _, err := tx.Exec(ctx, `insert into tag_field_suggestions(tag,field_key) values($1,$2) on conflict do nothing`, p.Tag, key); err != nil {
			return err
		}
	}
	return nil
}

// Resolve creation defaults once. Items do not retain a template association.
func (s *Store) prepareItem(ctx context.Context, p ItemProposalPayload) (ItemProposalPayload, error) {
	values := map[string]any{}
	if len(p.Attributes) > 0 {
		if err := json.Unmarshal(p.Attributes, &values); err != nil || values == nil {
			return p, errors.New("attributes must be a JSON object")
		}
	}
	if p.Operation == "" || p.Operation == "create" {
		if p.CategoryID != "" {
			template, err := s.GetCategoryDefinition(ctx, p.CategoryID)
			if err != nil {
				return p, err
			}
			defaults := map[string]any{}
			json.Unmarshal(template.DefaultValues, &defaults)
			for key, value := range values {
				defaults[key] = value
			}
			values = defaults
			p.Tags = append(append([]string{}, template.DefaultTags...), p.Tags...)
		}
	}
	if p.Operation == "quantity_adjust" {
		item, err := s.GetItem(ctx, p.ItemID)
		if err != nil {
			return p, err
		}
		json.Unmarshal(item.Attributes, &values)
		current := 1.0
		if value, ok := values["quantity"].(float64); ok {
			current = value
		}
		values["quantity"] = current + float64(p.QuantityDelta)
		p.Operation = "update"
	}
	for _, key := range p.RemoveFields {
		delete(values, key)
	}

	// Legacy clients map old core fields onto registered optional attributes.
	if p.Title != "" {
		if _, ok := values["title"]; !ok {
			values["title"] = p.Title
		}
	}
	if p.Location != nil {
		if *p.Location != "" {
			values["location"] = *p.Location
		} else {
			delete(values, "location")
		}
	}
	if p.Quantity > 1 {
		if _, ok := values["quantity"]; !ok {
			values["quantity"] = p.Quantity
		}
	}
	p.Attributes, _ = json.Marshal(values)
	p.CategoryID = ""
	labelTags := p.Tags
	if p.ItemID != "" && labelTags == nil {
		item, err := s.getItem(ctx, p.ItemID)
		if err != nil {
			return p, err
		}
		labelTags = item.Tags
	}
	p.Title = DisplayLabel(values, labelTags)
	p.Quantity = 1
	p.Location = nil
	return p, nil
}
func DisplayLabel(values map[string]any, tags []string) string {
	if title, ok := values["title"].(string); ok && strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	if len(tags) > 0 {
		return strings.Join(tags, " · ")
	}
	return "Item"
}
