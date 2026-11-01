package store

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestGraphMigrationPreservesExistingCatalog(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for Postgres integration")
	}
	ctx := context.Background()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(ctx)
	schema := "migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := connection.Exec(ctx, `create schema `+schema); err != nil {
		t.Fatal(err)
	}
	defer connection.Exec(ctx, `drop schema `+schema+` cascade`)
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	repo, err := Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if _, err := repo.pool.Exec(ctx, `create table schema_migrations(version text primary key,applied_at timestamptz default now())`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_initial.sql", "002_item_images.sql", "003_remy_conversations.sql", "004_item_organization.sql"} {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.pool.Exec(ctx, string(content)); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.pool.Exec(ctx, `insert into schema_migrations(version) values($1)`, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.EnsureDefaultUser(ctx, "Migration test"); err != nil {
		t.Fatal(err)
	}
	var categoryID, itemID string
	if err := repo.pool.QueryRow(ctx, `insert into categories(user_id,name) values($1,'Games') returning id`, DefaultUserID).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, `insert into category_attributes(category_id,key,label,data_type,required) values($1,'publisher','Publisher','text',true)`, categoryID); err != nil {
		t.Fatal(err)
	}
	if err := repo.pool.QueryRow(ctx, `insert into items(user_id,category_id,title,quantity,location,tags,attributes_jsonb) values($1,$2,'Wii Sports',2,'Den',array['Wii'],'{"publisher":"Nintendo","edition":"Original"}') returning id`, DefaultUserID, categoryID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, `insert into item_assets(item_id,object_key,mime_type) values($1,'front','image/png'),($1,'back','image/png')`, itemID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.Migrate(ctx); err != nil {
		t.Fatal("repeated migration:", err)
	}
	item, err := repo.GetItem(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	json.Unmarshal(item.Attributes, &values)
	if item.CategoryID != "" || values["title"] != "Wii Sports" || values["location"] != "Den" || values["quantity"] != float64(2) || values["edition"] != "Original" {
		t.Fatalf("legacy catalog data changed: %+v %v", item, values)
	}
	if len(item.Tags) != 2 || item.Tags[0] != "Wii" || item.Tags[1] != "Games" {
		t.Fatalf("old category organization lost: %v", item.Tags)
	}
	if len(item.Images) != 2 || !item.Images[0].IsPrimary || item.Images[1].IsPrimary {
		t.Fatal("migration did not select one primary picture")
	}
	vocabulary, err := repo.Vocabulary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vocabulary.TagFields["Games"]) != 1 || vocabulary.TagFields["Games"][0] != "publisher" {
		t.Fatal("category suggestions were not preserved")
	}
	template, err := repo.GetCategoryDefinition(ctx, categoryID)
	if err != nil {
		t.Fatal(err)
	}
	if len(template.DefaultTags) != 1 || template.DefaultTags[0] != "Games" || template.Attributes[0].Required {
		t.Fatal("legacy template did not become an optional creation shortcut")
	}
	if _, err := repo.CreateItemProposal(ctx, ItemProposalPayload{Operation: "update", ItemID: item.ID, Attributes: item.Attributes, Tags: item.Tags}); err != nil {
		t.Fatal("migrated field values cannot be edited:", err)
	}
}
