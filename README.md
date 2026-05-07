# TrendBorsa Backend

Real-time, demand-driven pricing engine for flash sales. Products go on sale at scheduled times. Prices move **up** when people buy and **down** when demand drops — like a stock market for products.

## Quick Start

```bash
# Run the server (port 8080, price tick every 10s)
make run

# Custom config
PORT=9090 TICK_INTERVAL=5s make run
```

The server seeds 4 demo drops on startup (2 active, 2 upcoming). The price engine starts immediately and recalculates prices every tick interval.

---

## Android Integration Guide

### Base URL

```
# Emulator → localhost
http://10.0.2.2:8080

# Physical device → use your machine's local IP
http://192.168.X.X:8080
```

### Overview

The Android app needs to:
1. **REST** — fetch drop list, drop details, stats, and perform purchases
2. **WebSocket** — connect to a drop room for real-time price/stock/purchase events

---

## REST API

### 1. List All Drops

```
GET /api/drops
GET /api/drops?status=active      // filter: "active", "upcoming", "ended"
```

**Response** `200 OK`
```json
[
  {
    "id": "drop-1",
    "product_name": "Sony WH-1000XM5 Kulaklık",
    "product_image": "https://cdn.trendyol.com/sony-wh1000xm5.jpg",
    "description": "Aktif Gürültü Önleme özellikli premium kablosuz kulaklık",
    "start_price": 8999,
    "current_price": 9496.01,
    "floor_price": 5399,
    "ceiling_price": 12599,
    "total_stock": 50,
    "remaining_stock": 48,
    "starts_at": "2026-05-07T16:34:33+03:00",
    "ends_at": "2026-05-07T16:54:33+03:00",
    "status": "active",
    "volatility": 0.03,
    "created_at": "2026-05-07T16:35:33+03:00"
  }
]
```

**Android usage:** Call on app launch to show the drop list screen. Use `status` filter to separate active/upcoming/ended sections.

---

### 2. Get Drop Detail

```
GET /api/drops/{id}
```

**Response** `200 OK` — same shape as a single item from the list above.

**Response** `404`
```json
{ "error": "drop not found" }
```

**Android usage:** Call when user taps a drop card. Use `current_price` for the big price display, `start_price` for the base price reference, `remaining_stock / total_stock` for the stock progress bar.

---

### 3. Get Price History (for chart)

```
GET /api/drops/{id}/history
```

**Response** `200 OK`
```json
[
  { "price": 8999.00, "timestamp": "2026-05-07T16:35:33+03:00" },
  { "price": 8864.15, "timestamp": "2026-05-07T16:35:43+03:00" },
  { "price": 8741.87, "timestamp": "2026-05-07T16:35:53+03:00" },
  { "price": 9128.44, "timestamp": "2026-05-07T16:36:03+03:00" }
]
```

**Android usage:** Call once when entering the auction screen to populate the initial chart. After that, update the chart from WebSocket `PRICE_UPDATE` events (append new points). Each point is ~10 seconds apart (tick interval).

---

### 4. Get Live Stats

```
GET /api/drops/{id}/stats
```

**Response** `200 OK`
```json
{
  "viewer_count": 847,
  "total_purchases": 23,
  "remaining_stock_pct": 94.0,
  "price_change_pct": 5.52,
  "time_remaining_seconds": 1136
}
```

| Field | Type | Description |
|-------|------|-------------|
| `viewer_count` | int | Number of active WebSocket connections for this drop |
| `total_purchases` | int | Total units sold so far |
| `remaining_stock_pct` | float | Remaining stock as percentage (0.0 - 100.0) |
| `price_change_pct` | float | Price change from `start_price` as percentage (can be negative) |
| `time_remaining_seconds` | int | Seconds until drop ends (0 if not active) |

**Android usage:** Call once on screen load for initial stats. After that, viewer count and stock come from WebSocket events. Use `time_remaining_seconds` to seed the countdown timer, then decrement locally every second.

---

### 5. Buy (Purchase at Current Price)

```
POST /api/drops/{id}/buy
Content-Type: application/json

{
  "user_id": "unique-user-id",
  "quantity": 1
}
```

| Field | Type | Rules |
|-------|------|-------|
| `user_id` | string | Required. Unique identifier for the user. |
| `quantity` | int | Required. Must be 1-3. Total per user per drop cannot exceed 3. |

**Response** `200 OK` — purchase successful
```json
{
  "purchase": {
    "id": "417fc2c9-22f4-4875-af86-b84fe82a7edc",
    "drop_id": "drop-1",
    "user_id": "user1",
    "price": 8999.00,
    "quantity": 2,
    "created_at": "2026-05-07T16:35:37+03:00"
  },
  "message": "Purchase successful"
}
```

