package store

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	pool *pgxpool.Pool
}

type Category struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Attributes  []Attribute `json:"attributes,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Attribute struct {
	ID           string          `json:"id"`
	CategoryID   string          `json:"category_id"`
	Key          string          `json:"key"`
	Label        string          `json:"label"`
	DataType     string          `json:"data_type"`
	Required     bool            `json:"required"`
	DisplayOrder int             `json:"display_order"`
	Config       json.RawMessage `json:"config,omitempty"`
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
	rows, err := s.pool.Query(ctx, `select id, user_id, name, coalesce(description, ''), created_at, updated_at
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
