package engine

import (
	"context"
	"log"
	"time"

	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/model"
	"github.com/mericguller/trendborse-backend/internal/store"
)

type LockCleaner struct {
	store    *store.MemoryStore
	hub      *hub.Hub
	interval time.Duration
}

func NewLockCleaner(s *store.MemoryStore, h *hub.Hub) *LockCleaner {
	return &LockCleaner{store: s, hub: h, interval: 5 * time.Second}
}

func (lc *LockCleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(lc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lc.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

func (lc *LockCleaner) cleanup() {
	expired := lc.store.GetExpiredLocks(time.Now())
	for _, lock := range expired {
		lock.Status = "expired"
		lc.store.SaveLock(lock)
		lc.store.RestoreStock(lock.DropID, lock.Quantity)

		drop, ok := lc.store.GetDrop(lock.DropID)
		if ok && drop.TotalStock > 0 {
			pct := float64(drop.RemainingStock) / float64(drop.TotalStock) * 100
			lc.hub.Broadcast(lock.DropID, model.StockUpdateEvent{
				Type:         "STOCK_UPDATE",
				DropID:       lock.DropID,
				RemainingPct: pct,
			})
			lc.hub.Broadcast(lock.DropID, model.LockExpiredEvent{
				Type:     "LOCK_EXPIRED",
				DropID:   lock.DropID,
				Quantity: lock.Quantity,
			})
		}

		log.Printf("Lock %s expired for drop %s, restored %d units", lock.ID, lock.DropID, lock.Quantity)
	}
}
