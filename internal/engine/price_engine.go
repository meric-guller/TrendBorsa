package engine

import (
	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type PriceEngine struct {
	store *store.MemoryStore
	hub   *hub.Hub
}

func New(s *store.MemoryStore, h *hub.Hub) *PriceEngine {
	return &PriceEngine{store: s, hub: h}
}
