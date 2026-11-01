package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	pool *pgxpool.Pool
}

const DefaultUserID = "00000000-0000-0000-0000-000000000001"

var ErrNotFound = errors.New("not found")

type Category struct {
	DefaultTags   []string        `json:"default_tags" db:"default_tags"`
	DefaultValues json.RawMessage `json:"default_values" db:"default_values"`
	ID            string          `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	Name          string          `json:"name" db:"name"`
	Description   string          `json:"description,omitempty" db:"description"`
	Attributes    []Attribute     `json:"attributes,omitempty" db:"-"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}

type Attribute struct {
	ID           string          `json:"id" db:"id"`
	CategoryID   string          `json:"category_id" db:"category_id"`
	Key          string          `json:"key" db:"key"`
	Label        string          `json:"label" db:"label"`
	DataType     string          `json:"data_type" db:"data_type"`
	Required     bool            `json:"required" db:"required"`
	DisplayOrder int             `json:"display_order" db:"display_order"`
	Config       json.RawMessage `json:"config,omitempty" db:"config"`
}

type AttributeDraft struct {
	Key          string          `json:"key"`
	Label        string          `json:"label"`
	DataType     string          `json:"data_type"`
	Required     bool            `json:"required"`
	DisplayOrder int             `json:"display_order"`
	Config       json.RawMessage `json:"config,omitempty"`
}

type Item struct {
	Tags       []string        `json:"tags" db:"tags"`
	Location   string          `json:"-" db:"location"`
	ID         string          `json:"id" db:"id"`
	UserID     string          `json:"user_id" db:"user_id"`
	CategoryID string          `json:"category_id" db:"category_id"`
	Title      string          `json:"title" db:"title"`
	Attributes json.RawMessage `json:"attributes" db:"attributes"`
	Quantity   int             `json:"-" db:"quantity"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
	Images     []ItemImage     `json:"images" db:"-"`
}

type ItemImage struct {
	IsPrimary        bool      `json:"is_primary" db:"is_primary"`
	ID               string    `json:"id" db:"id"`
	ItemID           string    `json:"item_id" db:"item_id"`
	OriginalKey      string    `json:"-" db:"original_key"`
	ThumbnailKey     string    `json:"-" db:"thumbnail_key"`
	MIMEType         string    `json:"mime_type" db:"mime_type"`
	OriginalFilename string    `json:"original_filename" db:"original_filename"`
	Width            int       `json:"width" db:"width"`
	Height           int       `json:"height" db:"height"`
	SizeBytes        int64     `json:"size_bytes" db:"size_bytes"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	OriginalURL      string    `json:"original_url" db:"-"`
	ThumbnailURL     string    `json:"thumbnail_url" db:"-"`
}

