package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/itemimages"
	"github.com/UnitVectorY-Labs/remventory/internal/remy"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type memoryObjects struct {
	data  map[string][]byte
	types map[string]string
}

func (m *memoryObjects) Put(_ context.Context, k, typ string, b []byte) error {
	m.data[k] = append([]byte{}, b...)
	m.types[k] = typ
	return nil
}
func (m *memoryObjects) Delete(_ context.Context, k string) error { delete(m.data, k); return nil }
func (m *memoryObjects) Get(_ context.Context, k string) (itemimages.Object, error) {
	b, ok := m.data[k]
	if !ok {
		return itemimages.Object{}, fmt.Errorf("not found")
	}
	return itemimages.Object{Body: io.NopCloser(bytes.NewReader(b)), ContentType: m.types[k], Size: int64(len(b))}, nil
}

// Every run gets a fresh schema; this never removes existing application data.
func TestLibraryApprovalAndPhotos(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for Postgres integration")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := conn.Exec(ctx, `create schema `+schema); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, `drop schema `+schema+` cascade`)
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	repo, err := store.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureDefaultUser(ctx, "Test librarian"); err != nil {
		t.Fatal(err)
	}
	objects := &memoryObjects{data: map[string][]byte{}, types: map[string]string{}}
	handler := New(Options{Config: config.Config{}, Store: repo, Remy: remy.New(config.Config{}, repo), Images: objects, Logger: slog.Default()})
	call := func(method, path string, payload any) map[string]any {
		t.Helper()
		var body io.Reader
		if payload != nil {
			b, _ := json.Marshal(payload)
			body = bytes.NewReader(b)
		}
		request := httptest.NewRequest(method, path, body)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code < 200 || response.Code >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	expectBad := func(path string, payload any) {
		t.Helper()
		body, _ := json.Marshal(payload)
		request := httptest.NewRequest("POST", path, bytes.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 400 {
			t.Fatalf("expected validation failure: %s", response.Body.String())
		}
	}
	expectBad("/api/proposals/item", map[string]any{"attributes": map[string]any{"purchase_price": 20}})
	fieldProposal := call("POST", "/api/proposals/graph", map[string]any{"operation": "define_field", "key": "purchase_price", "label": "Purchase price", "data_type": "number"})
	expectBad("/api/proposals/item", map[string]any{"attributes": map[string]any{"purchase_price": 20}})
	call("POST", "/api/proposals/"+fieldProposal["id"].(string)+"/decision", map[string]bool{"approve": true})
	tagProposal := call("POST", "/api/proposals/graph", map[string]any{"operation": "suggest_fields", "tag": "Games", "field_keys": []string{"title", "year", "links", "purchase_price"}})
	vocab := call("GET", "/api/vocabulary", nil)
	if len(vocab["tag_fields"].(map[string]any)) != 0 {
		t.Fatal("tag association changed before approval")
	}
	call("POST", "/api/proposals/"+tagProposal["id"].(string)+"/decision", map[string]bool{"approve": true})
	vocab = call("GET", "/api/vocabulary", nil)
	if len(vocab["tag_fields"].(map[string]any)["Games"].([]any)) != 4 {
		t.Fatal("tag field association missing")
	}
	expectBad("/api/proposals/item", map[string]any{"attributes": map[string]any{"purchase_price": "not a number"}})
	expectBad("/api/proposals/item", map[string]any{"attributes": map[string]any{"links": []any{map[string]any{"url": "https://example.com"}}}})
	create := func(title string, tags []string) string {
		t.Helper()
		proposal := call("POST", "/api/proposals/item", map[string]any{"title": title, "tags": tags, "location": "Den", "attributes": map[string]any{"year": 2006, "links": []any{map[string]any{"name": "Wikipedia", "url": "https://en.wikipedia.org/wiki/Wii"}}}})
		before := call("GET", "/api/items?q="+url.QueryEscape(title), nil)
		if before["total"].(float64) != 0 {
			t.Fatal("proposal changed inventory before approval")
		}
		approved := call("POST", "/api/proposals/"+proposal["id"].(string)+"/decision", map[string]bool{"approve": true})
		return approved["proposed_payload"].(map[string]any)["item_id"].(string)
	}
	itemID := create("Wii console", []string{"Consoles", "Wii", " wii "})
	sportsID := create("Wii Sports", []string{"Games", "Wii"})
	all := call("GET", "/api/items?tag=Wii", nil)
	if all["total"].(float64) != 2 {
		t.Fatalf("overlapping tag browse: %v", all)
	}
	combined := call("GET", "/api/items?tag=Wii,Games", nil)
	if combined["total"].(float64) != 1 {
		t.Fatalf("tag intersection: %v", combined)
	}
	search := call("GET", "/api/items?q=2006&attribute_key=location&attribute_value=Den&limit=1", nil)
	if search["total"].(float64) != 2 || len(search["items"].([]any)) != 1 {
		t.Fatal("metadata search/pagination failed")
	}
	queried := call("POST", "/api/query_inventory", map[string]any{"query": "2006"})
	if queried["total_records"].(float64) != 2 {
		t.Fatal("read-only query incorrectly depends on template membership")
	}
	update := call("POST", "/api/proposals/item", map[string]any{"operation": "update", "item_id": itemID, "title": "Wii console", "tags": []string{}, "location": "", "attributes": map[string]any{"description": "Our old console"}})
	call("POST", "/api/proposals/"+update["id"].(string)+"/decision", map[string]bool{"approve": true})
	updated := call("GET", "/api/items/"+itemID, nil)
	if len(updated["tags"].([]any)) != 0 || updated["attributes"].(map[string]any)["location"] != nil {
		t.Fatal("explicit clearing of organization fields failed")
	}
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 600, 400))); err != nil {
		t.Fatal(err)
	}
	upload := func(data []byte) map[string]any {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("image", "cover.png")
		part.Write(data)
		writer.Close()
		req := httptest.NewRequest("POST", "/api/items/"+itemID+"/images", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
		}
		var result map[string]any
		json.Unmarshal(rec.Body.Bytes(), &result)
		return result
	}
	first := upload(original.Bytes())
	expectBad("/api/proposals/item", map[string]any{"operation": "update", "item_id": sportsID, "attributes": map[string]any{}, "primary_image_id": first["id"]})
	retry := upload(original.Bytes())
	if first["id"] != retry["id"] {
		t.Fatal("retry duplicated photo")
	}
	for _, variant := range []string{"original", "thumbnail"} {
		req := httptest.NewRequest("GET", "/api/images/"+first["id"].(string)+"/"+variant, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatal(rec.Code)
		}
		if variant == "original" && !bytes.Equal(rec.Body.Bytes(), original.Bytes()) {
			t.Fatal("original image was changed")
		}
		if variant == "thumbnail" {
			dimensions, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
			if err != nil || dimensions.Width != 320 || dimensions.Height != 320 {
				t.Fatal("thumbnail dimensions")
			}
		}
	}
	secondImage := image.NewRGBA(image.Rect(0, 0, 400, 600))
	var second bytes.Buffer
	png.Encode(&second, secondImage)
	secondPhoto := upload(second.Bytes())
	saved := call("GET", "/api/items/"+itemID, nil)
	if len(saved["images"].([]any)) != 2 {
		t.Fatal("multiple pictures were not attached")
	}

	if first["is_primary"] != true || secondPhoto["is_primary"] != false {
		t.Fatal("first uploaded image should be primary")
	}
	primaryProposal := call("POST", "/api/proposals/item", map[string]any{"operation": "update", "item_id": itemID, "attributes": updated["attributes"], "primary_image_id": secondPhoto["id"]})
	beforePrimary := call("GET", "/api/items/"+itemID, nil)
	if beforePrimary["images"].([]any)[0].(map[string]any)["id"] != first["id"] {
		t.Fatal("primary changed before approval")
	}
	call("POST", "/api/proposals/"+primaryProposal["id"].(string)+"/decision", map[string]bool{"approve": true})
	afterPrimary := call("GET", "/api/items/"+itemID, nil)
	if afterPrimary["images"].([]any)[0].(map[string]any)["id"] != secondPhoto["id"] {
		t.Fatal("approved primary not first in details")
	}
	page := call("GET", "/api/items", nil)
	foundPrimary := false
	for _, raw := range page["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == itemID {
			foundPrimary = item["images"].([]any)[0].(map[string]any)["id"] == secondPhoto["id"]
		}
	}
	if !foundPrimary {
		t.Fatal("browse cards did not hydrate the primary first")
	}
	rejected := call("POST", "/api/proposals/item", map[string]any{"title": "Rejected item", "tags": []string{"Games"}})
	call("POST", "/api/proposals/"+rejected["id"].(string)+"/decision", map[string]bool{"approve": false})
	absent := call("GET", "/api/items?q=Rejected", nil)
	if absent["total"].(float64) != 0 {
		t.Fatal("rejection saved an item")
	}

	// Templates suggest fields but must not limit custom details or own item lifetime.
	templateProposal := call("POST", "/api/proposals/category", map[string]any{"name": "Game suggestions", "default_tags": []string{"Games", "Wii"}, "default_values": map[string]any{"platform": "Wii"}, "attributes": []any{map[string]any{"key": "year", "label": "Year", "data_type": "number", "required": true}}})
	call("POST", "/api/proposals/"+templateProposal["id"].(string)+"/decision", map[string]bool{"approve": true})
	templates := call("GET", "/api/categories", nil)["categories"].([]any)
	templateID := templates[0].(map[string]any)["id"].(string)
	flexible := call("POST", "/api/proposals/item", map[string]any{"title": "Flexible game", "category_id": templateID, "attributes": map[string]any{"links": []any{map[string]any{"name": "Reference", "url": "https://example.com/game"}}, "description": "No year is known"}})
	approved := call("POST", "/api/proposals/"+flexible["id"].(string)+"/decision", map[string]bool{"approve": true})
	flexibleID := approved["proposed_payload"].(map[string]any)["item_id"].(string)
	independent := call("GET", "/api/items/"+flexibleID, nil)
	if independent["category_id"] != "" || independent["attributes"].(map[string]any)["platform"] != "Wii" || len(independent["tags"].([]any)) != 2 {
		t.Fatal("template defaults were not copied independently")
	}
	// Editing a template can explicitly clear all starting fields and defaults.
	editedTemplate := call("POST", "/api/proposals/category", map[string]any{"operation": "update", "category_id": templateID, "name": "Empty starting point", "attributes": []any{}, "default_tags": []string{}, "default_values": map[string]any{}})
	call("POST", "/api/proposals/"+editedTemplate["id"].(string)+"/decision", map[string]bool{"approve": true})
	emptyTemplate := call("GET", "/api/categories/"+templateID, nil)
	if len(emptyTemplate["attributes"].([]any)) != 0 || len(emptyTemplate["default_values"].(map[string]any)) != 0 || len(emptyTemplate["default_tags"].([]any)) != 0 {
		t.Fatal("template edit did not clear starting fields, defaults, and tags")
	}
	removal := call("POST", "/api/proposals/category", map[string]any{"operation": "delete", "category_id": templateID})
	call("POST", "/api/proposals/"+removal["id"].(string)+"/decision", map[string]bool{"approve": true})
	retained := call("GET", "/api/items/"+flexibleID, nil)
	if retained["category_id"] != "" || retained["attributes"].(map[string]any)["description"] != "No year is known" {
		t.Fatal("template removal lost flexible item data")
	}

	bare := call("POST", "/api/proposals/item", map[string]any{"tags": []string{"Consoles", "NES"}, "attributes": map[string]any{}})
	bareApproved := call("POST", "/api/proposals/"+bare["id"].(string)+"/decision", map[string]bool{"approve": true})
	bareID := bareApproved["proposed_payload"].(map[string]any)["item_id"].(string)
	bareItem := call("GET", "/api/items/"+bareID, nil)
	if len(bareItem["attributes"].(map[string]any)) != 0 || bareItem["quantity"] != nil || bareItem["location"] != nil {
		t.Fatal("optional fields leaked back into core metadata")
	}

	// Exercise Remy's actual ADK loop and typed presentations with a deterministic provider.
	var modelStep atomic.Int32
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		encoded, _ := json.Marshal(request["messages"])
		if !bytes.Contains(encoded, []byte("purchase_price")) || !bytes.Contains(encoded, []byte("tag_fields")) {
			t.Error("Remy did not receive approved graph context")
		}
		var message map[string]any
		switch modelStep.Add(1) {
		case 1:
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "search-1", "type": "function", "function": map[string]any{"name": "search_inventory", "arguments": `{"tags":"Wii","limit":10}`}}}}
		case 2:
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "present-1", "type": "function", "function": map[string]any{"name": "present_evidence", "arguments": `{"evidence_id":"evidence-1","component_type":"item_cards","title":"Wii items"}`}}}}
		case 3:
			message = map[string]any{"role": "assistant", "content": "Here are the matching Wii items."}
		case 4:
			args, _ := json.Marshal(map[string]any{"operation": "update", "item_id": sportsID, "attributes": map[string]any{"description": "A party favorite"}})
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "proposal-1", "type": "function", "function": map[string]any{"name": "propose_item_change", "arguments": string(args)}}}}
		default:
			message = map[string]any{"role": "assistant", "content": "Please review the proposed description."}
		}
		reason := "stop"
		if message["tool_calls"] != nil {
			reason = "tool_calls"
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": reason}}})
	}))
	defer modelServer.Close()
	aiConfig := config.Config{MainModel: "test-model", OpenAIBaseURL: modelServer.URL}
	aiHandler := New(Options{Config: aiConfig, Store: repo, Remy: remy.New(aiConfig, repo), Logger: slog.Default(), Images: objects})
	ask := func(message string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"message": message})
		request := httptest.NewRequest("POST", "/api/remy/request", bytes.NewReader(body))
		response := httptest.NewRecorder()
		aiHandler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("Remy request: %d %s", response.Code, response.Body.String())
		}
		var result map[string]any
		json.Unmarshal(response.Body.Bytes(), &result)
		return result
	}
	answer := ask("Show my Wii items as picture cards")
	components := answer["components"].([]any)
	if len(components) != 1 || components[0].(map[string]any)["type"] != "item_cards" {
		t.Fatalf("Remy did not choose typed picture cards: %v", answer)
	}
	answer = ask("Propose adding the description A party favorite to Wii Sports")
	components = answer["components"].([]any)
	if len(components) != 1 || components[0].(map[string]any)["type"] != "item_proposal" {
		t.Fatalf("Remy did not produce a reviewable item change: %v", answer)
	}
	unchanged := call("GET", "/api/items/"+sportsID, nil)
	if unchanged["attributes"].(map[string]any)["description"] != nil {
		t.Fatal("Remy committed its own proposed description")
	}
	facets := call("GET", "/api/library", nil)
	if facets["total"].(float64) != 4 {
		t.Fatal("wrong library total")
	}
}
