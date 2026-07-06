package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

type Options struct {
	Config  config.Config
	Store   *store.Store
	Version string
	Logger  *slog.Logger
}

func New(opts Options) http.Handler {
	mux := http.NewServeMux()
	api := api{
		cfg:     opts.Config,
		store:   opts.Store,
		version: opts.Version,
		logger:  opts.Logger,
	}

	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	mux.HandleFunc("GET /api/config", api.withToken(api.configStatus))
	mux.HandleFunc("GET /api/categories", api.withToken(api.listCategories))
	mux.HandleFunc("GET /api/categories/{id}", api.withToken(api.getCategory))
	mux.HandleFunc("GET /api/items", api.withToken(api.listItems))
	mux.HandleFunc("POST /api/proposals/category", api.withToken(api.createCategoryProposal))
	mux.HandleFunc("POST /api/proposals/item", api.withToken(api.createItemProposal))
	mux.HandleFunc("GET /api/proposals/{id}", api.withToken(api.getProposal))
	mux.HandleFunc("POST /api/proposals/{id}/decision", api.withToken(api.decideProposal))

	return mux
}

type api struct {
	cfg     config.Config
	store   *store.Store
	version string
	logger  *slog.Logger
}

func (a api) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": a.version,
	})
}

func (a api) ready(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	body := map[string]any{
		"status":  "ready",
		"version": a.version,
		"config":  a.cfg.PublicStatus(),
	}

	if a.store == nil {
		status = http.StatusServiceUnavailable
		body["status"] = "not_ready"
		body["database"] = "not_configured"
		writeJSON(w, status, body)
		return
	}

	if err := a.store.Ping(r.Context()); err != nil {
		status = http.StatusServiceUnavailable
		body["status"] = "not_ready"
		body["database"] = err.Error()
		writeJSON(w, status, body)
		return
	}

	body["database"] = "ok"
	writeJSON(w, status, body)
}

func (a api) configStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": a.version,
		"config":  a.cfg.PublicStatus(),
	})
}

func (a api) listCategories(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	categories, err := a.store.ListCategories(r.Context(), limit, offset)
	if err != nil {
		a.logger.Error("list categories", "error", err)
		writeError(w, http.StatusInternalServerError, "list categories")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"limit":      limit,
		"offset":     offset,
	})
}

func (a api) getCategory(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	category, err := a.store.GetCategoryDefinition(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		a.logger.Error("get category", "error", err)
		writeError(w, http.StatusInternalServerError, "get category")
		return
	}

	writeJSON(w, http.StatusOK, category)
}

func (a api) listItems(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	categoryID := r.URL.Query().Get("category_id")
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "category_id is required")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	items, err := a.store.ListItems(r.Context(), categoryID, limit, offset)
	if err != nil {
		a.logger.Error("list items", "error", err)
		writeError(w, http.StatusInternalServerError, "list items")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"category_id": categoryID,
		"limit":       limit,
		"offset":      offset,
	})
}

func (a api) createCategoryProposal(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	var payload store.CategoryProposalPayload
	if !decodeJSON(w, r, &payload) {
		return
	}

	proposal, err := a.store.CreateCategoryProposal(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, proposal)
}

func (a api) createItemProposal(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	var payload store.ItemProposalPayload
	if !decodeJSON(w, r, &payload) {
		return
	}

	proposal, err := a.store.CreateItemProposal(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, proposal)
}

func (a api) getProposal(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	proposal, err := a.store.GetProposal(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if err != nil {
		a.logger.Error("get proposal", "error", err)
		writeError(w, http.StatusInternalServerError, "get proposal")
		return
	}

	writeJSON(w, http.StatusOK, proposal)
}

func (a api) decideProposal(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}

	var decision store.ProposalDecision
	if !decodeJSON(w, r, &decision) {
		return
	}

	proposal, err := a.store.DecideProposal(r.Context(), r.PathValue("id"), decision)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, proposal)
}

func (a api) withToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.AccessToken == "" {
			next(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+a.cfg.AccessToken {
			writeError(w, http.StatusUnauthorized, "missing or invalid access token")
			return
		}
		next(w, r)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return false
	}
	return true
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	if parsed > 200 {
		return 200
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}
