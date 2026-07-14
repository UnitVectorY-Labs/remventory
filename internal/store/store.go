package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
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
	ID          string      `json:"id" db:"id"`
	UserID      string      `json:"user_id" db:"user_id"`
	Name        string      `json:"name" db:"name"`
	Description string      `json:"description,omitempty" db:"description"`
	Attributes  []Attribute `json:"attributes,omitempty" db:"-"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
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
	ID         string          `json:"id" db:"id"`
	UserID     string          `json:"user_id" db:"user_id"`
	CategoryID string          `json:"category_id" db:"category_id"`
	Title      string          `json:"title" db:"title"`
	Attributes json.RawMessage `json:"attributes" db:"attributes"`
	Quantity   int             `json:"quantity" db:"quantity"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
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
	Operation   string           `json:"operation"`
	CategoryID  string           `json:"category_id,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Attributes  []AttributeDraft `json:"attributes"`
}

type ItemProposalPayload struct {
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
	rows, err := s.pool.Query(ctx, `select id, user_id, name, coalesce(description, '') as description, created_at, updated_at
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
	var category Category
	err := s.pool.QueryRow(ctx, `select id, user_id, name, coalesce(description, ''), created_at, updated_at
		from categories
		where id = $1`, categoryID).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Description,
		&category.CreatedAt,
		&category.UpdatedAt,
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
	if err := validateCategoryProposalPayload(&payload); err != nil {
		return Proposal{}, err
	}
	if payload.Operation == "delete" && strings.TrimSpace(payload.Name) == "" {
		category, err := s.GetCategoryDefinition(ctx, payload.CategoryID)
		if err != nil {
			return Proposal{}, err
		}
		payload.Name = category.Name
		payload.Description = category.Description
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
	return s.createProposal(ctx, DefaultUserID, "category_create", payload)
}

func (s *Store) CreateItemProposal(ctx context.Context, payload ItemProposalPayload) (Proposal, error) {
	var err error
	payload, err = s.validateItemProposalPayload(payload)
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
		if _, err := s.validateItemProposalPayload(revised); err != nil {
			return Proposal{}, fmt.Errorf("revised item proposal: %w", err)
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
		if err := s.commitProposal(ctx, tx, proposal); err != nil {
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

func (s *Store) commitProposal(ctx context.Context, tx pgx.Tx, proposal Proposal) error {
	switch proposal.Type {
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
		return applyItemChange(ctx, tx, proposal.UserID, payload)
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
	for i := range payload.Attributes {
		if payload.Attributes[i].Key == "" || payload.Attributes[i].Label == "" || payload.Attributes[i].DataType == "" {
			return errors.New("attribute key, label, and data_type are required")
		}
		if len(payload.Attributes[i].Config) == 0 {
			payload.Attributes[i].Config = json.RawMessage(`{}`)
		}
	}
	return nil
}

func (s *Store) validateItemProposalPayload(payload ItemProposalPayload) (ItemProposalPayload, error) {
	if payload.Operation == "" {
		payload.Operation = "create"
	}
	if payload.CategoryID == "" {
		return ItemProposalPayload{}, errors.New("category_id is required")
	}
	if payload.Operation != "delete" && strings.TrimSpace(payload.Title) == "" {
		return ItemProposalPayload{}, errors.New("title is required")
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
	return payload, nil
}

func applyCategoryChange(ctx context.Context, tx pgx.Tx, userID string, payload CategoryProposalPayload) error {
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
	err := tx.QueryRow(ctx, `insert into categories (user_id, name, description)
		values ($1, $2, nullif($3, ''))
		returning id`, userID, payload.Name, payload.Description).Scan(&categoryID)
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
	tag, err := tx.Exec(ctx, `update categories set name = $3, description = nullif($4, ''), updated_at = now()
		where id = $1 and user_id = $2`, payload.CategoryID, userID, payload.Name, payload.Description)
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

func applyItemChange(ctx context.Context, tx pgx.Tx, userID string, payload ItemProposalPayload) error {
	attributes := payload.Attributes
	if len(attributes) == 0 {
		attributes = json.RawMessage(`{}`)
	}
	quantity := payload.Quantity
	if quantity == 0 {
		quantity = 1
	}

	switch payload.Operation {
	case "create":
		_, err := tx.Exec(ctx, `insert into items (user_id, category_id, title, attributes_jsonb, quantity)
			values ($1, $2, $3, $4::jsonb, $5)`, userID, payload.CategoryID, payload.Title, string(attributes), quantity)
		return err
	case "update":
		tag, err := tx.Exec(ctx, `update items
			set title = $3, attributes_jsonb = $4::jsonb, quantity = $5, updated_at = now()
			where id = $1 and user_id = $2`, payload.ItemID, userID, payload.Title, string(attributes), quantity)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	case "quantity_adjust":
		delta := payload.QuantityDelta
		if delta == 0 {
			delta = 1
		}
		tag, err := tx.Exec(ctx, `update items
			set quantity = quantity + $3, updated_at = now()
			where id = $1 and user_id = $2 and quantity + $3 > 0`, payload.ItemID, userID, delta)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	case "delete":
		tag, err := tx.Exec(ctx, `delete from items where id = $1 and user_id = $2`, payload.ItemID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	default:
		return fmt.Errorf("unsupported item operation %q", payload.Operation)
	}
}

func (s *Store) ListItems(ctx context.Context, categoryID string, limit, offset int) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `select id, user_id, category_id, title, attributes_jsonb as attributes, quantity, created_at, updated_at
		from items
		where category_id = $1
		order by lower(title), created_at
		limit $2 offset $3`, categoryID, limit, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Item])
}

func (s *Store) getItem(ctx context.Context, itemID string) (Item, error) {
	var item Item
	err := s.pool.QueryRow(ctx, `select id, user_id, category_id, title, attributes_jsonb as attributes, quantity, created_at, updated_at
		from items where id = $1 and user_id = $2`, itemID, DefaultUserID).Scan(
		&item.ID,
		&item.UserID,
		&item.CategoryID,
		&item.Title,
		&item.Attributes,
		&item.Quantity,
		&item.CreatedAt,
		&item.UpdatedAt,
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
