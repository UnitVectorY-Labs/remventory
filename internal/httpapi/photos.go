package httpapi

import (
	"net/http"
	"strings"

	"github.com/UnitVectorY-Labs/remventory/internal/itemimages"
	"github.com/UnitVectorY-Labs/remventory/internal/photoidentify"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

func (a api) identifyPhoto(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}
	if a.cfg.VisionModel == "" {
		writeError(w, http.StatusServiceUnavailable, "Photo identification needs setup. You can enter details yourself.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, itemimages.MaxUploadBytes+(1<<20))
	file, header, err := r.FormFile("image")
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "select a JPEG, PNG, or GIF picture up to 12 MB")
		return
	}
	defer file.Close()
	prepared, err := itemimages.Prepare(file, header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	picture, err := itemimages.AnalysisImage(prepared.Original)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	categories, err := a.store.ListCategories(r.Context(), 200, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load item types")
		return
	}
	vocabulary, err := a.store.Vocabulary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load field vocabulary")
		return
	}
	var selected *store.Category
	if id := strings.TrimSpace(r.FormValue("template_id")); id != "" {
		for _, template := range vocabulary.Templates {
			if template.ID == id {
				selected = &template
				break
			}
		}
		if selected == nil {
			writeError(w, http.StatusBadRequest, "The selected template is no longer available. Choose another template and try again.")
			return
		}
	}
	suggestion, err := photoidentify.Identify(r.Context(), a.cfg, picture, categories, selected, vocabulary)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

func (a api) libraryFacets(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}
	result, err := a.store.LibraryFacets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load library filters")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
