package httpapi

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
)

// Only the status region polls; photos, navigation and user input remain mounted.
func (a api) identificationStatus(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	j, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "Identification not found", http.StatusNotFound)
		return
	}
	renderIdentificationStatus(w, j.ID, j.State)
}

func renderIdentificationStatus(w http.ResponseWriter, id, state string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if state != "queued" && state != "processing" {
		event, _ := json.Marshal(map[string]any{"identificationReady": map[string]string{"id": id}})
		w.Header().Set("HX-Trigger", string(event))
		fmt.Fprint(w, `<div class="processing-status" role="status">Ready. Preparing your review…</div>`)
		return
	}
	message := "Your item is queued. Remy will look at it shortly."
	if state == "processing" {
		message = "Remy is checking your photos and comparing catalogue records."
	}
	fmt.Fprintf(w, `<div class="processing-status" role="status" hx-get="/api/intake/%s/status" hx-trigger="load delay:3s" hx-swap="outerHTML" hx-sync="this:replace" hx-request='{"timeout":15000}'><span class="photo-analysis-spinner" aria-hidden="true"></span><div><strong>%s</strong><p>You can photograph another item. This work continues on the server.</p><small class="htmx-indicator">Checking progress…</small></div></div>`, html.EscapeString(id), message)
}
