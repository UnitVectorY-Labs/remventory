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
