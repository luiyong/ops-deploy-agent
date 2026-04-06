package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"ops/internal/server/store"
)

type JarHandler struct {
	store *store.JarStore
}

func NewJarHandler(store *store.JarStore) *JarHandler {
	return &JarHandler{store: store}
}

func (h *JarHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "parse multipart form", err)
		return
	}

	serviceName := strings.TrimSpace(r.FormValue("service_name"))
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read upload file", err)
		return
	}
	defer file.Close()

	meta, err := h.store.Upload(serviceName, header.Filename, file)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "upload jar", err)
		return
	}
	writeJSON(w, http.StatusCreated, meta)
}

func (h *JarHandler) List(w http.ResponseWriter, _ *http.Request) {
	metas, err := h.store.List()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list jars", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": metas})
}

func (h *JarHandler) Download(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	serviceName := strings.TrimSpace(r.URL.Query().Get("service_name"))

	var (
		path string
		err  error
	)
	if serviceName != "" {
		path, err = h.store.ResolveFilePath(serviceName, filename)
	} else {
		path, err = h.store.GetFilePath(filename)
	}
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "resolve jar", err)
		return
	}
	http.ServeFile(w, r, path)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string, err error) {
	if err == nil {
		err = errors.New(message)
	}
	writeJSON(w, status, map[string]any{
		"error":   message,
		"details": err.Error(),
	})
}
