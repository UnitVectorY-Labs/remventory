package httpapi

import (
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"net/http"
)

func (a api) vocabulary(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}
	result, err := a.store.Vocabulary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load field vocabulary")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (a api) proposeGraph(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}
	var payload store.GraphProposalPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	proposal, err := a.store.CreateGraphProposal(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, proposal)
}
