package handler

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/model"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type DropHandler struct {
	store *store.MemoryStore
	hub   *hub.Hub
}

func NewDropHandler(s *store.MemoryStore, h *hub.Hub) *DropHandler {
	return &DropHandler{store: s, hub: h}
}

func (h *DropHandler) List(w http.ResponseWriter, r *http.Request) {
	drops := h.store.GetAllDrops()

	// Filter by status if query param provided
	status := r.URL.Query().Get("status")
	if status != "" {
		filtered := make([]*model.Drop, 0)
		for _, d := range drops {
			if d.Status == status {
				filtered = append(filtered, d)
			}
		}
		drops = filtered
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drops)
}

func (h *DropHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	drop, ok := h.store.GetDrop(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "drop not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drop)
}

func (h *DropHandler) History(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	history := h.store.GetPriceHistory(id)
	if history == nil {
		history = []model.PricePoint{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

type CreateDropRequest struct {
	ProductName  string  `json:"product_name"`
	ProductImage string  `json:"product_image"`
	Description  string  `json:"description"`
	StartPrice   float64 `json:"start_price"`
	TotalStock   int     `json:"total_stock"`
	StartsAt     string  `json:"starts_at"`
	EndsAt       string  `json:"ends_at"`
	Volatility   float64 `json:"volatility"`
}

func (h *DropHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDropRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.ProductName == "" || req.StartPrice <= 0 || req.TotalStock <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "product_name, start_price, and total_stock are required"})
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		startsAt = time.Now().Add(5 * time.Minute)
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		endsAt = startsAt.Add(20 * time.Minute)
	}

	volatility := req.Volatility
	if volatility <= 0 {
		volatility = 0.03
	}

	status := "upcoming"
	if time.Now().After(startsAt) {
		status = "active"
	}

	drop := &model.Drop{
		ID:             uuid.New().String(),
		ProductName:    req.ProductName,
		ProductImage:   req.ProductImage,
		Description:    req.Description,
		StartPrice:     req.StartPrice,
		CurrentPrice:   req.StartPrice,
		FloorPrice:     math.Round(req.StartPrice * 0.6),
		CeilingPrice:   math.Round(req.StartPrice * 1.4),
		TotalStock:     req.TotalStock,
		RemainingStock: req.TotalStock,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Status:         status,
		Volatility:     volatility,
		CreatedAt:      time.Now(),
	}

	h.store.SaveDrop(drop)
	h.store.AddPricePoint(drop.ID, model.PricePoint{
		Price:     drop.StartPrice,
		Timestamp: time.Now(),
	})

	writeJSON(w, http.StatusCreated, drop)
}

type StatsResponse struct {
	ViewerCount       int     `json:"viewer_count"`
	TotalPurchases    int     `json:"total_purchases"`
	RemainingStockPct float64 `json:"remaining_stock_pct"`
	PriceChangePct    float64 `json:"price_change_pct"`
	TimeRemainingSec  int     `json:"time_remaining_seconds"`
}

func (h *DropHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	drop, ok := h.store.GetDrop(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "drop not found"})
		return
	}

	purchases := h.store.GetPurchases(id)
	totalQty := 0
	for _, p := range purchases {
		totalQty += p.Quantity
	}

	remainingPct := 0.0
	if drop.TotalStock > 0 {
		remainingPct = math.Round(float64(drop.RemainingStock)/float64(drop.TotalStock)*10000) / 100
	}

	priceChangePct := 0.0
	if drop.StartPrice > 0 {
		priceChangePct = math.Round((drop.CurrentPrice-drop.StartPrice)/drop.StartPrice*10000) / 100
	}

	timeRemaining := 0
	if drop.Status == "active" {
		remaining := time.Until(drop.EndsAt)
		if remaining > 0 {
			timeRemaining = int(remaining.Seconds())
		}
	}

	writeJSON(w, http.StatusOK, StatsResponse{
		ViewerCount:       h.hub.ViewerCount(id),
		TotalPurchases:    totalQty,
		RemainingStockPct: remainingPct,
		PriceChangePct:    priceChangePct,
		TimeRemainingSec:  timeRemaining,
	})
}
