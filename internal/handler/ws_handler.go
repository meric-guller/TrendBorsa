package handler

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/mericguller/trendborse-backend/internal/hub"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	hub *hub.Hub
}

func NewWSHandler(h *hub.Hub) *WSHandler {
	return &WSHandler{hub: h}
}

func (h *WSHandler) Handle(w http.ResponseWriter, r *http.Request) {
	dropID := chi.URLParam(r, "id")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	h.hub.Register(dropID, conn)
	defer func() {
		h.hub.Unregister(dropID, conn)
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