**Error responses:**

| Status | Body | When |
|--------|------|------|
| `400` | `{"error": "invalid request body"}` | Malformed JSON |
| `400` | `{"error": "user_id is required"}` | Missing user_id |
| `400` | `{"error": "quantity must be between 1 and 3"}` | Invalid quantity |
| `404` | `{"error": "drop not found"}` | Invalid drop ID |
| `409` | `{"error": "drop is not active"}` | Drop is upcoming or ended |
| `409` | `{"error": "purchase limit exceeded (max 3 per user)"}` | User already bought 3 |
| `409` | `{"error": "not enough stock"}` | Remaining stock < requested qty |

**Android usage:** Call when user confirms purchase in the bottom sheet. The `purchase.price` is the locked price at the moment of the request. Show the success screen with `purchase.price` vs `start_price` to calculate savings.

**Important:** The price is locked atomically at request time — it won't change between the user tapping "buy" and the response arriving. No separate lock-then-confirm step needed.

---

### 6. Create Drop (Admin)

```
POST /api/drops
Content-Type: application/json

{
  "product_name": "Samsung Galaxy S25",
  "product_image": "https://cdn.example.com/s25.jpg",
  "description": "Flagship telefon",
  "start_price": 64999,
  "total_stock": 25,
  "starts_at": "2026-05-07T17:00:00+03:00",
  "ends_at": "2026-05-07T17:20:00+03:00",
  "volatility": 0.04
}
```

| Field | Type | Required | Default |
|-------|------|----------|---------|
| `product_name` | string | Yes | — |
| `product_image` | string | No | `""` |
| `description` | string | No | `""` |
| `start_price` | float | Yes | — |
| `total_stock` | int | Yes | — |
| `starts_at` | string (RFC3339) | No | now + 5 minutes |
| `ends_at` | string (RFC3339) | No | starts_at + 20 minutes |
| `volatility` | float | No | 0.03 |

**Response** `201 Created`
```json
{
  "id": "ec454ab4-1431-4e90-850e-c8a77a440456",
  "product_name": "Samsung Galaxy S25",
  "product_image": "https://cdn.example.com/s25.jpg",
  "description": "Flagship telefon",
  "start_price": 64999,
  "current_price": 64999,
  "floor_price": 38999,
  "ceiling_price": 90999,
  "total_stock": 25,
  "remaining_stock": 25,
  "starts_at": "2026-05-07T17:00:00+03:00",
  "ends_at": "2026-05-07T17:20:00+03:00",
  "status": "upcoming",
  "volatility": 0.04,
  "created_at": "2026-05-07T16:35:37+03:00"
}
```

`floor_price` and `ceiling_price` are auto-calculated: `start_price * 0.6` and `start_price * 1.4`.

**Android usage:** Optional admin/debug screen to create new drops for demo purposes.

---

### 7. Health Check

```
GET /api/health
```

**Response** `200 OK`
```json
{ "status": "ok" }
```

---

## WebSocket — Real-Time Events

### Connect

```
ws://HOST:8080/ws/drops/{id}
```

Open a WebSocket connection to join a drop's room. The server pushes events as JSON messages. The client doesn't need to send any messages — just keep the connection alive.

### OkHttp Example (Kotlin)

```kotlin
val client = OkHttpClient()
val request = Request.Builder()
    .url("ws://10.0.2.2:8080/ws/drops/$dropId")
    .build()

client.newWebSocket(request, object : WebSocketListener() {
    override fun onMessage(webSocket: WebSocket, text: String) {
        val json = JSONObject(text)
        when (json.getString("type")) {
            "PRICE_UPDATE"  -> handlePriceUpdate(json)
            "STOCK_UPDATE"  -> handleStockUpdate(json)
            "PURCHASE_FEED" -> handlePurchaseFeed(json)
            "VIEWER_COUNT"  -> handleViewerCount(json)
            "DROP_STATUS"   -> handleDropStatus(json)
            "DROP_ENDED"    -> handleDropEnded(json)
        }
    }

    override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
        // Reconnect after delay
    }
})
```

### Event Types

#### PRICE_UPDATE
Sent every tick (default 10s) for active drops.

