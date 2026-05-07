# TrendBorsa - Android Development Plan

**Stack:** Jetpack Compose + MVVM + Hilt + OkHttp/Retrofit

**Backend:** Running at `http://10.0.2.2:8080` (emulator) or `http://<LOCAL_IP>:8080` (device)

---

## Steps

### STEP 1: Project Setup + Models
> **What:** Create Android project, add dependencies, define data models.
> **Details:**
> - Dependencies: OkHttp, Retrofit, kotlinx.serialization, Hilt, Compose Navigation
> - Create all data models (see README.md for exact Kotlin classes):
>   - `Drop`, `Purchase`, `PricePoint`, `Stats`, `BuyRequest`, `BuyResponse`
>   - WS event classes: `PriceUpdateEvent`, `StockUpdateEvent`, `PurchaseFeedEvent`, `ViewerCountEvent`, `DropStatusEvent`, `DropEndedEvent`
> - Create `TrendBorsaApi` Retrofit interface
> - Create `PriceStreamClient` (OkHttp WebSocket)
> - Create `FlashAuctionRepository` combining REST + WS
> - Set up Hilt DI module
>
> **Depends on:** Backend running
> **Verify:** App builds, `GET /api/drops` returns data in logcat

---

### STEP 2: Main Auction Screen
> **What:** Build the core auction screen with live price data.
> **Details:**
> - `FlashAuctionViewModel`:
>   - On init: fetch drop detail + history + stats, connect WS
>   - Collect WS events → update `@Composable` state
>   - Local countdown timer via `delay(1000)` loop
> - `FlashAuctionScreen` composable:
>   - Dark header (CANLI badge + viewer count + countdown)
>   - Product strip (thumbnail, brand, name, base price strikethrough)
>   - Big price display (green if below start, red if above, animated)
>   - Price chart (Compose Canvas — line path + area gradient)
>   - Stats row: 3 cards (start price, lowest, highest)
>   - Stock progress bar (gradient green→yellow→red)
>   - Activity feed (last 3 `PURCHASE_FEED` events)
>   - Sticky bottom bar (current price + "Satin Al" button)
>
> **Depends on:** Step 1
> **Verify:** Screen shows live price updates from backend WS

---

### STEP 3: Buy Flow
> **What:** Purchase flow with bottom sheet confirmation.
> **Details:**
> - On "Satin Al" tap → `POST /api/drops/{id}/buy`
> - Show `PriceLockBottomSheet`:
>   - Lock icon + "Fiyat Kilitlendi!" title
>   - Locked price card (green border)
>   - Quantity selector (1-3)
>   - Total row
>   - "Satin Alimi Onayla" green button
>   - "Vazgec" cancel
> - `PurchaseSuccessScreen`:
>   - Checkmark + "Satin Alma Basarili!"
>   - Savings badge ("X TL kazandin!")
>   - Receipt: product, qty, base price (strikethrough), borsa price, total
>   - "Siparislerime Git" + "Alisverise Devam Et"
> - Error handling:
>   - `409 "purchase limit exceeded"` → toast
>   - `409 "not enough stock"` → sold out state
>   - `409 "drop is not active"` → ended state
>
> **Depends on:** Step 2
> **Verify:** Full buy flow works end-to-end

---

### STEP 4: Drop List + Navigation
> **What:** Entry screen + navigation between screens.
> **Details:**
> - `DropListScreen`:
>   - Active drops: orange CANLI badge, price, stock bar, time remaining
>   - Upcoming drops: countdown to start, "Hatırlat" button
>   - Ended drops: grayed out, final stats
> - Navigation: `NavHost` with routes:
>   - `dropList` → `auction/{dropId}` → `success/{purchaseId}`
> - Session ended screen (DROP_ENDED event or timer hits 0)
>
> **Depends on:** Step 3
> **Verify:** Navigate from list → auction → buy → success

---

### STEP 5: Polish
> **What:** Animations, edge cases, demo readiness.
> **Details:**
> - Price change: scale animation + color flash on update
> - Haptic feedback on significant price moves
> - Smooth chart animation on new points
> - Stock bar color transition
> - Countdown urgency (red flash < 60s)
> - Loading/error/empty states
> - WS reconnect on disconnect
> - Pull-to-refresh on drop list
>
> **Depends on:** Step 4
> **Verify:** Demo-ready, smooth UX

---

## Dependency Graph

```
Step 1 (Setup + Models)
  └──> Step 2 (Auction Screen)
         └──> Step 3 (Buy Flow)
                └──> Step 4 (Drop List + Nav)
                       └──> Step 5 (Polish)
```
