# TrendBorsa Backend — Architecture

> Go backend for TrendBorsa: a real-time, demand-driven pricing engine for e-commerce.
> Products go on sale at scheduled times. Prices move **up** when people buy and **down** when demand drops — like a stock market for products.

---

## 1. Overview

```
┌─────────────────────────────────────────────────────────┐
│                    CLIENTS                              │
│   Android (KMP)  ·  iOS (KMP)  ·  Home Widget           │
└──────────┬──────────────────────────┬───────────────────┘
           │ REST (HTTP)              │ WebSocket
┌──────────▼──────────────────────────▼───────────────────┐
│                   GO BACKEND                            │
│                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   Router     │  │  WS Hub     │  │  Price Engine   │ │
│  │  (chi)       │  │  (gorilla)  │  │  (goroutine)    │ │
│  │             │  │             │  │                 │ │
│  │  GET /drops  │  │  broadcast  │  │  tick every 10s │ │
│  │  POST /buy   │  │  per-drop   │  │  calc new price │ │
│  │  GET /stats  │  │  rooms      │  │  clamp limits   │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│  ┌──────▼────────────────▼───────────────────▼────────┐ │
│  │                    STORE (in-memory)               │ │
│  │  sync.RWMutex-protected maps                       │ │
│  │  Drops · Purchases · PriceHistory                  │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| HTTP router | `chi` | Lightweight, stdlib-compatible, middleware support |
| WebSocket | `gorilla/websocket` | Battle-tested, widely used |
| Storage | In-memory (`sync.RWMutex` + maps) | Hackathon speed — no DB setup needed |
| Price engine | Single goroutine + ticker | Simple, predictable, easy to debug |
| JSON | `encoding/json` (stdlib) | No external dependency needed |
| Config | Environment variables | 12-factor, easy to change per environment |

---

## 2. Project Structure

```
trendborse-backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires everything, starts server
│
├── internal/
│   ├── model/
│   │   └── model.go             # Domain types: Drop, Purchase, PricePoint, events
│   │
│   ├── store/
│   │   └── memory.go            # Thread-safe in-memory store
│   │
│   ├── engine/
│   │   └── price_engine.go      # Price calculation goroutine
│   │
│   ├── hub/
│   │   └── hub.go               # WebSocket connection manager & broadcaster
│   │
│   ├── handler/
│   │   ├── drop_handler.go      # REST endpoints for drops
│   │   ├── purchase_handler.go  # POST /buy endpoint
│   │   └── ws_handler.go        # WebSocket upgrade & connection
│   │
│   └── config/
│       └── config.go            # Environment-based configuration
│
├── go.mod
├── go.sum
├── Makefile
└── ARCHITECTURE.md              # This file
```

---

## 3. Domain Model

### Drop

A **Drop** is a time-limited product sale with dynamic pricing.

```go
type Drop struct {
    ID             string
    ProductName    string
    ProductImage   string
    Description    string
    StartPrice     float64   // initial price when drop starts
    CurrentPrice   float64   // real-time price (moves with demand)
    FloorPrice     float64   // minimum allowed price
    CeilingPrice   float64   // maximum allowed price
    TotalStock     int
    RemainingStock int
    StartsAt       time.Time
    EndsAt         time.Time // optional hard deadline
    Status         string    // "upcoming" | "active" | "ended"
    Volatility     float64   // price sensitivity (0.01–0.10)
    CreatedAt      time.Time
}
```

### Purchase

```go
type Purchase struct {
    ID        string
    DropID    string
    UserID    string
    Price     float64   // price at time of purchase
    Quantity  int
    CreatedAt time.Time
}
```

### PricePoint (for chart history)

```go
type PricePoint struct {
    Price     float64
    Timestamp time.Time
}
```

### WebSocket Events (Server → Client)

```go
// Price changed
{ "type": "PRICE_UPDATE", "drop_id": "abc", "price": 1050.00, "prev_price": 1000.00, "direction": "UP" }

// Stock level changed
{ "type": "STOCK_UPDATE", "drop_id": "abc", "remaining_pct": 73.5 }

// Someone bought (social proof feed)
{ "type": "PURCHASE_FEED", "drop_id": "abc", "price": 1050.00, "message": "Biri 1050 TL'den aldı!" }

// Live viewer count
{ "type": "VIEWER_COUNT", "drop_id": "abc", "count": 847 }

// Drop status changed
{ "type": "DROP_STATUS", "drop_id": "abc", "status": "active" }

// Drop ended
{ "type": "DROP_ENDED", "drop_id": "abc", "final_price": 920.00, "total_sold": 153 }
```

---

## 4. REST API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/drops` | List all drops (filter by `?status=active,upcoming,ended`) |
| `GET` | `/api/drops/{id}` | Drop detail with current price |
| `GET` | `/api/drops/{id}/history` | Price history for chart (last N minutes) |
| `GET` | `/api/drops/{id}/stats` | Live stats (viewers, purchases, velocity) |
| `POST` | `/api/drops/{id}/buy` | Buy at current price `{ "user_id": "...", "quantity": 1 }` |
| `POST` | `/api/drops` | Create a new drop (admin) |
| `GET` | `/api/health` | Health check |