type Proposal struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	Type            string          `json:"type"`
	Status          string          `json:"status"`
	ProposedPayload json.RawMessage `json:"proposed_payload"`
	Reason          string          `json:"reason,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	DecidedAt       *time.Time      `json:"decided_at,omitempty"`
}

type CategoryProposalPayload struct {
	DefaultTags   []string         `json:"default_tags,omitempty"`
	DefaultValues json.RawMessage  `json:"default_values,omitempty"`
	Operation     string           `json:"operation"`
	CategoryID    string           `json:"category_id,omitempty"`
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Attributes    []AttributeDraft `json:"attributes"`
}

type ItemProposalPayload struct {
	RemoveFields       []string        `json:"remove_fields,omitempty"`
	PrimaryImageID     *string         `json:"primary_image_id,omitempty"`
	Tags               []string        `json:"tags"`
	Location           *string         `json:"location,omitempty"`
	Operation          string          `json:"operation"`
	CategoryID         string          `json:"category_id"`
	ItemID             string          `json:"item_id,omitempty"`
	Title              string          `json:"title"`
	Attributes         json.RawMessage `json:"attributes"`
	PreviousAttributes json.RawMessage `json:"previous_attributes,omitempty"`
	Quantity           int             `json:"quantity,omitempty"`
	QuantityDelta      int             `json:"quantity_delta,omitempty"`
}

type ProposalDecision struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

type ConversationTurn struct {
	Role       string   `json:"role"`
	Content    string   `json:"content"`
	References []string `json:"references,omitempty"`
}

func (s *Store) Conversation(ctx context.Context, id string) ([]ConversationTurn, error) {
	var turns []ConversationTurn
	err := s.pool.QueryRow(ctx, `select turns from remy_conversations where id=$1`, id).Scan(&turns)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return turns, err
}

func (s *Store) AppendConversation(ctx context.Context, id string, turn ConversationTurn) error {
	if turn.Role != "user" && turn.Role != "assistant" {
		return errors.New("invalid conversation role")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var turns []ConversationTurn
	err = tx.QueryRow(ctx, `select turns from remy_conversations where id=$1 for update`, id).Scan(&turns)
	if errors.Is(err, pgx.ErrNoRows) {
		turns = nil
	} else if err != nil {
		return err
	}
	turns = append(turns, turn)
	if len(turns) > 24 {
		turns = turns[len(turns)-24:]
	}
	encoded, err := json.Marshal(turns)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into remy_conversations(id,turns) values($1,$2) on conflict(id) do update set turns=excluded.turns, updated_at=now()`, id, encoded)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AppendConversationTurns(ctx context.Context, id string, newTurns ...ConversationTurn) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `insert into remy_conversations(id,turns) values($1,'[]'::jsonb) on conflict(id) do nothing`, id); err != nil {
		return err
	}
	var turns []ConversationTurn
	if err = tx.QueryRow(ctx, `select turns from remy_conversations where id=$1 for update`, id).Scan(&turns); err != nil {
		return err
	}
	for _, turn := range newTurns {
		if turn.Role != "user" && turn.Role != "assistant" {
			return errors.New("invalid conversation role")
		}
		turns = append(turns, turn)
	}
	if len(turns) > 24 {
		turns = turns[len(turns)-24:]
	}
	encoded, err := json.Marshal(turns)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update remy_conversations set turns=$2,updated_at=now() where id=$1`, id, encoded); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `create table if not exists schema_migrations (
		version text primary key,
		applied_at timestamptz not null default now()
	)`)
	if err != nil {
		return err
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		applied, err := s.migrationApplied(ctx, entry.Name())
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}

		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(ctx, `insert into schema_migrations (version) values ($1)`, entry.Name()); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `select exists(select 1 from schema_migrations where version = $1)`, version).Scan(&exists)
	return exists, err
}

func (s *Store) EnsureDefaultUser(ctx context.Context, displayName string) error {
	_, err := s.pool.Exec(ctx, `insert into users (id, display_name)
		values ('00000000-0000-0000-0000-000000000001', $1)
		on conflict (id) do update set display_name = excluded.display_name`, displayName)
	return err
}

func (s *Store) ListCategories(ctx context.Context, limit, offset int) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `select id, user_id, name, coalesce(description, '') as description, created_at, updated_at, default_tags, default_values
		from categories
		order by lower(name), created_at
		limit $1 offset $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	categories, err := pgx.CollectRows(rows, pgx.RowToStructByName[Category])
	if err != nil {
		return nil, err
	}
	if len(categories) == 0 {
		return categories, nil
	}

	for i := range categories {
		attributes, err := s.ListCategoryAttributes(ctx, categories[i].ID)
		if err != nil {
			return nil, err
		}
		categories[i].Attributes = attributes
	}

	return categories, nil
}

func (s *Store) ListCategoryAttributes(ctx context.Context, categoryID string) ([]Attribute, error) {
	rows, err := s.pool.Query(ctx, `select id, category_id, key, label, data_type, required, display_order, config_json as config
		from category_attributes
		where category_id = $1
		order by display_order, label`, categoryID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Attribute])
}

func (s *Store) GetCategoryDefinition(ctx context.Context, categoryID string) (Category, error) {
	if categoryID == "" {
		return Category{}, nil
	}
	var category Category
	err := s.pool.QueryRow(ctx, `select id, user_id, name, coalesce(description, ''), created_at, updated_at, default_tags, default_values
		from categories
		where id = $1`, categoryID).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Description,
		&category.CreatedAt,
		&category.UpdatedAt,
		&category.DefaultTags,
		&category.DefaultValues,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, err
	}

	attributes, err := s.ListCategoryAttributes(ctx, category.ID)
	if err != nil {
		return Category{}, err
	}
	category.Attributes = attributes
	return category, nil
}

func (s *Store) CreateCategoryProposal(ctx context.Context, payload CategoryProposalPayload) (Proposal, error) {
	if (payload.Operation == "update" || payload.Operation == "delete") && payload.CategoryID != "" {
		category, err := s.GetCategoryDefinition(ctx, payload.CategoryID)
		if err != nil {
			return Proposal{}, err
		}
		if strings.TrimSpace(payload.Name) == "" {
			payload.Name = category.Name
		}
		if strings.TrimSpace(payload.Description) == "" {
			payload.Description = category.Description
		}
		if payload.DefaultTags == nil {
			payload.DefaultTags = category.DefaultTags
		}
		if len(payload.DefaultValues) == 0 {
			payload.DefaultValues = category.DefaultValues
		}
		if len(payload.Attributes) == 0 {
			payload.Attributes = make([]AttributeDraft, 0, len(category.Attributes))
			for _, attribute := range category.Attributes {
				payload.Attributes = append(payload.Attributes, AttributeDraft{
					Key:          attribute.Key,
					Label:        attribute.Label,
					DataType:     attribute.DataType,
					Required:     attribute.Required,
					DisplayOrder: attribute.DisplayOrder,
					Config:       attribute.Config,
				})
			}
		}
	}
	fields, err := s.ListFields(ctx)
	if err != nil {
		return Proposal{}, err
	}
	known := map[string]FieldDefinition{}
	for _, field := range fields {
		known[field.Key] = field
	}
	for i := range payload.Attributes {
		field, ok := known[payload.Attributes[i].Key]
		if !ok {
			return Proposal{}, fmt.Errorf("define and approve field %q first", payload.Attributes[i].Key)
		}
		payload.Attributes[i].Label = field.Label
		payload.Attributes[i].DataType = field.DataType
		payload.Attributes[i].Config = field.Config
		payload.Attributes[i].Required = false
	}
	if err := validateCategoryProposalPayload(&payload); err != nil {
		return Proposal{}, err
	}
	if len(payload.DefaultValues) == 0 {
		payload.DefaultValues = json.RawMessage(`{}`)
	}
	if err := validateItemAttributeValues(payload.DefaultValues, fieldAttributes(fields)); err != nil {
		return Proposal{}, err
	}
	return s.createProposal(ctx, DefaultUserID, "category_create", payload)
}

