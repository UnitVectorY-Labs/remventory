package photoidentify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	result, err := Identify(context.Background(), cfg, []byte("picture"), []store.Category{})
	if err != nil {
		t.Fatal(err)
	}
	if result.CategoryID != "" || result.Attributes["year"] != float64(2006) || result.Notes == "" {
		t.Fatalf("suggestions lost flexible metadata or accepted invented template: %+v", result)
	}
}
func TestPhotoFailureIsRecoverable(t *testing.T) {
	for _, body := range []string{`{"choices":[]}`, `{"choices":[{"message":{"content":"not JSON"}}]}`, `garbage`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, err := Identify(context.Background(), config.Config{VisionBaseURL: server.URL, VisionModel: "vision"}, nil, nil)
		server.Close()
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
