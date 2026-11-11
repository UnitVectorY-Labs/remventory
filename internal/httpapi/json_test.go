package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONStrict(t *testing.T) {
	tests := []struct {
		name, body string
		valid      bool
	}{
		{"valid", `{"name":"item"}`, true},
		{"duplicate", `{"name":"one","name":"two"}`, false},
		{"invalid UTF-8", "{\"name\":\"\xff\"}", false},
		{"case mismatch", `{"Name":"item"}`, false},
		{"unknown", `{"name":"item","extra":1}`, false},
		{"trailing value", `{"name":"item"} {}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			var target struct {
				Name string `json:"name"`
			}
			if got := decodeJSON(rec, req, &target); got != tt.valid {
				t.Fatalf("decodeJSON = %v, want %v; response: %s", got, tt.valid, rec.Body.String())
			}
		})
	}
}