func (s *Store) CreateItemProposal(ctx context.Context, payload ItemProposalPayload) (Proposal, error) {
	var err error
	payload, err = s.prepareItem(ctx, payload)
	if err != nil {
		return Proposal{}, err
	}
	payload, err = s.validateItemProposalPayload(ctx, payload)
	if err != nil {
		return Proposal{}, err
	}
	if payload.Operation == "update" && len(payload.PreviousAttributes) == 0 {
		current, err := s.getItem(ctx, payload.ItemID)
		if err != nil {
			return Proposal{}, err
		}
		payload.PreviousAttributes = current.Attributes
	}
	return s.createProposal(ctx, DefaultUserID, "item_change", payload)
}

func (s *Store) createProposal(ctx context.Context, userID, proposalType string, payload any) (Proposal, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return Proposal{}, err
	}

	var proposal Proposal
	err = scanProposal(s.pool.QueryRow(ctx, `insert into proposals (user_id, type, proposed_payload_jsonb)
		values ($1, $2, $3::jsonb)
		returning id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at`,
		userID, proposalType, string(payloadBytes),
	), &proposal)
	return proposal, err
}

func (s *Store) GetProposal(ctx context.Context, proposalID string) (Proposal, error) {
	var proposal Proposal
	err := scanProposal(s.pool.QueryRow(ctx, `select id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at
		from proposals
		where id = $1`, proposalID), &proposal)
	if errors.Is(err, pgx.ErrNoRows) {
		return Proposal{}, ErrNotFound
	}
	return proposal, err
}

func (s *Store) ListPendingProposals(ctx context.Context, limit int) ([]Proposal, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx, `select id,user_id,type,status,proposed_payload_jsonb as proposed_payload,coalesce(reason,'') as reason,created_at,decided_at from proposals where user_id=$1 and status='pending' order by created_at desc limit $2`, DefaultUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	proposals := []Proposal{}
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(&p.ID, &p.UserID, &p.Type, &p.Status, &p.ProposedPayload, &p.Reason, &p.CreatedAt, &p.DecidedAt); err != nil {
			return nil, err
		}
		proposals = append(proposals, p)
	}
	return proposals, rows.Err()
}

func (s *Store) RevisePendingProposal(ctx context.Context, proposalID string, payload json.RawMessage) (Proposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Proposal{}, err
	}
	defer tx.Rollback(ctx)

	var proposal Proposal
	err = scanProposal(tx.QueryRow(ctx, `select id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at
		from proposals where id = $1 for update`, proposalID), &proposal)
	if errors.Is(err, pgx.ErrNoRows) {
		return Proposal{}, ErrNotFound
	}
	if err != nil {
		return Proposal{}, err
	}
	if proposal.Status != "pending" {
		return Proposal{}, errors.New("only pending proposals can be revised")
	}
	if !json.Valid(payload) {
		return Proposal{}, errors.New("revised proposal payload must be valid JSON")
	}

	switch proposal.Type {
	case "graph_change":
		var revised GraphProposalPayload
		if err := json.Unmarshal(payload, &revised); err != nil {
			return Proposal{}, err
		}
		if err := validateGraphPayload(&revised); err != nil {
			return Proposal{}, err
		}
	case "category_create":
		var revised CategoryProposalPayload
		if err := json.Unmarshal(payload, &revised); err != nil {
			return Proposal{}, errors.New("revised category proposal must be valid")
		}
		if err := validateCategoryProposalPayload(&revised); err != nil {
			return Proposal{}, fmt.Errorf("revised category proposal: %w", err)
		}
	case "item_change":
		var revised ItemProposalPayload
		if err := json.Unmarshal(payload, &revised); err != nil {
			return Proposal{}, errors.New("revised item proposal must be valid")
		}
		if revised.CategoryID != "" {
			return Proposal{}, errors.New("items have no template membership; revise their tags and registered values directly")
		}
		values := map[string]any{}
		if err := json.Unmarshal(revised.Attributes, &values); err != nil {
			return Proposal{}, err
		}
		labelTags := revised.Tags
		if revised.ItemID != "" && labelTags == nil {
			item, err := s.getItem(ctx, revised.ItemID)
			if err != nil {
				return Proposal{}, err
			}
			labelTags = item.Tags
		}
		revised.Title = DisplayLabel(values, labelTags)
		normalized, err := s.validateItemProposalPayload(ctx, revised)
		if err != nil {
			return Proposal{}, fmt.Errorf("revised item proposal: %w", err)
		}
		payload, err = json.Marshal(normalized)
		if err != nil {
			return Proposal{}, err
		}
	default:
		return Proposal{}, fmt.Errorf("unsupported proposal type %q", proposal.Type)
	}

	err = scanProposal(tx.QueryRow(ctx, `update proposals set proposed_payload_jsonb = $2::jsonb
		where id = $1
		returning id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at`,
		proposalID, string(payload)), &proposal)
	if err != nil {
		return Proposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}

