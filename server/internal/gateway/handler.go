package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/utachicodes/wrkflw/server/internal/auth"
	"github.com/utachicodes/wrkflw/server/internal/httpapi"
)

type configStore interface {
	Get(context.Context, string) (Config, error)
	Upsert(context.Context, string, Config) (Config, error)
	MarkPulled(context.Context, string) (Config, error)
}

// Handler serves the gateway configuration endpoints. All routes sit behind
// account-scoped guards: browser sessions and personal API tokens pass,
// agent credentials do not.
type Handler struct {
	store configStore
}

// NewHandler returns a Handler backed by store.
func NewHandler(store configStore) *Handler {
	return &Handler{store: store}
}

// GetConfig returns the account's gateway config for the settings UI. Reads
// do not stamp last_pulled_at; only daemon pulls do.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request, user auth.User) {
	config, err := h.store.Get(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, err, "gateway config could not be loaded")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

// UpdateConfig validates and stores the account's gateway config.
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request, user auth.User) {
	var input Config
	if !decodeJSON(w, r, &input) {
		return
	}
	stored, err := h.store.Upsert(r.Context(), user.ID, input)
	if errors.Is(err, ErrInvalidConfig) {
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid gateway configuration\n"))
		return
	}
	if err != nil {
		writeInternalError(w, err, "gateway config could not be saved")
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

// PullConfig returns the account's gateway config for the daemon and stamps
// last_pulled_at so the settings UI can show real connection status.
func (h *Handler) PullConfig(w http.ResponseWriter, r *http.Request, user auth.User) {
	config, err := h.store.MarkPulled(r.Context(), user.ID)
	if err != nil {
		writeInternalError(w, err, "gateway config could not be loaded")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target *Config) bool {
	return httpapi.DecodeJSON(w, r, target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeInternalError(w http.ResponseWriter, err error, message string) {
	if httpapi.WriteServiceUnavailable(w, err) {
		return
	}
	writeError(w, http.StatusInternalServerError, message)
}
