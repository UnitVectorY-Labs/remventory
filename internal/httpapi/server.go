package httpapi

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/itemimages"
	"github.com/UnitVectorY-Labs/remventory/internal/remy"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

//go:embed static/*
var staticFiles embed.FS

type Options struct {
	Config     config.Config
	Store      *store.Store
	Remy       *remy.Service
	MCPHandler http.Handler
	Version    string
	Logger     *slog.Logger
	Images     itemimages.ObjectStore
}

func New(opts Options) http.Handler {
	mux := http.NewServeMux()
	api := api{
		cfg:     opts.Config,
		store:   opts.Store,
		remy:    opts.Remy,
		version: opts.Version,
		logger:  opts.Logger,
		images:  opts.Images,
	}

	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	mux.HandleFunc("GET /{$}", api.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS())))
	if opts.MCPHandler != nil {
		mux.Handle("/mcp", opts.MCPHandler)
		mux.Handle("/mcp/", opts.MCPHandler)
	}
	mux.HandleFunc("GET /api/config", api.withToken(api.configStatus))
	mux.HandleFunc("POST /api/remy/request", api.withToken(api.remyRequest))
	mux.HandleFunc("POST /api/remy/dialog", api.withToken(api.remyDialog))
	mux.HandleFunc("POST /api/query_inventory", api.withToken(api.queryInventory))
	mux.HandleFunc("GET /api/categories", api.withToken(api.listCategories))
	mux.HandleFunc("GET /api/categories/{id}", api.withToken(api.getCategory))
	mux.HandleFunc("GET /api/items", api.withToken(api.listItems))
	mux.HandleFunc("GET /api/items/{id}", api.withToken(api.getItem))
	mux.HandleFunc("POST /api/items/{id}/images", api.withToken(api.uploadItemImage))
	mux.HandleFunc("GET /api/images/{id}/{variant}", api.serveItemImage)
	mux.HandleFunc("POST /api/proposals/category", api.withToken(api.createCategoryProposal))
	mux.HandleFunc("POST /api/proposals/item", api.withToken(api.createItemProposal))
	mux.HandleFunc("GET /api/proposals/{id}", api.withToken(api.getProposal))
	mux.HandleFunc("POST /api/proposals/{id}/decision", api.withToken(api.decideProposal))

	return mux
}

func (a api) remyDialog(w http.ResponseWriter, r *http.Request) {
	if a.remy == nil {
		writeError(w, http.StatusServiceUnavailable, "remy is not configured")
		return
	}

	var payload remy.DialogRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	response, err := a.remy.Dialog(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type api struct {
	cfg     config.Config
	store   *store.Store
	remy    *remy.Service
	version string
	logger  *slog.Logger
	images  itemimages.ObjectStore
}

func staticFS() fs.FS {
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return fsys
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
	if a.cfg.MainModel == "" || a.cfg.ThinkingModel == "" {
		status = http.StatusServiceUnavailable
		body["status"] = "not_ready"
		body["model"] = "main_or_thinking_not_configured"
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

func (a api) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, staticFS(), "index.html")
}

func (a api) remyRequest(w http.ResponseWriter, r *http.Request) {
	if a.remy == nil {
		writeError(w, http.StatusServiceUnavailable, "remy is not configured")
		return
	}

	var payload remy.Request
	if !decodeJSON(w, r, &payload) {
		return
	}

	response, err := a.remy.Handle(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a api) queryInventory(w http.ResponseWriter, r *http.Request) {
	if a.remy == nil {
		writeError(w, http.StatusServiceUnavailable, "remy is not configured")
		return
	}

	var payload struct {
		Query      string `json:"query"`
		CategoryID string `json:"category_id"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}

	result, err := a.remy.QueryInventory(r.Context(), payload.Query, payload.CategoryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
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
	for i := range items {
		setImageURLs(items[i].Images)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"category_id": categoryID,
		"limit":       limit,
		"offset":      offset,
	})
}

func (a api) getItem(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database is not configured")
		return
	}
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		a.logger.Error("get item", "error", err)
		writeError(w, http.StatusInternalServerError, "get item")
		return
	}
	setImageURLs(item.Images)
	writeJSON(w, http.StatusOK, item)
}

func (a api) uploadItemImage(w http.ResponseWriter, r *http.Request) {
	if a.store == nil || a.images == nil {
		writeError(w, http.StatusServiceUnavailable, "image storage is not configured")
		return
	}
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		a.logger.Error("get item for image upload", "error", err)
		writeError(w, http.StatusInternalServerError, "get item")
		return
	}
	if len(item.Images) > 0 {
		writeError(w, http.StatusConflict, "this item already has a picture")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, itemimages.MaxUploadBytes+(1<<20))
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "select an image to upload")
		return
	}
	defer file.Close()
	prepared, err := itemimages.Prepare(file, header)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, itemimages.ErrTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, err.Error())
		return
	}
	if err := itemimages.StorePrepared(r.Context(), a.images, prepared); err != nil {
		a.logger.Error("store item image objects", "error", err)
		writeError(w, http.StatusBadGateway, "store image")
		return
	}
	created, err := a.store.CreateItemImage(r.Context(), store.ItemImage{
		ID: prepared.ID, ItemID: item.ID, OriginalKey: prepared.OriginalKey, ThumbnailKey: prepared.ThumbnailKey,
		MIMEType: prepared.MIMEType, OriginalFilename: prepared.OriginalFilename,
		Width: prepared.Width, Height: prepared.Height, SizeBytes: int64(len(prepared.Original)),
	})
	if err != nil {
		_ = a.images.Delete(r.Context(), prepared.OriginalKey)
		_ = a.images.Delete(r.Context(), prepared.ThumbnailKey)
		a.logger.Error("save item image", "error", err)
		writeError(w, http.StatusInternalServerError, "save image")
		return
	}
	created.OriginalURL = "/api/images/" + created.ID + "/original"
	created.ThumbnailURL = "/api/images/" + created.ID + "/thumbnail"
	writeJSON(w, http.StatusCreated, created)
}

func (a api) serveItemImage(w http.ResponseWriter, r *http.Request) {
	if a.store == nil || a.images == nil {
		http.NotFound(w, r)
		return
	}
	imageRecord, err := a.store.GetItemImage(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	key := imageRecord.OriginalKey
	if r.PathValue("variant") == "thumbnail" {
		key = imageRecord.ThumbnailKey
	} else if r.PathValue("variant") != "original" {
		http.NotFound(w, r)
		return
	}
	object, err := a.images.Get(r.Context(), key)
	if err != nil {
		a.logger.Error("read item image", "error", err)
		writeError(w, http.StatusBadGateway, "read image")
		return
	}
	defer object.Body.Close()
	w.Header().Set("Content-Type", object.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.PathValue("variant") == "original" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": imageRecord.OriginalFilename}))
	}
	if object.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	}
	_, _ = io.Copy(w, object.Body)
}

func setImageURLs(images []store.ItemImage) {
	for i := range images {
		images[i].OriginalURL = "/api/images/" + images[i].ID + "/original"
		images[i].ThumbnailURL = "/api/images/" + images[i].ID + "/thumbnail"
	}
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