func (s *Store) DecideProposal(ctx context.Context, proposalID string, decision ProposalDecision) (Proposal, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Proposal{}, err
	}
	defer tx.Rollback(ctx)

	var proposal Proposal
	err = scanProposal(tx.QueryRow(ctx, `select id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at
		from proposals
		where id = $1
		for update`, proposalID), &proposal)
	if errors.Is(err, pgx.ErrNoRows) {
		return Proposal{}, ErrNotFound
	}
	if err != nil {
		return Proposal{}, err
	}
	if proposal.Status != "pending" {
		return Proposal{}, errors.New("proposal has already been decided")
	}

	status := "rejected"
	if decision.Approve {
		status = "approved"
		if err := s.commitProposal(ctx, tx, &proposal); err != nil {
			return Proposal{}, err
		}
	}

	err = scanProposal(tx.QueryRow(ctx, `update proposals
		set status = $2, reason = nullif($3, ''), decided_at = now()
		where id = $1
		returning id, user_id, type, status, proposed_payload_jsonb, coalesce(reason, ''), created_at, decided_at`,
		proposal.ID, status, decision.Reason,
	), &proposal)
	if err != nil {
		return Proposal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}

func (s *Store) commitProposal(ctx context.Context, tx pgx.Tx, proposal *Proposal) error {
	switch proposal.Type {
	case "graph_change":
		var payload GraphProposalPayload
		if err := json.Unmarshal(proposal.ProposedPayload, &payload); err != nil {
			return err
		}
		return applyGraphChange(ctx, tx, payload)
	case "category_create":
		var payload CategoryProposalPayload
		if err := json.Unmarshal(proposal.ProposedPayload, &payload); err != nil {
			return err
		}
		return applyCategoryChange(ctx, tx, proposal.UserID, payload)
	case "item_change":
		var payload ItemProposalPayload
		if err := json.Unmarshal(proposal.ProposedPayload, &payload); err != nil {
			return err
		}
		fields, err := readFields(ctx, tx)
		if err != nil {
			return err
		}
		if payload.Operation == "create" || payload.Operation == "update" {
			if err := validateItemAttributeValues(payload.Attributes, fieldAttributes(fields)); err != nil {
				return err
			}
		}
		itemID, err := applyItemChange(ctx, tx, proposal.UserID, payload)
		if err != nil {
			return err
		}
		if itemID != "" && payload.ItemID != itemID {
			payload.ItemID = itemID
			encoded, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `update proposals set proposed_payload_jsonb = $2::jsonb where id = $1`, proposal.ID, string(encoded)); err != nil {
				return err
			}
			proposal.ProposedPayload = encoded
		}
		return nil
	default:
		return fmt.Errorf("unsupported proposal type %q", proposal.Type)
	}
}

func validateCategoryProposalPayload(payload *CategoryProposalPayload) error {
	if payload.Operation == "" {
		payload.Operation = "create"
	}
	switch payload.Operation {
	case "create":
		if strings.TrimSpace(payload.Name) == "" {
			return errors.New("category name is required")
		}
	case "update":
		if payload.CategoryID == "" || strings.TrimSpace(payload.Name) == "" {
			return errors.New("category_id and name are required for update")
		}
	case "delete":
		if payload.CategoryID == "" {
			return errors.New("category_id is required for delete")
		}
		return nil
	default:
		return errors.New("operation must be create, update, or delete")
	}
	seenKeys := map[string]bool{}
	for i := range payload.Attributes {
		attribute := &payload.Attributes[i]
		if seenKeys[attribute.Key] {
			return fmt.Errorf("field %q is repeated", attribute.Key)
		}
		seenKeys[attribute.Key] = true
		if attribute.Key == "" || attribute.Label == "" || attribute.DataType == "" {
			return errors.New("attribute key, label, and data_type are required")
		}
		switch attribute.DataType {
		case "text", "number", "boolean", "date", "url", "links":
		case "enum":
			var config struct {
				Options []string `json:"options"`
			}
			if err := json.Unmarshal(attribute.Config, &config); err != nil || len(config.Options) == 0 {
				return fmt.Errorf("enum attribute %q requires config.options", attribute.Key)
			}
			seen := map[string]bool{}
			for _, option := range config.Options {
				if strings.TrimSpace(option) == "" || seen[option] {
					return fmt.Errorf("enum attribute %q options must be non-empty and unique", attribute.Key)
				}
				seen[option] = true
			}
		default:
			return fmt.Errorf("attribute %q has unsupported data_type %q", attribute.Key, attribute.DataType)
		}
		if len(attribute.Config) == 0 {
			attribute.Config = json.RawMessage(`{}`)
		} else if !json.Valid(attribute.Config) {
			return fmt.Errorf("attribute %q config must be valid JSON", attribute.Key)
		}
	}
	return nil
}

