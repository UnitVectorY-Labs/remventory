package httpapi

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/intake"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

func TestWorkspaceDirectLinks(t *testing.T) {
	handler := New(Options{Logger: slog.Default()})
	for _, path := range []string{"/", "/pending?page=2", "/review", "/admin", "/capture", "/items/example", "/identify/example", "/canonical/example", "/?identify=legacy"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 200 || !strings.Contains(r.Body.String(), `/static/navigation.js`) {
			t.Errorf("direct link %s: %d", path, r.Code)
		}
	}
	for _, path := range []string{"/missing", "/api/missing", "/items/example/missing"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 404 {
			t.Errorf("unknown route %s: %d", path, r.Code)
		}
	}
}

func TestPendingIdentificationPaginationValidation(t *testing.T) {
	// Invalid pagination must be rejected before making any database queries.
	a := api{store: &store.Store{}, intake: &intake.Service{}}
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "limit=abc", "offset=-1", "offset=abc", "offset=999999999999999999999"} {
		r := httptest.NewRecorder()
		a.pendingIdentifications(r, httptest.NewRequest("GET", "/api/intake/pending?"+query, nil))
		if r.Code != 400 {
			t.Errorf("invalid pagination %s: %d", query, r.Code)
		}
	}
}
