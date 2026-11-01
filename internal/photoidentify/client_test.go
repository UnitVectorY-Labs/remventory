package photoidentify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

func TestPhotoSuggestionsAreStructuredAndProviderIndependent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong endpoint or credentials")
		}
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["model"] != "my-vision-model" {
			t.Error("vision model not selected")
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "```json\n{\"title\":\"Likely Wii\",\"category_id\":\"invented-id\",\"tags\":[\"Consoles\",\"Wii\"],\"attributes\":{\"year\":2006,\"reference\":\"https://en.wikipedia.org/wiki/Wii\"},\"notes\":\"Year inferred; verify model label.\"}\n```"}}}})
	}))
	defer server.Close()
	cfg := config.Config{VisionBaseURL: server.URL + "/v1", VisionModel: "my-vision-model", VisionAPIKey: "test-key"}
	result, err := Identify(context.Background(), cfg, []byte("picture"), []store.Category{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CategoryID != "" || result.Attributes["year"] != float64(2006) || result.Notes == "" {
		t.Fatalf("suggestions lost flexible metadata or accepted invented template: %+v", result)
	}
}

func TestPhotoIdentificationIncludesSelectedTemplate(t *testing.T) {
	template := store.Category{ID: "selected-template", Name: "Nintendo Switch games", Description: "Read the game cover and identify the edition", DefaultTags: []string{"Switch"}, DefaultValues: json.RawMessage(`{"platform":"Nintendo Switch"}`), Attributes: []store.Attribute{{Key: "title", Label: "Title", DataType: "text"}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		var prompt string
		if err := json.Unmarshal(payload.Messages[0].Content, &prompt); err != nil {
			t.Error(err)
			return
		}
		for _, value := range []string{"Selected template:", "Nintendo Switch games", "Read the game cover", `"platform":"Nintendo Switch"`, `"key":"title"`, "Do not assign ongoing template membership"} {
			if !strings.Contains(strings.ToLower(prompt), strings.ToLower(value)) {
				t.Errorf("selected template context missing %q", value)
			}
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"title\":\"Zelda\",\"attributes\":{\"title\":\"Zelda\",\"invented\":true},\"notes\":\"Check the edition.\"}"}}]}`))
	}))
	defer server.Close()
	vocabulary := store.GraphVocabulary{Fields: []store.FieldDefinition{{Key: "title", Label: "Title", DataType: "text"}}}
	result, err := Identify(context.Background(), config.Config{VisionModel: "vision", VisionBaseURL: server.URL}, []byte("photo"), nil, &template, vocabulary)
	if err != nil {
		t.Fatal(err)
	}
	if result.Attributes["title"] != "Zelda" || result.Attributes["invented"] != nil {
		t.Fatalf("template bypassed approved fields: %+v", result)
	}
}
func TestPhotoFailureIsRecoverable(t *testing.T) {
	for _, body := range []string{`{"choices":[]}`, `{"choices":[{"message":{"content":"not JSON"}}]}`, `garbage`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, err := Identify(context.Background(), config.Config{VisionBaseURL: server.URL, VisionModel: "vision"}, nil, nil, nil)
		server.Close()
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
