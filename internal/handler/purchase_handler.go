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
const lockDuration = 30 * time.Second

type PurchaseHandler struct {
	store *store.MemoryStore
	hub   *hub.Hub
}

func NewPurchaseHandler(s *store.MemoryStore, h *hub.Hub) *PurchaseHandler {
	return &PurchaseHandler{store: s, hub: h}
}

type LockRequest struct {
	UserID   string `json:"user_id"`
	Quantity int    `json:"quantity"`
}

type LockResponse struct {
	Lock    model.PriceLock `json:"lock"`
	Message string          `json:"message"`
}

type ConfirmRequest struct {
	LockID string `json:"lock_id"`
}

type ConfirmResponse struct {
	Purchase model.Purchase `json:"purchase"`
	Message  string         `json:"message"`
}

type CancelRequest struct {
	LockID string `json:"lock_id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Lock reserves stock at the current price for 30 seconds.
func (h *PurchaseHandler) Lock(w http.ResponseWriter, r *http.Request) {
	dropID := chi.URLParam(r, "id")

	var req LockRequest
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

	// Atomic check-and-reserve under a single mutex acquisition
	drop, err := h.store.ReserveStock(dropID, req.UserID, req.Quantity, maxPerUser)
	if err != nil {
		status := http.StatusConflict
		if err.Error() == "drop not found" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, ErrorResponse{Error: err.Error()})
		return
	}

	now := time.Now()
	lock := model.PriceLock{
		ID:          uuid.New().String(),
		DropID:      dropID,
		UserID:      req.UserID,
		LockedPrice: drop.CurrentPrice,
		Quantity:    req.Quantity,
		Status:      "pending",
		CreatedAt:   now,
		ExpiresAt:   now.Add(lockDuration),
	}
	h.store.SaveLock(&lock)

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
	h.hub.Broadcast(dropID, model.PriceLockEvent{
		Type:     "PRICE_LOCK",
		DropID:   dropID,
		Quantity: req.Quantity,
	})

	writeJSON(w, http.StatusOK, LockResponse{
		Lock:    lock,
		Message: "Price locked for 30 seconds",
	})
}

// Confirm finalizes a pending lock into a purchase.
func (h *PurchaseHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	dropID := chi.URLParam(r, "id")

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.LockID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "lock_id is required"})
		return
	}

	lock, ok := h.store.GetLock(req.LockID)
	if !ok || lock.DropID != dropID {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "lock not found"})
		return
	}

	if lock.Status != "pending" {
		status := http.StatusConflict
		if lock.Status == "expired" {
			status = http.StatusGone
		}
		writeJSON(w, status, ErrorResponse{Error: "lock already " + lock.Status})
		return
	}

	// Check if lock expired (race with cleaner — not yet cleaned up)
	if time.Now().After(lock.ExpiresAt) {
		lock.Status = "expired"
		h.store.SaveLock(lock)
		h.store.RestoreStock(lock.DropID, lock.Quantity)
		broadcastStockUpdate(h, dropID)
		writeJSON(w, http.StatusGone, ErrorResponse{Error: "lock expired"})
		return
	}

	// Confirm the lock
	lock.Status = "confirmed"
	h.store.SaveLock(lock)

	purchase := model.Purchase{
		ID:        uuid.New().String(),
		DropID:    dropID,
		UserID:    lock.UserID,
		Price:     lock.LockedPrice,
		Quantity:  lock.Quantity,
		CreatedAt: time.Now(),
	}
	h.store.AddPurchase(purchase)

	// Broadcast purchase feed
	masked := maskUser(lock.UserID)
	h.hub.Broadcast(dropID, model.PurchaseFeedEvent{
		Type:    "PURCHASE_FEED",
		DropID:  dropID,
		Price:   lock.LockedPrice,
		Message: masked + " " + formatPrice(lock.LockedPrice) + " TL'den aldı!",
	})
	broadcastStockUpdate(h, dropID)

	writeJSON(w, http.StatusOK, ConfirmResponse{
		Purchase: purchase,
		Message:  "Purchase successful",
	})
}

// Cancel releases a pending lock and restores stock.
func (h *PurchaseHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	dropID := chi.URLParam(r, "id")

	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.LockID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "lock_id is required"})
		return
	}

	lock, ok := h.store.GetLock(req.LockID)
	if !ok || lock.DropID != dropID {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "lock not found"})
		return
	}

	if lock.Status != "pending" {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "lock already " + lock.Status})
		return
	}

	lock.Status = "cancelled"
	h.store.SaveLock(lock)
	h.store.RestoreStock(dropID, lock.Quantity)
	broadcastStockUpdate(h, dropID)

	writeJSON(w, http.StatusOK, map[string]string{"message": "Lock cancelled, stock restored"})
}

func broadcastStockUpdate(h *PurchaseHandler, dropID string) {
	drop, ok := h.store.GetDrop(dropID)
	if !ok {
		return
	}
	remainingPct := 0.0
	if drop.TotalStock > 0 {
		remainingPct = float64(drop.RemainingStock) / float64(drop.TotalStock) * 100
	}
	h.hub.Broadcast(dropID, model.StockUpdateEvent{
		Type:         "STOCK_UPDATE",
		DropID:       dropID,
		RemainingPct: remainingPct,
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