func (s *Store) validateItemProposalPayload(ctx context.Context, payload ItemProposalPayload) (ItemProposalPayload, error) {
	if payload.Operation == "" {
		payload.Operation = "create"
	}
	if len(payload.Attributes) == 0 {
		payload.Attributes = json.RawMessage(`{}`)
	}
	if payload.Operation == "create" && payload.Quantity == 0 {
		payload.Quantity = 1
	}
	switch payload.Operation {
	case "create", "update", "delete", "quantity_adjust":
	default:
		return ItemProposalPayload{}, errors.New("operation must be create, update, delete, or quantity_adjust")
	}
	if payload.Operation != "create" && payload.ItemID == "" {
		return ItemProposalPayload{}, errors.New("item_id is required for update, delete, and quantity_adjust operations")
	}
	if payload.Tags != nil {
		cleaned := []string{}
		seen := map[string]bool{}
		for _, tag := range payload.Tags {
			tag = strings.TrimSpace(tag)
			if len(tag) > 80 {
				return ItemProposalPayload{}, errors.New("tags must be at most 80 characters")
			}
			if tag != "" && !seen[strings.ToLower(tag)] {
				cleaned = append(cleaned, tag)
				seen[strings.ToLower(tag)] = true
			}
		}
		if len(cleaned) > 50 {
			return ItemProposalPayload{}, errors.New("at most 50 tags are allowed")
		}
		payload.Tags = cleaned
	}
	if payload.Quantity < 0 {
		return ItemProposalPayload{}, errors.New("quantity must be positive")
	}
	if payload.PrimaryImageID != nil {
		if payload.ItemID == "" {
			return payload, errors.New("save the item and upload its photo before choosing a primary image")
		}
		picture, err := s.GetItemImage(ctx, *payload.PrimaryImageID)
		if err != nil || picture.ItemID != payload.ItemID {
			return payload, errors.New("primary picture must belong to this item")
		}
	}
	if payload.Operation == "create" || payload.Operation == "update" {
		fields, err := s.ListFields(ctx)
		if err != nil {
			return payload, err
		}
		if err := validateItemAttributeValues(payload.Attributes, fieldAttributes(fields)); err != nil {
			return payload, err
		}
	}

	return payload, nil
}

func validateItemAttributeValues(raw json.RawMessage, definitions []Attribute) error {
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return errors.New("attributes must be a JSON object")
	}
	defined := make(map[string]Attribute, len(definitions))
	for _, definition := range definitions {
		defined[definition.Key] = definition
	}
	for key, value := range values {
		definition, ok := defined[key]
		if !ok {
			return fmt.Errorf("field %q is not defined; propose and approve its definition first", key)
		}
		valid := false
		switch definition.DataType {
		case "text", "url":
			_, valid = value.(string)
		case "date":
			if text, ok := value.(string); ok {
				_, err := time.Parse("2006-01-02", text)
				valid = err == nil
			}
		case "number":
			_, valid = value.(float64)
		case "boolean":
			_, valid = value.(bool)
		case "json":
			valid = true
		case "links":
			links, ok := value.([]any)
			valid = ok
			for _, rawLink := range links {
				link, ok := rawLink.(map[string]any)
				if !ok {
					valid = false
					break
				}
				name, named := link["name"].(string)
				address, addressed := link["url"].(string)
				parsed, err := url.Parse(address)
				if !named || strings.TrimSpace(name) == "" || !addressed || err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
					valid = false
					break
				}
			}
		case "enum":
			stringValue, isString := value.(string)
			var config struct {
				Options []string `json:"options"`
			}
			_ = json.Unmarshal(definition.Config, &config)
			for _, option := range config.Options {
				if isString && stringValue == option {
					valid = true
					break
				}
			}
		}
		if !valid {
			return fmt.Errorf("attribute %q is not a valid %s value", key, definition.DataType)
		}
	}
	return nil
}

