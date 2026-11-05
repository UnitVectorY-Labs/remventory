package httpapi

import (
	"encoding/json"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"net/http"
	"strings"
)

func (a api) proposeCatalogue(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	var input struct {
		Operation string        `json:"operation"`
		URL       string        `json:"url"`
		Name      string        `json:"name"`
		Ref       authority.Ref `json:"ref"`
		EntityID  string        `json:"entity_id"`
		Relation  string        `json:"relation"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "item not found")
		return
	}
	var card *authority.DisplayCard
	input.URL = strings.TrimSpace(input.URL)
	if input.Operation == "reference" || input.Operation == "reference_remove" {
		if err = authority.ValidateURL(input.URL); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if input.Operation == "reference" && strings.TrimSpace(input.Name) == "" {
			writeError(w, 400, "give the reference link a name")
			return
		}
		var attrs map[string]json.RawMessage
		json.Unmarshal(item.Attributes, &attrs)
		links := []map[string]string{}
		if len(attrs["links"]) > 0 {
			if e := json.Unmarshal(attrs["links"], &links); e != nil {
				writeError(w, 400, "existing references need manual review in Edit my fields")
				return
			}
		}
		if input.Operation == "reference_remove" {
			filtered := []map[string]string{}
			found := false
			for _, l := range links {
				if l["url"] == input.URL && (input.Name == "" || l["name"] == input.Name) {
					found = true
					continue
				}
				filtered = append(filtered, l)
			}
			if !found {
				writeError(w, 409, "reference no longer exists")
				return
			}
			links = filtered
		} else {
			for _, l := range links {
				if l["url"] == input.URL {
					writeError(w, 409, "this reference is already saved")
					return
				}
			}
			links = append(links, map[string]string{"name": input.Name, "url": input.URL})
		}
		fields, _ := json.Marshal(map[string]any{"links": links})
		p, e := a.store.CreateItemProposal(r.Context(), store.ItemProposalPayload{Operation: "update", ItemID: item.ID, Attributes: fields})
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"proposal": p, "reference": map[string]string{"name": input.Name, "url": input.URL, "operation": input.Operation}})
		return
	}
	if input.Operation == "link" {
		if input.URL != "" {
			ref, known, e := authority.ParseURL(input.URL)
			if e != nil {
				writeError(w, 400, e.Error())
				return
			}
			if !known {
				writeError(w, 400, "this URL can be saved as a reference; choose Reference link instead of Catalogue record")
				return
			}
			input.Ref = ref
		}
		g, e := a.intake.Resolve(r.Context(), input.Ref, false)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		profile, _ := a.store.ProfileForItem(r.Context(), item)
		c, e := a.intake.CardForPolicy(r.Context(), g, &profile.IntakeConfig)
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		card = &c
	}
	p, err := a.store.ProposeCatalogue(r.Context(), store.CatalogueProposal{Operation: input.Operation, ItemID: item.ID, EntityID: input.EntityID, Relation: input.Relation, Ref: input.Ref})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"proposal": p, "card": card})
}

func (a api) curateItemFields(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "item not found")
		return
	}
	suggestions, err := a.intake.CurateFields(r.Context(), item)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"suggestions": suggestions, "attributes": item.Attributes})
}