```json
{
  "type": "PRICE_UPDATE",
  "drop_id": "drop-1",
  "price": 9496.01,
  "prev_price": 8999.00,
  "direction": "UP"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `price` | float | New current price |
| `prev_price` | float | Previous price (before this tick) |
| `direction` | string | `"UP"`, `"DOWN"`, or `"STABLE"` |

**Android usage:**
- Update the big price display with `price`
- Set trend arrow/color based on `direction` (UP = red arrow up, DOWN = green arrow down)
- Append a new `PricePoint(price, now)` to the chart data
- Calculate percentage change: `(price - prev_price) / prev_price * 100`

---

#### STOCK_UPDATE
Sent every tick AND after each purchase.

```json
{
  "type": "STOCK_UPDATE",
  "drop_id": "drop-1",
  "remaining_pct": 94.0
}
```

| Field | Type | Description |
|-------|------|-------------|
| `remaining_pct` | float | Remaining stock as percentage (0.0 - 100.0) |

**Android usage:** Update the stock progress bar. Color logic:
- `> 50%` → green
- `20-50%` → yellow/orange
- `< 20%` → red

---

#### PURCHASE_FEED
Sent immediately when someone buys.

```json
{
  "type": "PURCHASE_FEED",
  "drop_id": "drop-1",
  "price": 9496.01,
  "message": "m***c 9496.01 TL'den aldı!"
}
```

**Android usage:** Show in the activity feed at the bottom of the auction screen. Keep the last 5-10 items. Username is already masked by the server.

---

#### VIEWER_COUNT
Sent every tick.

```json
{
  "type": "VIEWER_COUNT",
  "drop_id": "drop-1",
  "count": 847
}
```

**Android usage:** Update "X kisi izliyor" text in the header.

---

#### DROP_STATUS
Sent when a drop transitions between states.

```json
{
  "type": "DROP_STATUS",
  "drop_id": "drop-1",
  "status": "active"
}
```

**Android usage:** If you're on a drop screen showing "upcoming" and receive `status: "active"`, transition the UI from the waiting/countdown state to the live auction view.

---

#### DROP_ENDED
Sent when a drop ends (time expired or stock depleted).

```json
{
  "type": "DROP_ENDED",
  "drop_id": "drop-1",
  "final_price": 9200.50,
  "total_sold": 47
}
```

**Android usage:** Show the "Satis Sona Erdi" (session ended) screen with final price and total sold stats. Disable the buy button.

---

## Typical Android Screen Flow

### 1. Drop List Screen
```
onLaunch:
  GET /api/drops → populate list
  
  For each active drop card, show:
    - product_name, product_image
    - current_price (with start_price as strikethrough)
    - remaining_stock / total_stock as progress bar
    - Time remaining (calculated from ends_at)
  
  For upcoming drops:
    - Countdown to starts_at
```

### 2. Auction Screen (after tapping an active drop)
```
onEnter:
  GET /api/drops/{id}          → populate product info, current price
  GET /api/drops/{id}/history  → populate initial chart
  GET /api/drops/{id}/stats    → seed viewer count, countdown timer
  WS  /ws/drops/{id}           → connect for real-time updates

onPriceUpdate:
  → update price display + trend arrow
  → append point to chart
  → animate price change (scale + color flash)

onStockUpdate:
  → update stock progress bar

onPurchaseFeed:
  → prepend to activity feed (keep last 5)

onViewerCount:
  → update "X kisi izliyor"

onDropEnded:
  → show ended overlay / navigate to ended screen

countdownTimer (local, every 1s):
  → decrement time_remaining_seconds
  → when 0 → show ended state
```

### 3. Buy Flow
```
User taps "Satin Al":
  POST /api/drops/{id}/buy { user_id, quantity }
  
  if 200:
    → show success bottom sheet / screen
    → display purchase.price as locked price
    → calculate savings: start_price - purchase.price
  
  if 409 "purchase limit exceeded":
    → show "Limit asildi" toast
  
  if 409 "not enough stock":
    → show "Stok tukendi" state
  
  if 409 "drop is not active":
    → show ended state
```

### 4. Success Screen
```
From purchase response:
  - product_name (from drop detail)
  - purchase.price (borsa price)
  - start_price (normal price, strikethrough)
  - savings = start_price - purchase.price
  - purchase.quantity
  - total = purchase.price * purchase.quantity
```

---

## Data Models (Kotlin)

```kotlin
@Serializable
data class Drop(
    val id: String,
    @SerialName("product_name") val productName: String,
    @SerialName("product_image") val productImage: String,
    val description: String,
    @SerialName("start_price") val startPrice: Double,
    @SerialName("current_price") val currentPrice: Double,
    @SerialName("floor_price") val floorPrice: Double,
    @SerialName("ceiling_price") val ceilingPrice: Double,
    @SerialName("total_stock") val totalStock: Int,
    @SerialName("remaining_stock") val remainingStock: Int,
    @SerialName("starts_at") val startsAt: String,
    @SerialName("ends_at") val endsAt: String,
    val status: String,         // "upcoming", "active", "ended"
    val volatility: Double,
    @SerialName("created_at") val createdAt: String
)

