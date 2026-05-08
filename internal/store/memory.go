package store

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mericguller/trendborse-backend/internal/model"
)

type MemoryStore struct {
	mu           sync.RWMutex
	drops        map[string]*model.Drop
	purchases    map[string][]model.Purchase
	priceHistory map[string][]model.PricePoint
	locks        map[string]*model.PriceLock
}

func New() *MemoryStore {
	return &MemoryStore{
		drops:        make(map[string]*model.Drop),
		purchases:    make(map[string][]model.Purchase),
		priceHistory: make(map[string][]model.PricePoint),
		locks:        make(map[string]*model.PriceLock),
	}
}

func (s *MemoryStore) GetAllDrops() []*model.Drop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model.Drop, 0, len(s.drops))
	for _, d := range s.drops {
		result = append(result, d)
	}
	return result
}

func (s *MemoryStore) GetDrop(id string) (*model.Drop, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.drops[id]
	return d, ok
}

func (s *MemoryStore) SaveDrop(d *model.Drop) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drops[d.ID] = d
}

func (s *MemoryStore) AddPurchase(p model.Purchase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purchases[p.DropID] = append(s.purchases[p.DropID], p)
}

func (s *MemoryStore) GetPurchases(dropID string) []model.Purchase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.purchases[dropID]
}

func (s *MemoryStore) AddPricePoint(dropID string, pp model.PricePoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.priceHistory[dropID] = append(s.priceHistory[dropID], pp)
}

func (s *MemoryStore) GetPriceHistory(dropID string) []model.PricePoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.priceHistory[dropID]
}

func (s *MemoryStore) GetRecentPurchaseCount(dropID string, since time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, p := range s.purchases[dropID] {
		if p.CreatedAt.After(since) {
			count += p.Quantity
		}
	}
	return count
}

func (s *MemoryStore) GetUserPurchaseCount(dropID, userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, p := range s.purchases[dropID] {
		if p.UserID == userID {
			count += p.Quantity
		}
	}
	return count
}

func (s *MemoryStore) SaveLock(l *model.PriceLock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locks[l.ID] = l
}

func (s *MemoryStore) GetLock(id string) (*model.PriceLock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.locks[id]
	return l, ok
}

func (s *MemoryStore) GetExpiredLocks(now time.Time) []*model.PriceLock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.PriceLock
	for _, l := range s.locks {
		if l.Status == "pending" && now.After(l.ExpiresAt) {
			result = append(result, l)
		}
	}
	return result
}

// ReserveStock atomically checks user limit + stock and decrements under a single lock.
func (s *MemoryStore) ReserveStock(dropID, userID string, quantity, maxPerUser int) (*model.Drop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	drop, ok := s.drops[dropID]
	if !ok {
		return nil, errors.New("drop not found")
	}
	if drop.Status != "active" {
		return nil, errors.New("drop is not active")
	}

	// Count confirmed purchases + pending locks for this user
	userUsage := 0
	for _, p := range s.purchases[dropID] {
		if p.UserID == userID {
			userUsage += p.Quantity
		}
	}
	for _, l := range s.locks {
		if l.DropID == dropID && l.UserID == userID && l.Status == "pending" {
			userUsage += l.Quantity
		}
	}
	if userUsage+quantity > maxPerUser {
		return nil, errors.New("purchase limit exceeded")
	}
	if drop.RemainingStock < quantity {
		return nil, errors.New("not enough stock")
	}

	drop.RemainingStock -= quantity
	return drop, nil
}

// RestoreStock adds quantity back to the drop's remaining stock.
func (s *MemoryStore) RestoreStock(dropID string, quantity int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if drop, ok := s.drops[dropID]; ok {
		drop.RemainingStock += quantity
	}
}

func (s *MemoryStore) Seed() {
	now := time.Now()

	drops := []model.Drop{
		{
			ID:             "drop-1",
			ProductName:    "Sony WH-1000XM5 Kulaklık",
			ProductImage:   "https://cdn.trendyol.com/sony-wh1000xm5.jpg",
			Description:    "Aktif Gürültü Önleme özellikli premium kablosuz kulaklık",
			StartPrice:     8999,
			CurrentPrice:   8999,
			FloorPrice:     8499,
			CeilingPrice:   9499,
			TotalStock:     50,
			RemainingStock: 50,
			StartsAt:       now.Add(-1 * time.Minute),
			EndsAt:         now.Add(19 * time.Minute),
			Status:         "active",
			Volatility:     0.002,
			CreatedAt:      now,
		},
		{
			ID:             "drop-2",
			ProductName:    "iPhone 16 Pro Max Kılıf",
			ProductImage:   "https://cdn.trendyol.com/iphone16-case.jpg",
			Description:    "Premium silikon koruma kılıfı - MagSafe uyumlu",
			StartPrice:     599,
			CurrentPrice:   599,
			FloorPrice:     499,
			CeilingPrice:   699,
			TotalStock:     200,
			RemainingStock: 200,
			StartsAt:       now.Add(5 * time.Minute),
			EndsAt:         now.Add(25 * time.Minute),
			Status:         "upcoming",
			Volatility:     0.008,
			CreatedAt:      now,
		},
		{
			ID:             "drop-3",
			ProductName:    "Nike Air Max 90",
			ProductImage:   "https://cdn.trendyol.com/nike-airmax90.jpg",
			Description:    "Klasik tasarım, modern konfor - Unisex spor ayakkabı",
			StartPrice:     4299,
			CurrentPrice:   4299,
			FloorPrice:     3799,
			CeilingPrice:   4799,
			TotalStock:     30,
			RemainingStock: 30,
			StartsAt:       now.Add(-2 * time.Minute),
			EndsAt:         now.Add(18 * time.Minute),
			Status:         "active",
			Volatility:     0.003,
			CreatedAt:      now,
		},
		{
			ID:             "drop-4",
			ProductName:    "Dyson V15 Süpürge",
			ProductImage:   "https://cdn.trendyol.com/dyson-v15.jpg",
			Description:    "Lazer toz algılama teknolojili kablosuz süpürge",
			StartPrice:     24999,
			CurrentPrice:   24999,
			FloorPrice:     24499,
			CeilingPrice:   25499,
			TotalStock:     15,
			RemainingStock: 15,
			StartsAt:       now.Add(10 * time.Minute),
			EndsAt:         now.Add(30 * time.Minute),
			Status:         "upcoming",
			Volatility:     0.001,
			CreatedAt:      now,
		},
	}

	for i := range drops {
		s.SaveDrop(&drops[i])
		s.AddPricePoint(drops[i].ID, model.PricePoint{
			Price:     drops[i].StartPrice,
			Timestamp: now,
		})
	}

	fmt.Printf("Seeded %d drops\n", len(drops))
}