### WebSocket

| Path | Description |
|------|-------------|
| `GET` `/ws/drops/{id}` | Upgrade to WebSocket. Joins drop room, receives real-time events. |

---

## 5. Price Engine Algorithm

The engine runs as a **single goroutine** with a `time.Ticker`. Every tick (default: 10 seconds), it recalculates the price for each active drop.

```
FOR each active drop:

  purchases_in_window = count purchases in last tick interval
  
  IF purchases_in_window > 0:
      // Demand exists → price goes UP
      increase = purchases_in_window × volatility × current_price
      new_price = current_price + increase
  ELSE:
      // No demand → price decays DOWN
      decay = volatility × 0.5 × current_price
      new_price = current_price - decay

  // Clamp to floor/ceiling
  new_price = clamp(new_price, floor_price, ceiling_price)

  // Add small noise for realism (±0.5%)
  noise = random(-0.005, +0.005) × current_price
  new_price = new_price + noise

  // Broadcast via WebSocket hub
  hub.Broadcast(drop_id, PriceUpdateEvent{...})
```

### Configurable Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `TICK_INTERVAL` | `10s` | How often price recalculates |
| `VOLATILITY` | `0.03` | Per-drop price sensitivity |
| `FLOOR_PCT` | `0.60` | Floor = StartPrice × 0.60 |
| `CEILING_PCT` | `1.40` | Ceiling = StartPrice × 1.40 |
| `NOISE_RANGE` | `0.005` | Random noise amplitude |

---

## 6. WebSocket Hub

The hub manages per-drop **rooms**. Each room has a set of connected clients.

```
Hub
├── rooms: map[dropID] → set of *websocket.Conn
├── Register(dropID, conn)
├── Unregister(dropID, conn)
├── Broadcast(dropID, event)        // send to all clients in room
├── BroadcastAll(event)             // send to all connected clients
└── ViewerCount(dropID) → int
```

- Connections are cleaned up on disconnect (ping/pong heartbeat).
- Each connection runs two goroutines: one for reading (client messages), one for writing (server events).
- The hub itself is a single goroutine processing register/unregister/broadcast channels.

---

## 7. Store (In-Memory)

Thread-safe with `sync.RWMutex`. All data lives in Go maps.

```go
type MemoryStore struct {
    mu            sync.RWMutex
    drops         map[string]*Drop
    purchases     map[string][]Purchase     // dropID → purchases
    priceHistory  map[string][]PricePoint   // dropID → price points
}
```

For hackathon: seeded with 3–5 demo drops on startup.

---

## 8. Configuration

All config via environment variables with sensible defaults:

```bash
PORT=8080                  # Server port
TICK_INTERVAL=10s          # Price engine tick
DEFAULT_VOLATILITY=0.03    # Default volatility for new drops
CORS_ORIGIN=*              # CORS allowed origins
```

---

## 9. Running

```bash
# Development
make run

# Build binary
make build

# Run binary
./bin/trendborse-server

# With custom config
PORT=9090 TICK_INTERVAL=5s make run
```

---

## 10. Demo Seed Data

On startup, the server creates demo drops:

| Product | Start Price | Floor | Ceiling | Stock | Starts |
|---------|------------|-------|---------|-------|--------|
| Sony WH-1000XM5 Kulaklik | 8.999 TL | 5.399 TL | 12.599 TL | 50 | +2 min |
| iPhone 16 Pro Max Kilif | 599 TL | 359 TL | 839 TL | 200 | +5 min |
| Nike Air Max 90 | 4.299 TL | 2.579 TL | 6.019 TL | 30 | Now (active) |
| Dyson V15 Supurge | 24.999 TL | 14.999 TL | 34.999 TL | 15 | +10 min |

---

## 11. Sequence Diagrams

### Drop Lifecycle

```
Server Start → Seed demo drops (status: "upcoming")
                    │
                    ▼
          StartsAt time reached
          Engine sets status = "active"
          Hub broadcasts DROP_STATUS
                    │
                    ▼
          ┌─── Price Engine Loop ───┐
          │  tick every 10s          │
          │  calc new price          │◄── purchases feed into next tick
          │  broadcast PRICE_UPDATE  │
          │  broadcast STOCK_UPDATE  │
          └──────────┬──────────────┘
                     │
          stock == 0 OR EndsAt reached
                     │
                     ▼
          Engine sets status = "ended"
          Hub broadcasts DROP_ENDED
```

### Purchase Flow

```
Client                    Server
  │                         │
  │  POST /api/drops/X/buy  │
  │ ───────────────────────►│
  │                         │── Check drop is active
  │                         │── Check stock > 0
  │                         │── Lock price (current_price)
  │                         │── Decrement stock
  │                         │── Record purchase
  │                         │── Broadcast PURCHASE_FEED
  │                         │── Broadcast STOCK_UPDATE
  │  { purchase details }   │
  │ ◄───────────────────────│
```
