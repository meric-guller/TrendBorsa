# TrendBorsa - Development Plan

## Current State Assessment

### DONE (Scaffolding)
- [x] Project structure (Go modules, chi router, gorilla/websocket)
- [x] Models: `Drop`, `Purchase`, `PricePoint`, all WS event types
- [x] Store: In-memory with `sync.RWMutex` — CRUD for drops, purchases, price history
- [x] Hub: WebSocket room management, broadcast, viewer count
- [x] WebSocket handler: upgrade, register/unregister, read loop
- [x] Drop handler: `GET /api/drops`, `GET /api/drops/{id}`, `GET /api/drops/{id}/history`
- [x] Config: port, tick interval, volatility, CORS
- [x] Main.go wiring: router, middleware, CORS, all routes
- [x] Makefile: run, build, clean

### SKELETON (Exists but empty)
- [ ] Price Engine — struct only, no `Run()`, no `recalculate()`, no `broadcast()`
- [ ] Purchase Handler — `Buy()` returns 501 Not Implemented

### MISSING (Not started)
- [ ] Seed data on startup
- [ ] Drop lifecycle management (upcoming -> active -> ended)
- [ ] Price algorithm implementation
- [ ] Buy endpoint (validation, stock decrement, price lock)
- [ ] Per-user purchase limits
- [ ] Stats endpoint (`/api/drops/{id}/stats`)
- [ ] Admin create drop endpoint (`POST /api/drops`)
- [ ] Android app (entire thing)

---

## Development Steps

### STEP 1: Seed Data
> **File:** `internal/store/memory.go`
> **What:** Add a `Seed()` method that creates 3-4 demo drops on startup.
> **Details:**
> - 1 drop with `status: "active"` (starts now, ends in 20 min)
> - 1 drop with `status: "upcoming"` (starts in 5 min)
> - 1 drop with `status: "upcoming"` (starts in 15 min)
> - Set realistic prices (floor = startPrice * 0.6, ceiling = startPrice * 1.4)
> - Call `Seed()` from `main.go` after `store.New()`
>
> **Depends on:** Nothing
> **Verify:** `make run` + `curl localhost:8080/api/drops` returns seeded drops

---

### STEP 2: Price Engine Core
> **File:** `internal/engine/price_engine.go`
> **What:** Implement the price recalculation goroutine.
> **Details:**
> - Add `Run(ctx context.Context)` method — starts a `time.Ticker` (from config)
> - On each tick, iterate all drops with `status == "active"`
> - Count purchases in last tick window using `store.GetRecentPurchaseCount(dropID, since)`
>   - Add `GetRecentPurchaseCount(dropID string, since time.Time) int` to store
> - Price formula:
>   ```
>   if purchases > 0:
>       increase = purchases * volatility * currentPrice
>       newPrice = currentPrice + increase
>   else:
>       decay = volatility * 0.5 * currentPrice
>       newPrice = currentPrice - decay
>   noise = random(-0.005, +0.005) * currentPrice
>   newPrice = clamp(newPrice + noise, floorPrice, ceilingPrice)
>   ```
> - Update drop's `CurrentPrice` in store
> - Add price point to history via `store.AddPricePoint()`
> - Broadcast `PriceUpdateEvent` via hub
> - Broadcast `StockUpdateEvent` via hub
> - Broadcast `ViewerCountEvent` via hub
> - Call `engine.Run(ctx)` as a goroutine from `main.go`
>
> **Depends on:** Step 1 (needs drops to exist)
> **Verify:** `make run`, connect to `ws://localhost:8080/ws/drops/{id}`, see price updates arriving every tick

---

