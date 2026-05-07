package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/model"
	"github.com/mericguller/trendborse-backend/internal/store"
)

const maxPerUser = 3

type PurchaseHandler struct {
	store *store.MemoryStore
	hub   *hub.Hub
}

func NewPurchaseHandler(s *store.MemoryStore, h *hub.Hub) *PurchaseHandler {
	return &PurchaseHandler{store: s, hub: h}
}

type BuyRequest struct {
	UserID   string `json:"user_id"`
	Quantity int    `json:"quantity"`
}

type BuyResponse struct {
	Purchase model.Purchase `json:"purchase"`
	Message  string         `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *PurchaseHandler) Buy(w http.ResponseWriter, r *http.Request) {
	dropID := chi.URLParam(r, "id")

	var req BuyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "user_id is required"})
		return
	}
	if req.Quantity < 1 || req.Quantity > maxPerUser {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "quantity must be between 1 and 3"})
		return
	}

	drop, ok := h.store.GetDrop(dropID)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "drop not found"})
		return
	}

	if drop.Status != "active" {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "drop is not active"})
		return
	}

	userTotal := h.store.GetUserPurchaseCount(dropID, req.UserID)
	if userTotal+req.Quantity > maxPerUser {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "purchase limit exceeded (max 3 per user)"})
		return
	}

	if drop.RemainingStock < req.Quantity {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "not enough stock"})
		return
	}

	// Atomic: lock price + decrement stock
	lockedPrice := drop.CurrentPrice
	drop.RemainingStock -= req.Quantity
	h.store.SaveDrop(drop)

	purchase := model.Purchase{
		ID:        uuid.New().String(),
		DropID:    dropID,
		UserID:    req.UserID,
		Price:     lockedPrice,
		Quantity:  req.Quantity,
		CreatedAt: time.Now(),
	}
	h.store.AddPurchase(purchase)

	// Broadcast purchase feed
	masked := maskUser(req.UserID)
	h.hub.Broadcast(dropID, model.PurchaseFeedEvent{
		Type:    "PURCHASE_FEED",
		DropID:  dropID,
		Price:   lockedPrice,
		Message: masked + " " + formatPrice(lockedPrice) + " TL'den aldı!",
	})

	// Broadcast stock update
	remainingPct := 0.0
	if drop.TotalStock > 0 {
		remainingPct = float64(drop.RemainingStock) / float64(drop.TotalStock) * 100
	}
	h.hub.Broadcast(dropID, model.StockUpdateEvent{
		Type:         "STOCK_UPDATE",
		DropID:       dropID,
		RemainingPct: remainingPct,
	})

	writeJSON(w, http.StatusOK, BuyResponse{
		Purchase: purchase,
		Message:  "Purchase successful",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func maskUser(userID string) string {
	if len(userID) <= 2 {
		return userID + "***"
	}
	return string(userID[0]) + "***" + string(userID[len(userID)-1])
}

func formatPrice(p float64) string {
	return fmt.Sprintf("%.2f", p)
}
