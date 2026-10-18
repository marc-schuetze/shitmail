package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/marc-schuetze/shitmail/internal/api/middleware"
	"github.com/marc-schuetze/shitmail/internal/storage"
)

type TokenHandler struct{ Store *storage.TokenStore }

func (h TokenHandler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, 401, "SSO required")
		return
	}
	tokens, err := h.Store.List(r.Context(), user.UID)
	if err != nil {
		writeError(w, 500, "database error")
		return
	}
	writeJSON(w, 200, map[string]any{"tokens": tokens})
}

func (h TokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, 401, "SSO required")
		return
	}
	var input struct {
		Name          string `json:"name"`
		ExpiresInDays int    `json:"expiresInDays"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 80 {
		writeError(w, 400, "name must be 1–80 characters")
		return
	}
	if input.ExpiresInDays == 0 {
		input.ExpiresInDays = 30
	}
	if input.ExpiresInDays < 1 || input.ExpiresInDays > 365 {
		writeError(w, 400, "expiry must be 1–365 days")
		return
	}
	token, secret, err := h.Store.Create(r.Context(), user.UID, input.Name, time.Duration(input.ExpiresInDays)*24*time.Hour)
	if err != nil {
		writeError(w, 500, "could not create token")
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "secret": secret})
}

func (h TokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFrom(r.Context())
	if !ok {
		writeError(w, 401, "SSO required")
		return
	}
	if err := h.Store.Revoke(r.Context(), user.UID, chi.URLParam(r, "id")); err != nil {
		writeError(w, 500, "could not revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
