package httpapi

import (
	"fmt"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/itemimages"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"io"
	"net/http"
	"strings"
)

func (a api) intakeReady(w http.ResponseWriter) bool {
	if a.store == nil || a.intake == nil {
		writeError(w, 503, "identification is not configured")
		return false
	}
	return true
}
func (a api) listIdentifications(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	jobs, err := a.store.Identifications(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	for i := range jobs {
		for m := range jobs[i].Result.Matches {
			jobs[i].Result.Matches[m].Graph = authority.Graph{Root: jobs[i].Result.Matches[m].Graph.Root}
		}
		jobs[i].Result.Searches = nil
	}
	writeJSON(w, 200, map[string]any{"jobs": jobs, "worker_enabled": a.cfg.WorkerEnabled})
}
func (a api) createIdentification(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	if a.images == nil {
		writeError(w, 503, "photo storage is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*itemimages.MaxUploadBytes+(2<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, 400, "upload up to eight photos, 12 MB each")
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["images"]
	if len(files) == 0 || len(files) > 8 {
		writeError(w, 400, "select one to eight photos for one object")
		return
	}
	if len(r.FormValue("text")) > 32000 {
		writeError(w, 400, "identification clues must be under 32 KB")
		return
	}
	category := strings.TrimSpace(r.FormValue("category"))
	if len(category) > 300 {
		writeError(w, 400, "category is too long")
		return
	}
	prepared := []itemimages.Prepared{}
	for _, h := range files {
		file, err := h.Open()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		p, err := itemimages.Prepare(file, h)
		file.Close()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		p.OriginalKey = "intake/" + p.ID + "/original"
		p.ThumbnailKey = "intake/" + p.ID + "/thumbnail.jpg"
		prepared = append(prepared, p)
	}
	stored := []itemimages.Prepared{}
	success := false
	defer func() {
		if !success {
			for _, p := range stored {
				_ = a.images.Delete(r.Context(), p.OriginalKey)
				_ = a.images.Delete(r.Context(), p.ThumbnailKey)
			}
		}
	}()
	photos := []store.IdentificationPhoto{}
	for _, p := range prepared {
		if err := itemimages.StorePrepared(r.Context(), a.images, p); err != nil {
			writeError(w, 502, "could not store photographs")
			return
		}
		stored = append(stored, p)
		photos = append(photos, store.IdentificationPhoto{ID: p.ID, Key: p.OriginalKey, ThumbnailKey: p.ThumbnailKey, MIME: p.MIMEType, Filename: p.OriginalFilename, Width: p.Width, Height: p.Height, Size: int64(len(p.Original))})
	}
	job, err := a.store.CreateIdentification(r.Context(), category, "", store.IdentificationInput{Text: r.FormValue("text"), TemplateID: r.FormValue("template_id")}, photos)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	success = true
	writeJSON(w, 202, job)
}
func (a api) getIdentification(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	job, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "identification not found")
		return
	}
	if r.URL.Query().Get("compact") == "1" {
		for i := range job.Result.Matches {
			job.Result.Matches[i].Graph = authority.Graph{Root: job.Result.Matches[i].Graph.Root}
		}
		job.Result.Searches = nil
	}
	writeJSON(w, 200, job)
}
func (a api) identificationAction(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.store.SetIdentificationState(r.Context(), r.PathValue("id"), input.Action); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a api) proposeIdentification(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	var input store.IdentificationProposal
	if !decodeJSON(w, r, &input) {
		return
	}
	input.JobID = r.PathValue("id")
	p, err := a.store.ProposeIdentification(r.Context(), input)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, p)
}
func (a api) enrichCollection(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	var input struct {
		Tag    string `json:"tag"`
		Limit  int    `json:"limit"`
		ItemID string `json:"item_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.ItemID != "" {
		item, err := a.store.GetItem(r.Context(), input.ItemID)
		if err != nil {
			writeError(w, 404, "item not found")
			return
		}
		if active, e := a.store.ActiveIdentification(r.Context(), item.ID); e == nil {
			writeJSON(w, 202, map[string]any{"jobs": []store.IdentificationJob{active}})
			return
		}
		job, err := a.store.CreateIdentification(r.Context(), strings.Join(item.Tags, ", "), item.ID, store.IdentificationInput{Text: item.Title + "\nOriginal fields: " + string(item.Attributes)}, nil)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, 202, map[string]any{"jobs": []store.IdentificationJob{job}})
		return
	}
	jobs, err := a.store.QueueEnrichment(r.Context(), input.Tag, input.Limit)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, map[string]any{"jobs": jobs})
}
func (a api) identificationImage(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) || a.images == nil {
		return
	}
	job, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "identification not found")
		return
	}
	for _, p := range job.Photos {
		if p.ID != r.PathValue("photo") {
			continue
		}
		key := p.ThumbnailKey
		if r.PathValue("variant") == "original" {
			key = p.Key
		}
		obj, err := a.images.Get(r.Context(), key)
		if err != nil {
			writeError(w, 404, "photo not found")
			return
		}
		defer obj.Body.Close()
		w.Header().Set("Content-Type", obj.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = io.Copy(w, obj.Body)
		return
	}
	writeError(w, 404, "photo not found")
}
func (a api) authoritySearch(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	p, err := a.intake.Providers.Provider(r.URL.Query().Get("provider"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := p.Search(r.Context(), authority.Query{Text: r.URL.Query().Get("q"), Title: r.URL.Query().Get("title"), Artist: r.URL.Query().Get("artist"), Barcode: r.URL.Query().Get("barcode"), Catalog: r.URL.Query().Get("catalog"), Format: r.URL.Query().Get("format")})
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": out})
}
func (a api) resolveForIdentification(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	var ref authority.Ref
	if !decodeJSON(w, r, &ref) {
		return
	}
	job, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil || job.State != "review" {
		writeError(w, 409, "job must be in review")
		return
	}
	if len(job.Result.Matches) >= 12 {
		writeError(w, 400, "candidate limit reached; retry identification")
		return
	}
	g, err := a.intake.Resolve(r.Context(), ref, false)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	label := g.Root.ID
	uri := ""
	for _, e := range g.Entities {
		if e.Ref == g.Root {
			label = e.Label
			uri = e.URI
		}
	}
	role := "alternative"
	if g.Association == "contains" {
		role = "component"
	}
	job.Result.Outcome = "partial"
	job.Result.Matches = append(job.Result.Matches, store.IdentificationMatch{Role: role, Candidate: authority.Candidate{Ref: g.Root, Label: label, URI: uri}, Graph: g, Confidence: "partial", Reason: "Manually selected for review; source validated identifier."})
	if err = a.store.ReplaceIdentificationResult(r.Context(), job.ID, job.Result); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 200, job)
}
func (a api) graphSearch(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	q := r.URL.Query()
	items, err := a.store.GraphSearch(r.Context(), q.Get("q"), q.Get("kind"), q.Get("year"), q.Get("entity_id"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	for i := range items {
		setImageURLsForItem(&items[i])
	}
	writeJSON(w, 200, map[string]any{"items": items, "limit": 100})
}
func setImageURLsForItem(item *store.Item) { setImageURLs(item.Images) }
func (a api) canonicalDetail(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	d, err := a.store.EntityDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, d)
}
func (a api) itemCanonical(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	links, err := a.store.ItemCanonical(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"links": links})
}
func (a api) relatedOwnership(w http.ResponseWriter, r *http.Request) {
	if !a.intakeReady(w) {
		return
	}
	job, err := a.store.Identification(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	out := []map[string]any{}
	seen := map[string]bool{}
	for index, m := range job.Result.Matches {
		for _, e := range m.Graph.Entities {
			isWork := e.Kind == "release-group" || e.Ref == m.Graph.Root
			if !isWork {
				continue
			}
			entityID, lookupErr := a.store.CanonicalID(r.Context(), e.Ref)
			if lookupErr != nil {
				continue
			}
			items, err := a.store.GraphSearch(r.Context(), "", "", "", entityID)
			if err != nil {
				writeError(w, 500, err.Error())
				return
			}
			for _, item := range items {
				if item.ID == job.ItemID {
					continue
				}
				key := fmt.Sprint(index) + ":" + item.ID
				if !seen[key] {
					seen[key] = true
					setImageURLsForItem(&item)
					out = append(out, map[string]any{"match_index": index, "item": item, "via": e.Label, "basis": "cached canonical graph"})
				}
			}
		}
		// Original-field matches are useful before the existing collection is enriched,
		// but are deliberately not presented as verified canonical duplicates.
		items, _, _ := a.store.SearchInventory(r.Context(), m.Candidate.Label, "", "", "", 10, 0)
		for _, item := range items {
			if item.ID == job.ItemID {
				continue
			}
			key := fmt.Sprint(index) + ":" + item.ID
			if !seen[key] {
				seen[key] = true
				setImageURLsForItem(&item)
				out = append(out, map[string]any{"match_index": index, "item": item, "via": m.Candidate.Label, "basis": "original text only; unverified"})
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ownership": out})
}
