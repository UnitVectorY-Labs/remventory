package httpapi

import (
	"encoding/json/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdentificationStatusFragments(t *testing.T) {
	for _, state := range []string{"queued", "processing", "review", "failed", "approved", "rejected"} {
		t.Run(state, func(t *testing.T) {
			response := httptest.NewRecorder()
			renderIdentificationStatus(response, "a-unique-job", state)
			body := response.Body.String()
			pending := state == "queued" || state == "processing"
			if strings.Contains(body, `hx-trigger="load delay:3s"`) != pending {
				t.Fatal("poll must continue only while server works", body)
			}
			if strings.Contains(body, "<script") {
				t.Fatal("unexpected executable response")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("stale progress could be cached")
			}
			if !pending {
				var event map[string]map[string]string
				if err := json.Unmarshal([]byte(response.Header().Get("HX-Trigger")), &event); err != nil || event["identificationReady"]["id"] != "a-unique-job" {
					t.Fatal("terminal state did not announce completion", err)
				}
			}
		})
	}
	response := httptest.NewRecorder()
	renderIdentificationStatus(response, `bad" onclick="alert(1)`, "queued")
	if strings.Contains(response.Body.String(), `onclick="`) {
		t.Fatal("unescaped job identifier")
	}
}