@Serializable
data class PricePoint(
    val price: Double,
    val timestamp: String
)

@Serializable
data class Stats(
    @SerialName("viewer_count") val viewerCount: Int,
    @SerialName("total_purchases") val totalPurchases: Int,
    @SerialName("remaining_stock_pct") val remainingStockPct: Double,
    @SerialName("price_change_pct") val priceChangePct: Double,
    @SerialName("time_remaining_seconds") val timeRemainingSeconds: Int
)

@Serializable
data class BuyRequest(
    @SerialName("user_id") val userId: String,
    val quantity: Int
)

@Serializable
data class BuyResponse(
    val purchase: Purchase,
    val message: String
)

@Serializable
data class Purchase(
    val id: String,
    @SerialName("drop_id") val dropId: String,
    @SerialName("user_id") val userId: String,
    val price: Double,
    val quantity: Int,
    @SerialName("created_at") val createdAt: String
)

// WebSocket events — parse "type" field first, then deserialize
@Serializable
data class PriceUpdateEvent(
    val type: String,           // "PRICE_UPDATE"
    @SerialName("drop_id") val dropId: String,
    val price: Double,
    @SerialName("prev_price") val prevPrice: Double,
    val direction: String       // "UP", "DOWN", "STABLE"
)

@Serializable
data class StockUpdateEvent(
    val type: String,           // "STOCK_UPDATE"
    @SerialName("drop_id") val dropId: String,
    @SerialName("remaining_pct") val remainingPct: Double
)

@Serializable
data class PurchaseFeedEvent(
    val type: String,           // "PURCHASE_FEED"
    @SerialName("drop_id") val dropId: String,
    val price: Double,
    val message: String
)

@Serializable
data class ViewerCountEvent(
    val type: String,           // "VIEWER_COUNT"
    @SerialName("drop_id") val dropId: String,
    val count: Int
)

@Serializable
data class DropStatusEvent(
    val type: String,           // "DROP_STATUS"
    @SerialName("drop_id") val dropId: String,
    val status: String
)

@Serializable
data class DropEndedEvent(
    val type: String,           // "DROP_ENDED"
    @SerialName("drop_id") val dropId: String,
    @SerialName("final_price") val finalPrice: Double,
    @SerialName("total_sold") val totalSold: Int
)
```

---

## Retrofit Interface

```kotlin
interface TrendBorsaApi {

    @GET("api/drops")
    suspend fun getDrops(@Query("status") status: String? = null): List<Drop>

    @GET("api/drops/{id}")
    suspend fun getDrop(@Path("id") id: String): Drop

    @GET("api/drops/{id}/history")
    suspend fun getHistory(@Path("id") id: String): List<PricePoint>

    @GET("api/drops/{id}/stats")
    suspend fun getStats(@Path("id") id: String): Stats

    @POST("api/drops/{id}/buy")
    suspend fun buy(@Path("id") id: String, @Body request: BuyRequest): BuyResponse

    @POST("api/drops")
    suspend fun createDrop(@Body request: CreateDropRequest): Drop
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `TICK_INTERVAL` | `10s` | Price recalculation interval |
| `DEFAULT_VOLATILITY` | `0.03` | Default volatility for new drops |
| `CORS_ORIGIN` | `*` | CORS allowed origins |

---

## Price Algorithm

Every tick, for each active drop:

```
if purchases_in_last_tick > 0:
    price goes UP:  new = current + (purchases × volatility × current)
else:
    price decays DOWN:  new = current - (volatility × 0.5 × current)

noise = random(±0.5%) × current
new = clamp(new + noise, floor_price, ceiling_price)
```

- More purchases in a tick window → bigger price jump
- No purchases → slow decay toward floor
- `volatility` (0.01-0.10) controls sensitivity
- Price is always clamped between `floor_price` and `ceiling_price`

---

## Seed Data

On startup, the server creates these demo drops:

| ID | Product | Start Price | Floor | Ceiling | Stock | Status |
|----|---------|------------|-------|---------|-------|--------|
| drop-1 | Sony WH-1000XM5 Kulaklık | 8.999 TL | 5.399 TL | 12.599 TL | 50 | active |
| drop-2 | iPhone 16 Pro Max Kılıf | 599 TL | 359 TL | 839 TL | 200 | upcoming (5 min) |
| drop-3 | Nike Air Max 90 | 4.299 TL | 2.579 TL | 6.019 TL | 30 | active |
| drop-4 | Dyson V15 Süpürge | 24.999 TL | 14.999 TL | 34.999 TL | 15 | upcoming (10 min) |
