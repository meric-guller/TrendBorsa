package handler

import (
	"net/http"

	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type PurchaseHandler struct {
	store *store.MemoryStore
	hub   *hub.Hub
}

func NewPurchaseHandler(s *store.MemoryStore, h *hub.Hub) *PurchaseHandler {
	return &PurchaseHandler{store: s, hub: h}
}

func (h *PurchaseHandler) Buy(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	w.WriteHeader(http.StatusNotImplemented)
}
