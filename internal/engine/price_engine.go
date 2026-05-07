package engine

import (
	"context"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/model"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type PriceEngine struct {
	store    *store.MemoryStore
	hub      *hub.Hub
	interval time.Duration
}

func New(s *store.MemoryStore, h *hub.Hub, interval time.Duration) *PriceEngine {
	return &PriceEngine{store: s, hub: h, interval: interval}
}

func (e *PriceEngine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	log.Printf("Price engine started (tick every %s)", e.interval)

	for {
		select {
		case <-ticker.C:
			e.tick()
		case <-ctx.Done():
			log.Println("Price engine stopped")
			return
		}
	}
}

func (e *PriceEngine) tick() {
	drops := e.store.GetAllDrops()
	now := time.Now()

	for _, drop := range drops {
		// Lifecycle: upcoming -> active
		if drop.Status == "upcoming" && now.After(drop.StartsAt) {
			drop.Status = "active"
			e.store.SaveDrop(drop)
			e.hub.Broadcast(drop.ID, model.DropStatusEvent{
				Type:   "DROP_STATUS",
				DropID: drop.ID,
				Status: "active",
			})
			log.Printf("Drop %s is now ACTIVE", drop.ID)
		}

		// Lifecycle: active -> ended
		if drop.Status == "active" && (now.After(drop.EndsAt) || drop.RemainingStock <= 0) {
			drop.Status = "ended"
			e.store.SaveDrop(drop)

			totalSold := drop.TotalStock - drop.RemainingStock
			e.hub.Broadcast(drop.ID, model.DropEndedEvent{
				Type:       "DROP_ENDED",
				DropID:     drop.ID,
				FinalPrice: drop.CurrentPrice,
				TotalSold:  totalSold,
			})
			log.Printf("Drop %s ENDED (final: %.2f, sold: %d)", drop.ID, drop.CurrentPrice, totalSold)
			continue
		}

		// Price recalculation only for active drops
		if drop.Status != "active" {
			continue
		}

		e.recalculate(drop, now)
	}
}

func (e *PriceEngine) recalculate(drop *model.Drop, now time.Time) {
	prevPrice := drop.CurrentPrice

	since := now.Add(-e.interval)
	purchases := e.store.GetRecentPurchaseCount(drop.ID, since)

	var newPrice float64
	if purchases > 0 {
		increase := float64(purchases) * drop.Volatility * drop.CurrentPrice
		newPrice = drop.CurrentPrice + increase
	} else {
		decay := drop.Volatility * 0.5 * drop.CurrentPrice
		newPrice = drop.CurrentPrice - decay
	}

	// Add noise for realism (±0.5%)
	noise := (rand.Float64()*0.01 - 0.005) * drop.CurrentPrice
	newPrice += noise

	// Clamp to floor/ceiling
	newPrice = math.Max(drop.FloorPrice, math.Min(newPrice, drop.CeilingPrice))

	// Round to 2 decimals
	newPrice = math.Round(newPrice*100) / 100

	drop.CurrentPrice = newPrice
	e.store.SaveDrop(drop)

	e.store.AddPricePoint(drop.ID, model.PricePoint{
		Price:     newPrice,
		Timestamp: now,
	})

	direction := "STABLE"
	if newPrice > prevPrice {
		direction = "UP"
	} else if newPrice < prevPrice {
		direction = "DOWN"
	}

	e.hub.Broadcast(drop.ID, model.PriceUpdateEvent{
		Type:      "PRICE_UPDATE",
		DropID:    drop.ID,
		Price:     newPrice,
		PrevPrice: prevPrice,
		Direction: direction,
	})

	remainingPct := 0.0
	if drop.TotalStock > 0 {
		remainingPct = math.Round(float64(drop.RemainingStock)/float64(drop.TotalStock)*10000) / 100
	}
	e.hub.Broadcast(drop.ID, model.StockUpdateEvent{
		Type:         "STOCK_UPDATE",
		DropID:       drop.ID,
		RemainingPct: remainingPct,
	})

	viewerCount := e.hub.ViewerCount(drop.ID)
	e.hub.Broadcast(drop.ID, model.ViewerCountEvent{
		Type:   "VIEWER_COUNT",
		DropID: drop.ID,
		Count:  viewerCount,
	})
}