func applyCategoryChange(ctx context.Context, tx pgx.Tx, userID string, payload CategoryProposalPayload) error {
	if payload.Operation != "delete" {
		fields, err := readFields(ctx, tx)
		if err != nil {
			return err
		}
		known := map[string]FieldDefinition{}
		for _, field := range fields {
			known[field.Key] = field
		}
		for i := range payload.Attributes {
			field, ok := known[payload.Attributes[i].Key]
			if !ok {
				return fmt.Errorf("field %q must be defined and approved first", payload.Attributes[i].Key)
			}
			payload.Attributes[i].Label = field.Label
			payload.Attributes[i].DataType = field.DataType
			payload.Attributes[i].Config = field.Config
			payload.Attributes[i].Required = false
		}
		if len(payload.DefaultValues) == 0 {
			payload.DefaultValues = json.RawMessage(`{}`)
		}
		if err := validateItemAttributeValues(payload.DefaultValues, fieldAttributes(fields)); err != nil {
			return err
		}
	}
	switch payload.Operation {
	case "create":
		return insertCategory(ctx, tx, userID, payload)
	case "update":
		return updateCategory(ctx, tx, userID, payload)
	case "delete":
		tag, err := tx.Exec(ctx, `delete from categories where id = $1 and user_id = $2`, payload.CategoryID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	default:
		return fmt.Errorf("unsupported category operation %q", payload.Operation)
	}
}

func insertCategory(ctx context.Context, tx pgx.Tx, userID string, payload CategoryProposalPayload) error {
	var categoryID string
	err := tx.QueryRow(ctx, `insert into categories (user_id, name, description,default_tags,default_values)
		values ($1, $2, nullif($3, ''),$4,$5)
		returning id`, userID, payload.Name, payload.Description, nonNilTags(payload.DefaultTags), payload.DefaultValues).Scan(&categoryID)
	if err != nil {
		return err
	}
	for i, attr := range payload.Attributes {
		order := attr.DisplayOrder
		if order == 0 {
			order = i + 1
		}
		config := attr.Config
		if len(config) == 0 {
			config = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(ctx, `insert into category_attributes
			(category_id, key, label, data_type, required, display_order, config_json)
			values ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
			categoryID, attr.Key, attr.Label, attr.DataType, attr.Required, order, string(config),
		); err != nil {
			return err
		}
	}
	return nil
}

func updateCategory(ctx context.Context, tx pgx.Tx, userID string, payload CategoryProposalPayload) error {
	tag, err := tx.Exec(ctx, `update categories set name = $3, description = nullif($4, ''), default_tags=$5,default_values=$6,updated_at = now()
		where id = $1 and user_id = $2`, payload.CategoryID, userID, payload.Name, payload.Description, nonNilTags(payload.DefaultTags), payload.DefaultValues)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `delete from category_attributes where category_id = $1`, payload.CategoryID); err != nil {
		return err
	}
	for i, attr := range payload.Attributes {
		order := attr.DisplayOrder
		if order == 0 {
			order = i + 1
		}
		config := attr.Config
		if len(config) == 0 {
			config = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(ctx, `insert into category_attributes
			(category_id, key, label, data_type, required, display_order, config_json)
			values ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
			payload.CategoryID, attr.Key, attr.Label, attr.DataType, attr.Required, order, string(config)); err != nil {
			return err
		}
	}
	return nil
}

func applyItemChange(ctx context.Context, tx pgx.Tx, userID string, payload ItemProposalPayload) (string, error) {
	attributes := payload.Attributes
	if len(attributes) == 0 {
		attributes = json.RawMessage(`{}`)
	}
	payload.CategoryID = ""
	if payload.Operation == "create" || payload.Operation == "update" {
		values := map[string]any{}
		if err := json.Unmarshal(attributes, &values); err != nil {
			return "", err
		}
		labelTags := payload.Tags
		if payload.Operation == "update" && labelTags == nil {
			if err := tx.QueryRow(ctx, `select tags from items where id=$1 and user_id=$2 for update`, payload.ItemID, userID).Scan(&labelTags); err != nil {
				return "", err
			}
		}
		payload.Title = DisplayLabel(values, labelTags)
	}
	quantity := 1

	switch payload.Operation {
	case "create":
		var itemID string
		err := tx.QueryRow(ctx, `insert into items (user_id, category_id, title, attributes_jsonb, quantity, tags, location)
			values ($1, nullif($2,'')::uuid, $3, $4::jsonb, $5, $6, $7) returning id`, userID, payload.CategoryID, payload.Title, string(attributes), quantity, nonNilTags(payload.Tags), locationValue(payload.Location)).Scan(&itemID)
		return itemID, err
	case "update":
		tag, err := tx.Exec(ctx, `update items
			set title = $3, attributes_jsonb = $4::jsonb, quantity = $5, tags = coalesce($6, tags), location = coalesce($7, location), category_id = nullif($8,'')::uuid, updated_at = now()
			where id = $1 and user_id = $2`, payload.ItemID, userID, payload.Title, string(attributes), quantity, payload.Tags, payload.Location, payload.CategoryID)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "", ErrNotFound
		}
		if payload.PrimaryImageID != nil {
			var belongs bool
			if err := tx.QueryRow(ctx, `select exists(select 1 from item_assets where id=$1 and item_id=$2)`, *payload.PrimaryImageID, payload.ItemID).Scan(&belongs); err != nil {
				return "", err
			}
			if !belongs {
				return "", errors.New("primary picture must belong to this item")
			}
			if _, err := tx.Exec(ctx, `update item_assets set is_primary=false where item_id=$1`, payload.ItemID); err != nil {
				return "", err
			}
			if _, err := tx.Exec(ctx, `update item_assets set is_primary=true where id=$1`, *payload.PrimaryImageID); err != nil {
				return "", err
			}
		}
		return payload.ItemID, nil
	case "quantity_adjust":
		delta := payload.QuantityDelta
		if delta == 0 {
			delta = 1
		}
		tag, err := tx.Exec(ctx, `update items
			set quantity = quantity + $3, updated_at = now()
			where id = $1 and user_id = $2 and quantity + $3 > 0`, payload.ItemID, userID, delta)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "", ErrNotFound
		}
		return payload.ItemID, nil
	case "delete":
		tag, err := tx.Exec(ctx, `delete from items where id = $1 and user_id = $2`, payload.ItemID, userID)
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "", ErrNotFound
		}
		return "", nil
	default:
		return "", fmt.Errorf("unsupported item operation %q", payload.Operation)
	}
}

