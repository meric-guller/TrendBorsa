package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type DropHandler struct {
	store *store.MemoryStore
}

func NewDropHandler(s *store.MemoryStore) *DropHandler {
	return &DropHandler{store: s}
}

func (h *DropHandler) List(w http.ResponseWriter, r *http.Request) {
	drops := h.store.GetAllDrops()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drops)
}

func (h *DropHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	drop, ok := h.store.GetDrop(id)
	if !ok {
		http.Error(w, "drop not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drop)
}

func (h *DropHandler) History(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	history := h.store.GetPriceHistory(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}
