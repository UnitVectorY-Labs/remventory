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
	policy := j.Input.Policy
	if policy == nil {
		profile, e := a.store.ProfileForIdentification(r.Context(), j)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		policy = &profile.IntakeConfig
	}
	cards := []authority.DisplayCard{}
	for _, m := range j.Result.Matches {
		c, e := a.intake.CardForPolicy(r.Context(), m.Graph, policy)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		if c.Title == "" {
			c.Title = m.Candidate.Label
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
	item, _ := a.store.GetItem(r.Context(), r.PathValue("id"))
	profile, _ := a.store.ProfileForItem(r.Context(), item)
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
		c, e := a.intake.CardForPolicy(r.Context(), d.Graph, &profile.IntakeConfig)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		if c.Title == "" {
			c.Title = item.Title
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

// Field choices come from cached verified metadata, not an invented universal media form.
func (a api) catalogueFieldChoices(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "item not found")
		return
	}
	profile, err := a.store.ProfileForItem(r.Context(), item)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	links, err := a.store.ItemCanonical(r.Context(), item.ID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	choices := map[string]map[string]string{}
	for _, link := range links {
		d, e := a.store.EntityDetail(r.Context(), link["id"].(string))
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		provider := d.Graph.Root.Provider
		if choices[provider] == nil {
			choices[provider] = map[string]string{}
		}
		flattenCatalogueFields("", authority.DisplayFields(d.Graph), choices[provider])
	}
	writeJSON(w, 200, map[string]any{"profile": profile, "choices": choices})
}
func flattenCatalogueFields(prefix string, values map[string]any, out map[string]string) {
	for key, value := range values {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			flattenCatalogueFields(path, nested, out)
			continue
		}
		if text := authority.FactText(value); text != "" {
			out[path] = text
		}
	}
}
