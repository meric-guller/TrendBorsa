package store

import (
	"sync"

	"github.com/mericguller/trendborse-backend/internal/model"
)

type MemoryStore struct {
	mu           sync.RWMutex
	drops        map[string]*model.Drop
	purchases    map[string][]model.Purchase
	priceHistory map[string][]model.PricePoint
}

func New() *MemoryStore {
	return &MemoryStore{
		drops:        make(map[string]*model.Drop),
		purchases:    make(map[string][]model.Purchase),
		priceHistory: make(map[string][]model.PricePoint),
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