### STEP 3: Drop Lifecycle
> **File:** `internal/engine/price_engine.go` (add to tick loop)
> **What:** Manage status transitions based on time and stock.
> **Details:**
> - In each tick, also check all drops:
>   - If `status == "upcoming"` and `time.Now() >= StartsAt` → set `status = "active"`, broadcast `DropStatusEvent`
>   - If `status == "active"` and (`time.Now() >= EndsAt` or `RemainingStock <= 0`) → set `status = "ended"`, broadcast `DropEndedEvent`
> - Only run price recalculation for `status == "active"` drops
>
> **Depends on:** Step 2
> **Verify:** Create a drop starting in 30s, watch it transition from upcoming -> active via WS

---

### STEP 4: Buy Endpoint
> **File:** `internal/handler/purchase_handler.go`
> **What:** Implement the `POST /api/drops/{id}/buy` endpoint.
> **Details:**
> - Parse request body: `{ "user_id": "string", "quantity": int }`
> - Validations:
>   - Drop exists and `status == "active"`
>   - `quantity >= 1` and `quantity <= 3` (max per transaction)
>   - Per-user limit: count user's existing purchases for this drop, reject if `total + quantity > 3`
>     - Add `GetUserPurchaseCount(dropID, userID string) int` to store
>   - `RemainingStock >= quantity`
> - On success:
>   - Lock current price (use `store.mu.Lock` to atomically read price + decrement stock)
>   - Create `Purchase` record with `price = currentPrice` at lock time
>   - Decrement `RemainingStock` by quantity
>   - Save purchase via `store.AddPurchase()`
>   - Broadcast `PurchaseFeedEvent` via hub
>   - Broadcast `StockUpdateEvent` via hub
>   - Return 200 with purchase details JSON
> - On failure: return appropriate error (400/409/410)
>
> **Depends on:** Steps 1-3
> **Verify:** `curl -X POST localhost:8080/api/drops/{id}/buy -d '{"user_id":"test","quantity":1}'`

---

### STEP 5: Admin Create Drop + Stats
> **File:** `internal/handler/drop_handler.go`
> **What:** Add `POST /api/drops` and `GET /api/drops/{id}/stats`.
> **Details:**
> - `POST /api/drops` — parse body, create Drop with generated ID, save to store
>   - Required fields: `product_name`, `start_price`, `total_stock`, `starts_at`, `ends_at`
>   - Auto-calculate: `floor_price`, `ceiling_price`, `current_price = start_price`
>   - Register route in `main.go`
> - `GET /api/drops/{id}/stats` — return:
>   ```json
>   {
>     "viewer_count": 847,
>     "total_purchases": 23,
>     "remaining_stock_pct": 0.54,
>     "price_change_pct": 12.5,
>     "time_remaining_seconds": 542
>   }
>   ```
>   - Register route in `main.go`
>
> **Depends on:** Step 4
> **Verify:** Create a drop via POST, verify it shows in GET /api/drops

---

### STEP 6: Android — Project Setup + Models
> **What:** Create Android project with Compose, set up dependencies.
> **Details:**
> - Create new Android project (or use existing KMP setup per ARCHITECTURE.md)
> - Add dependencies: OkHttp, Retrofit, kotlinx.serialization, Hilt, Compose Navigation
> - Create data models mirroring backend:
>   - `Drop`, `Purchase`, `PricePoint`, `PriceUpdate`
>   - `PriceTrend` enum, `SessionStatus` enum
>   - WS event sealed class hierarchy
> - Create `FlashAuctionApi` Retrofit interface
> - Create `PriceStreamClient` (OkHttp WebSocket)
> - Create `FlashAuctionRepository`
> - Set up Hilt DI module
>
> **Depends on:** Steps 1-5 (backend must be runnable)
> **Verify:** App builds, repository can call `GET /api/drops` from emulator

---

