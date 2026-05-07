package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/mericguller/trendborse-backend/internal/config"
	"github.com/mericguller/trendborse-backend/internal/engine"
	"github.com/mericguller/trendborse-backend/internal/handler"
	"github.com/mericguller/trendborse-backend/internal/hub"
	"github.com/mericguller/trendborse-backend/internal/store"
)

func main() {
	cfg := config.Load()

	s := store.New()
	s.Seed()
	h := hub.New()
	eng := engine.New(s, h, cfg.TickInterval)
	go eng.Run(context.Background())

	dropH := handler.NewDropHandler(s, h)
	purchaseH := handler.NewPurchaseHandler(s, h)
	wsH := handler.NewWSHandler(h)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/drops", func(r chi.Router) {
		r.Get("/", dropH.List)
		r.Post("/", dropH.Create)
		r.Get("/{id}", dropH.Get)
		r.Get("/{id}/history", dropH.History)
		r.Get("/{id}/stats", dropH.Stats)
		r.Post("/{id}/buy", purchaseH.Buy)
	})

	r.Get("/ws/drops/{id}", wsH.Handle)

	log.Printf("TrendBorsa server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
