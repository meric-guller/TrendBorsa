package hub

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*websocket.Conn]bool
}

func New() *Hub {
	return &Hub{
		rooms: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *Hub) Register(dropID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[dropID] == nil {
		h.rooms[dropID] = make(map[*websocket.Conn]bool)
	}
	h.rooms[dropID][conn] = true
}

func (h *Hub) Unregister(dropID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.rooms[dropID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.rooms, dropID)
		}
	}
}

func (h *Hub) Broadcast(dropID string, event any) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("hub: marshal error: %v", err)
		return
	}

	h.mu.RLock()
	conns := h.rooms[dropID]
	h.mu.RUnlock()

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("hub: write error: %v", err)
			conn.Close()
			h.Unregister(dropID, conn)
		}
	}
}

func (h *Hub) ViewerCount(dropID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[dropID])
}