func (s *Store) ListItems(ctx context.Context, categoryID string, limit, offset int) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `select id, user_id, coalesce(category_id::text,'') as category_id, title, attributes_jsonb as attributes, quantity, created_at, updated_at, tags, location
		from items
		where category_id = $1
		order by lower(title), created_at
		limit $2 offset $3`, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Item])
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Images, err = s.ListItemImages(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

// SearchInventory performs filtering, ordering, pagination, and exact counting in Postgres.
// The agent receives only the requested page, even when the inventory is large.
func (s *Store) SearchInventory(ctx context.Context, query, categoryID, attributeKey, attributeValue string, limit, offset int) ([]Item, int, error) {
	return s.SearchLibrary(ctx, query, categoryID, attributeKey, attributeValue, "", "", limit, offset)
}
func (s *Store) SearchLibrary(ctx context.Context, query, categoryID, attributeKey, attributeValue, tag, location string, limit, offset int) ([]Item, int, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := `($1 = '' or lower(i.title || ' ' || i.attributes_jsonb::text || ' ' || array_to_string(i.tags, ' ')) like '%' || lower($1) || '%')
		and ($2 = '' or i.category_id = nullif($2,'')::uuid)
		and ($3 = '' or lower(coalesce(i.attributes_jsonb ->> $3, '')) = lower($4)) and ($5 = '' or i.tags @> string_to_array($5, ',')) and ($6 = '' or i.attributes_jsonb->>'location' = $6)`
	var total int
	if err := s.pool.QueryRow(ctx, `select count(*) from items i where `+where, query, categoryID, attributeKey, attributeValue, tag, location).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `select i.id, i.user_id, coalesce(i.category_id::text,'') as category_id, i.title, i.attributes_jsonb as attributes, i.quantity, i.created_at, i.updated_at, i.tags, i.location
		from items i where `+where+` order by lower(i.title), i.created_at, i.id limit $7 offset $8`, query, categoryID, attributeKey, attributeValue, tag, location, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Item])
	if err != nil {
		return nil, 0, err
	}
	if err := s.HydrateItemImages(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// HydrateItemImages loads pictures for a bounded item page with one query.
func (s *Store) HydrateItemImages(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	rows, err := s.pool.Query(ctx, `select id,item_id,object_key as original_key,coalesce(thumbnail_object_key,'') as thumbnail_key,mime_type,original_filename,width,height,size_bytes,created_at,is_primary from item_assets where item_id::text=any($1::text[]) order by is_primary desc,created_at,id`, ids)
	if err != nil {
		return err
	}
	images, err := pgx.CollectRows(rows, pgx.RowToStructByName[ItemImage])
	if err != nil {
		return err
	}
	byItem := map[string][]ItemImage{}
	for _, image := range images {
		byItem[image.ItemID] = append(byItem[image.ItemID], image)
	}
	for i := range items {
		items[i].Images = byItem[items[i].ID]
	}
	return nil
}

// AggregateInventory computes deterministic inventory summaries in Postgres.
func (s *Store) AggregateInventory(ctx context.Context, categoryID, metric, attributeKey, filterKey, filterValue string) (map[string]any, error) {
	where := `($1 = '' or category_id = nullif($1,'')::uuid) and ($2 = '' or lower(coalesce(attributes_jsonb ->> $2, '')) = lower($3))`
	result := map[string]any{}
	var records int
	if err := s.pool.QueryRow(ctx, `select count(*) from items where `+where, categoryID, filterKey, filterValue).Scan(&records); err != nil {
		return nil, err
	}
	result["records"] = records
	switch metric {
	case "records":
	case "units":
		var n float64
		if err := s.pool.QueryRow(ctx, `select coalesce(sum(case when jsonb_typeof(attributes_jsonb->'quantity')='number' then (attributes_jsonb->>'quantity')::numeric else 0 end),0) from items where `+where, categoryID, filterKey, filterValue).Scan(&n); err != nil {
			return nil, err
		}
		result["units"] = n
	case "min", "max":
		var n *float64
		fn := "min"
		if metric == "max" {
			fn = "max"
		}
		err := s.pool.QueryRow(ctx, `select `+fn+`(nullif(attributes_jsonb ->> $4,'')::numeric) from items where `+where, categoryID, filterKey, filterValue, attributeKey).Scan(&n)
		if err != nil {
			return nil, err
		}
		if n == nil {
			return nil, fmt.Errorf("no numeric values for %s", attributeKey)
		}
		result[metric] = *n
	case "group":
		return nil, errors.New("group metric requires grouped query")
	default:
		return nil, errors.New("unsupported aggregate metric")
	}
	return result, nil
}

func (s *Store) GroupInventory(ctx context.Context, categoryID, groupBy, filterKey, filterValue string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `select coalesce(attributes_jsonb ->> $2, 'Unspecified'), count(*) from items
		where ($1 = '' or category_id = nullif($1,'')::uuid) and ($3 = '' or lower(coalesce(attributes_jsonb ->> $3, '')) = lower($4)) group by 1 order by 1`, categoryID, groupBy, filterKey, filterValue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		out[key] = count
	}
	return out, rows.Err()
}

