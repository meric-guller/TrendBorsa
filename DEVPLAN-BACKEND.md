# TrendBorsa - Backend Development Plan

**Status: COMPLETE**

All backend steps are implemented and verified.

---

## Steps (All Done)

### STEP 1: Seed Data
> **File:** `internal/store/memory.go`
> **Status:** COMPLETE
> - `Seed()` method creates 4 demo drops (2 active, 2 upcoming)
> - Called from `main.go` on startup
> - `GetRecentPurchaseCount()` and `GetUserPurchaseCount()` added

### STEP 2: Price Engine Core
> **File:** `internal/engine/price_engine.go`
> **Status:** COMPLETE
> - `Run(ctx)` goroutine with configurable ticker
> - Demand-based price recalculation each tick
> - Broadcasts `PRICE_UPDATE`, `STOCK_UPDATE`, `VIEWER_COUNT` events

### STEP 3: Drop Lifecycle
> **File:** `internal/engine/price_engine.go`
> **Status:** COMPLETE
> - `upcoming -> active` when `StartsAt` reached
> - `active -> ended` when `EndsAt` reached or stock depleted
> - Broadcasts `DROP_STATUS` and `DROP_ENDED` events

### STEP 4: Two-Step Buy Flow (Lock → Confirm)
> **Files:** `internal/handler/purchase_handler.go`, `internal/model/model.go`, `internal/store/memory.go`, `internal/engine/lock_cleaner.go`
> **Status:** COMPLETE
> - `POST /api/drops/{id}/lock` — locks price for 30s, reserves stock atomically
> - `POST /api/drops/{id}/confirm` — finalizes lock into purchase within 30s window
> - `POST /api/drops/{id}/cancel` — user cancels lock, stock restored
> - `PriceLock` model with status: pending/confirmed/expired/cancelled
> - `ReserveStock()` — atomic check-and-decrement (user limit counts pending locks + confirmed purchases)
> - `RestoreStock()` — returns stock on cancel/expiry
> - `LockCleaner` goroutine (5s ticker) expires stale locks and restores stock
> - New WS events: `PRICE_LOCK`, `LOCK_EXPIRED`
> - Broadcasts `STOCK_UPDATE` on lock/confirm/cancel/expiry, `PURCHASE_FEED` on confirm

### STEP 5: Admin Create Drop + Stats
> **File:** `internal/handler/drop_handler.go`
> **Status:** COMPLETE
> - `POST /api/drops` — create new drops with auto-calculated floor/ceiling
> - `GET /api/drops/{id}/stats` — viewer count, purchases, stock %, price change %, time remaining
> - `GET /api/drops?status=active` — filter by status

---

## API Surface

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Health check |
| `GET` | `/api/drops` | List drops (optional `?status=`) |
| `POST` | `/api/drops` | Create drop |
| `GET` | `/api/drops/{id}` | Drop detail |
| `GET` | `/api/drops/{id}/history` | Price history |
| `GET` | `/api/drops/{id}/stats` | Live stats |
| `POST` | `/api/drops/{id}/lock` | Lock price (30s) |
| `POST` | `/api/drops/{id}/confirm` | Confirm purchase |
| `POST` | `/api/drops/{id}/cancel` | Cancel lock |
| `WS` | `/ws/drops/{id}` | Real-time events |

## Running

```bash
make run                           # port 8080, tick 10s
PORT=9090 TICK_INTERVAL=5s make run  # custom
```
