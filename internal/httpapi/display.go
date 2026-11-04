package httpapi

import (
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"net/http"
)

func (a api) identificationCards(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	j, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "identification not found")
		return
	}
	cards := []authority.DisplayCard{}
	for _, m := range j.Result.Matches {
		c, e := a.intake.Card(r.Context(), m.Graph)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		cards = append(cards, c)
	}
	writeJSON(w, 200, map[string]any{"cards": cards})
}
func (a api) itemCards(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	links, err := a.store.ItemCanonical(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "item not found")
		return
	}
	cards := []authority.DisplayCard{}
	seen := map[string]bool{}
	for _, link := range links {
		id, _ := link["id"].(string)
		if seen[id] {
			continue
		}
		seen[id] = true
		d, e := a.store.EntityDetail(r.Context(), id)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		c, e := a.intake.Card(r.Context(), d.Graph)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		cards = append(cards, c)
		if len(cards) >= 8 {
			break
		}
	}
	writeJSON(w, 200, map[string]any{"cards": cards})
}
func (a api) canonicalCard(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	d, err := a.store.EntityDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "record not found")
		return
	}
	c, err := a.intake.Card(r.Context(), d.Graph)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"card": c, "owners": d.Owners})
}