### STEP 7: Android — Main Auction Screen
> **What:** Build the core auction screen with live data.
> **Details:**
> - `FlashAuctionViewModel`:
>   - Connect to WS on init, collect price updates
>   - Manage `currentPrice`, `priceTrend`, `stockPercentage`, `priceHistory`, `remainingSeconds`
>   - Countdown timer via `delay(1000)` loop
> - `FlashAuctionScreen` composable:
>   - Dark header bar (CANLI badge + viewer count + countdown timer)
>   - Product strip (thumbnail, brand, name, base price)
>   - Big price display (colored by trend, animated)
>   - Price chart (Compose Canvas — line + area gradient)
>   - Stats row (start price, lowest, highest)
>   - Stock progress bar (gradient green->yellow->red)
>   - Activity feed (last 3 purchases from WS events)
>   - Sticky bottom bar (current price + "Satin Al" button)
>
> **Depends on:** Step 6
> **Verify:** Screen renders with live price updates from backend WS

---

### STEP 8: Android — Buy Flow
> **What:** Implement price lock bottom sheet and purchase confirmation.
> **Details:**
> - On "Satin Al" tap → call `POST /api/drops/{id}/buy`
> - Show `PriceLockBottomSheet`:
>   - Lock icon + "Fiyat Kilitlendi!" title
>   - Locked price card with green border
>   - 30s countdown ring (animated circular progress)
>   - Quantity selector (1-3, with +/- buttons)
>   - Total row
>   - "Satin Alimi Onayla" green button
>   - "Vazgec" cancel
> - On confirm → show success screen or error
> - `PurchaseSuccessScreen`:
>   - Checkmark icon
>   - Savings badge ("X TL kazandin!")
>   - Receipt breakdown (product, qty, base price, borsa price, total)
>   - "Siparislerime Git" + "Alisverise Devam Et" buttons
> - Error states: stock out, session ended, limit exceeded
>
> **Depends on:** Step 7
> **Verify:** Full buy flow works end-to-end against running backend

---

### STEP 9: Android — Drop List + Navigation
> **What:** Entry screen showing available/upcoming drops.
> **Details:**
> - `DropListScreen` — shows all drops from `GET /api/drops`
>   - Active drops: orange "CANLI" badge, current price, stock bar
>   - Upcoming drops: countdown to start, "Hatırlat" (remind) button
>   - Ended drops: grayed out, final stats
> - Navigation: DropList -> FlashAuction -> PurchaseSuccess
> - Compose Navigation with `NavHost`
> - Session ended screen (when timer hits 0 or stock depletes while watching)
>
> **Depends on:** Step 8
> **Verify:** Can navigate from list to auction to purchase to success

---

### STEP 10: Polish
> **What:** Visual polish, animations, edge cases.
> **Details:**
> - Price change animation (scale + color flash on update)
> - Haptic feedback on significant price moves
> - Smooth chart animation when new points arrive
> - Stock bar color transition animation
> - Countdown urgency (red flash when < 60s)
> - Loading/error/empty states
> - WebSocket reconnection on disconnect
> - Pull-to-refresh on drop list
>
> **Depends on:** Step 9
> **Verify:** Demo-ready, smooth UX

---

## Step Dependency Graph

```
Step 1 (Seed Data)
  └──> Step 2 (Price Engine)
         └──> Step 3 (Lifecycle)
                └──> Step 4 (Buy Endpoint)
                       └──> Step 5 (Admin + Stats)
                              └──> Step 6 (Android Setup)
                                     └──> Step 7 (Auction Screen)
                                            └──> Step 8 (Buy Flow)
                                                   └──> Step 9 (Drop List + Nav)
                                                          └──> Step 10 (Polish)
```

**Parallel opportunity:** Steps 6-7 can start as soon as Step 3 is done (buy can use mock data initially). Steps 1-5 are strictly sequential.

---

## Quick Commands

```bash
# Run backend
cd TrendBorsa && make run

# Test endpoints
curl localhost:8080/api/health
curl localhost:8080/api/drops
curl localhost:8080/api/drops/{id}
curl localhost:8080/api/drops/{id}/history

# Test WebSocket (wscat)
npx wscat -c ws://localhost:8080/ws/drops/{id}

# Test buy
curl -X POST localhost:8080/api/drops/{id}/buy \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user1","quantity":1}'
```