func (s *Store) GetItem(ctx context.Context, itemID string) (Item, error) {
	item, err := s.getItem(ctx, itemID)
	if err != nil {
		return Item{}, err
	}
	item.Images, err = s.ListItemImages(ctx, item.ID)
	return item, err
}

func (s *Store) ListItemImages(ctx context.Context, itemID string) ([]ItemImage, error) {
	rows, err := s.pool.Query(ctx, `select id, item_id, object_key as original_key,
		coalesce(thumbnail_object_key, '') as thumbnail_key, mime_type, original_filename,
		width, height, size_bytes, created_at, is_primary
		from item_assets where item_id = $1 order by is_primary desc, created_at, id`, itemID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ItemImage])
}

func (s *Store) CreateItemImage(ctx context.Context, picture ItemImage, checksum string) (ItemImage, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return picture, err
	}
	defer tx.Rollback(ctx)
	var itemID string
	if err := tx.QueryRow(ctx, `select id from items where id=$1 for update`, picture.ItemID).Scan(&itemID); err != nil {
		return picture, err
	}
	err = tx.QueryRow(ctx, `insert into item_assets(id,item_id,object_key,thumbnail_object_key,mime_type,original_filename,width,height,size_bytes,metadata_jsonb,is_primary)
 values($1,$2,$3,$4,$5,$6,$7,$8,$9,jsonb_build_object('sha256',$10::text),not exists(select 1 from item_assets where item_id=$2 and is_primary)) returning created_at,is_primary`, picture.ID, picture.ItemID, picture.OriginalKey, picture.ThumbnailKey, picture.MIMEType, picture.OriginalFilename, picture.Width, picture.Height, picture.SizeBytes, checksum).Scan(&picture.CreatedAt, &picture.IsPrimary)
	if err != nil {
		return picture, err
	}
	return picture, tx.Commit(ctx)
}

func (s *Store) GetItemImage(ctx context.Context, imageID string) (ItemImage, error) {
	var image ItemImage
	err := s.pool.QueryRow(ctx, `select a.id, a.item_id, a.object_key, coalesce(a.thumbnail_object_key, ''),
		a.mime_type, a.original_filename, a.width, a.height, a.size_bytes, a.created_at,a.is_primary
		from item_assets a join items i on i.id = a.item_id
		where a.id = $1 and i.user_id = $2`, imageID, DefaultUserID).Scan(
		&image.ID, &image.ItemID, &image.OriginalKey, &image.ThumbnailKey, &image.MIMEType,
		&image.OriginalFilename, &image.Width, &image.Height, &image.SizeBytes, &image.CreatedAt, &image.IsPrimary)
	if errors.Is(err, pgx.ErrNoRows) {
		return ItemImage{}, ErrNotFound
	}
	return image, err
}

func (s *Store) getItem(ctx context.Context, itemID string) (Item, error) {
	var item Item
	err := s.pool.QueryRow(ctx, `select id, user_id, coalesce(category_id::text,'') as category_id, title, attributes_jsonb as attributes, quantity, created_at, updated_at, tags, location
		from items where id = $1 and user_id = $2`, itemID, DefaultUserID).Scan(
		&item.ID,
		&item.UserID,
		&item.CategoryID,
		&item.Title,
		&item.Attributes,
		&item.Quantity,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Tags,
		&item.Location,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProposal(row rowScanner, proposal *Proposal) error {
	var decidedAt pgtype.Timestamptz
	if err := row.Scan(
		&proposal.ID,
		&proposal.UserID,
		&proposal.Type,
		&proposal.Status,
		&proposal.ProposedPayload,
		&proposal.Reason,
		&proposal.CreatedAt,
		&decidedAt,
	); err != nil {
		return err
	}
	if decidedAt.Valid {
		proposal.DecidedAt = &decidedAt.Time
	} else {
		proposal.DecidedAt = nil
	}
	return nil
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
func locationValue(location *string) string {
	if location == nil {
		return ""
	}
	return strings.TrimSpace(*location)
}

func (s *Store) ItemImageByChecksum(ctx context.Context, itemID, checksum string) (ItemImage, error) {
	var id string
	err := s.pool.QueryRow(ctx, `select id from item_assets where item_id=$1 and metadata_jsonb->>'sha256'=$2`, itemID, checksum).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ItemImage{}, ErrNotFound
	}
	if err != nil {
		return ItemImage{}, err
	}
	return s.GetItemImage(ctx, id)
}
