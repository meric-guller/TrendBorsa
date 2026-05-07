# TrendBorsa Backend

Real-time, demand-driven pricing engine for flash sales. Prices move **up** when people buy and **down** when demand drops — like a stock market for products.

## Quick Start

```bash
make run                             # port 8080, tick 10s
PORT=9090 TICK_INTERVAL=5s make run  # custom
```

Seeds 4 demo drops on startup (2 active, 2 upcoming). Price engine ticks immediately.

---

## Client Connection

| Platform | REST Base URL | WebSocket URL |
|----------|---------------|---------------|
| iOS Simulator | `http://localhost:8080` | `ws://localhost:8080` |
| iOS Device | `http://<YOUR_IP>:8080` | `ws://<YOUR_IP>:8080` |
| Android Emulator | `http://10.0.2.2:8080` | `ws://10.0.2.2:8080` |
| Android Device | `http://<YOUR_IP>:8080` | `ws://<YOUR_IP>:8080` |

> iOS: Add `NSAppTransportSecurity > NSAllowsLocalNetworking = YES` to Info.plist for local HTTP.

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

---

### 2. Get Drop Detail

```
GET /api/drops/{id}
```

**Response** `200 OK` — same shape as list item above.

**Response** `404` — `{"error": "drop not found"}`

---

### 3. Price History (for chart)

```
GET /api/drops/{id}/history
```

**Response** `200 OK`
```json
[
  {"price": 8999.00, "timestamp": "2026-05-07T16:35:33+03:00"},
  {"price": 8864.15, "timestamp": "2026-05-07T16:35:43+03:00"},
  {"price": 9128.44, "timestamp": "2026-05-07T16:36:03+03:00"}
]
```

Call once on screen entry to seed the chart. After that, append from `PRICE_UPDATE` WebSocket events.

---

### 4. Live Stats

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
| `viewer_count` | int | Active WebSocket connections |
| `total_purchases` | int | Units sold |
| `remaining_stock_pct` | float | 0.0 - 100.0 |
| `price_change_pct` | float | From start_price (can be negative) |
| `time_remaining_seconds` | int | Seconds until drop ends |

Call once on entry. Use `time_remaining_seconds` to seed countdown, then decrement locally.

---

### 5. Buy

```
POST /api/drops/{id}/buy
Content-Type: application/json

{"user_id": "unique-user-id", "quantity": 1}
```

| Field | Type | Rules |
|-------|------|-------|
| `user_id` | string | Required |
| `quantity` | int | 1-3. Total per user per drop max 3. |

**Response** `200 OK`
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

**Errors:**

| Status | Error | When |
|--------|-------|------|
| `400` | `invalid request body` | Bad JSON |
| `400` | `user_id is required` | Missing field |
| `400` | `quantity must be between 1 and 3` | Invalid qty |
| `404` | `drop not found` | Bad ID |
| `409` | `drop is not active` | Not active |
| `409` | `purchase limit exceeded (max 3 per user)` | Over limit |
| `409` | `not enough stock` | Sold out |

Price is locked atomically at request time. No separate lock step needed.

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

| Field | Required | Default |
|-------|----------|---------|
| `product_name` | Yes | — |
| `start_price` | Yes | — |
| `total_stock` | Yes | — |
| `product_image` | No | `""` |
| `description` | No | `""` |
| `starts_at` | No | now + 5 min |
| `ends_at` | No | starts_at + 20 min |
| `volatility` | No | 0.03 |

`floor_price` = `start_price * 0.6`, `ceiling_price` = `start_price * 1.4` (auto-calculated).

---

### 7. Health Check

```
GET /api/health  →  {"status": "ok"}
```

---

## WebSocket

### Connect

```
ws://HOST:8080/ws/drops/{id}
```

Join a drop's room. Server pushes JSON events. Client doesn't need to send messages.

---

### Event Types

#### PRICE_UPDATE (every tick)
```json
{"type": "PRICE_UPDATE", "drop_id": "drop-1", "price": 9496.01, "prev_price": 8999.00, "direction": "UP"}
```
`direction`: `"UP"`, `"DOWN"`, or `"STABLE"`

#### STOCK_UPDATE (every tick + after purchase)
```json
{"type": "STOCK_UPDATE", "drop_id": "drop-1", "remaining_pct": 94.0}
```

#### PURCHASE_FEED (after each purchase)
```json
{"type": "PURCHASE_FEED", "drop_id": "drop-1", "price": 9496.01, "message": "m***c 9496.01 TL'den aldı!"}
```

#### VIEWER_COUNT (every tick)
```json
{"type": "VIEWER_COUNT", "drop_id": "drop-1", "count": 847}
```

#### DROP_STATUS (on state transition)
```json
{"type": "DROP_STATUS", "drop_id": "drop-1", "status": "active"}
```

#### DROP_ENDED (when drop ends)
```json
{"type": "DROP_ENDED", "drop_id": "drop-1", "final_price": 9200.50, "total_sold": 47}
```

---

## Screen Flow (Both Platforms)

### 1. Drop List
```
onAppear:
  GET /api/drops → show list grouped by status
  Active: CANLI badge, price, stock bar, time remaining
  Upcoming: countdown to start
  Ended: grayed out
```

### 2. Auction Screen
```
onEnter:
  GET /api/drops/{id}          → product info
  GET /api/drops/{id}/history  → seed chart
  GET /api/drops/{id}/stats    → seed countdown + viewer count
  WS  /ws/drops/{id}           → real-time updates

  PRICE_UPDATE  → update price + chart + trend arrow
  STOCK_UPDATE  → update stock bar
  PURCHASE_FEED → prepend to activity feed
  VIEWER_COUNT  → update header
  DROP_ENDED    → show ended state
```

### 3. Buy Flow
```
Tap "Satin Al":
  POST /api/drops/{id}/buy {user_id, quantity}
  200 → success screen (show savings: start_price - purchase.price)
  409 → error toast/alert
```

---

## Data Models

### Swift (iOS)

```swift
struct Drop: Codable {
    let id: String
    let productName: String
    let productImage: String
    let description: String
    let startPrice: Double
    let currentPrice: Double
    let floorPrice: Double
    let ceilingPrice: Double
    let totalStock: Int
    let remainingStock: Int
    let startsAt: String
    let endsAt: String
    let status: String           // "upcoming", "active", "ended"
    let volatility: Double
    let createdAt: String
}

struct PricePoint: Codable, Identifiable {
    var id: String { timestamp }
    let price: Double
    let timestamp: String
}

struct Stats: Codable {
    let viewerCount: Int
    let totalPurchases: Int
    let remainingStockPct: Double
    let priceChangePct: Double
    let timeRemainingSeconds: Int
}

struct BuyRequest: Codable {
    let userId: String
    let quantity: Int
}

struct BuyResponse: Codable {
    let purchase: Purchase
    let message: String
}

struct Purchase: Codable {
    let id: String
    let dropId: String
    let userId: String
    let price: Double
    let quantity: Int
    let createdAt: String
}

struct ErrorResponse: Codable {
    let error: String
}

// WS Events
enum WSEvent {
    case priceUpdate(price: Double, prevPrice: Double, direction: String)
    case stockUpdate(remainingPct: Double)
    case purchaseFeed(price: Double, message: String)
    case viewerCount(count: Int)
    case dropStatus(status: String)
    case dropEnded(finalPrice: Double, totalSold: Int)
}
```

> Use `JSONDecoder` with `.convertFromSnakeCase` key decoding strategy to map `product_name` → `productName` automatically.

### Kotlin (Android)

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
    val status: String,
    val volatility: Double,
    @SerialName("created_at") val createdAt: String
)

@Serializable
data class PricePoint(val price: Double, val timestamp: String)

@Serializable
data class Stats(
    @SerialName("viewer_count") val viewerCount: Int,
    @SerialName("total_purchases") val totalPurchases: Int,
    @SerialName("remaining_stock_pct") val remainingStockPct: Double,
    @SerialName("price_change_pct") val priceChangePct: Double,
    @SerialName("time_remaining_seconds") val timeRemainingSeconds: Int
)

@Serializable
data class BuyRequest(@SerialName("user_id") val userId: String, val quantity: Int)

@Serializable
data class BuyResponse(val purchase: Purchase, val message: String)

@Serializable
data class Purchase(
    val id: String,
    @SerialName("drop_id") val dropId: String,
    @SerialName("user_id") val userId: String,
    val price: Double,
    val quantity: Int,
    @SerialName("created_at") val createdAt: String
)

// WS events — parse "type" first, then deserialize specific class
@Serializable data class PriceUpdateEvent(val type: String, @SerialName("drop_id") val dropId: String, val price: Double, @SerialName("prev_price") val prevPrice: Double, val direction: String)
@Serializable data class StockUpdateEvent(val type: String, @SerialName("drop_id") val dropId: String, @SerialName("remaining_pct") val remainingPct: Double)
@Serializable data class PurchaseFeedEvent(val type: String, @SerialName("drop_id") val dropId: String, val price: Double, val message: String)
@Serializable data class ViewerCountEvent(val type: String, @SerialName("drop_id") val dropId: String, val count: Int)
@Serializable data class DropStatusEvent(val type: String, @SerialName("drop_id") val dropId: String, val status: String)
@Serializable data class DropEndedEvent(val type: String, @SerialName("drop_id") val dropId: String, @SerialName("final_price") val finalPrice: Double, @SerialName("total_sold") val totalSold: Int)
```

---

## Networking Examples

### Swift — URLSession (async/await)

```swift
struct TrendBorsaAPI {
    let baseURL = "http://localhost:8080"

    private var decoder: JSONDecoder {
        let d = JSONDecoder()
        d.keyDecodingStrategy = .convertFromSnakeCase
        return d
    }

    func getDrops(status: String? = nil) async throws -> [Drop] {
        var url = "\(baseURL)/api/drops"
        if let status { url += "?status=\(status)" }
        let (data, _) = try await URLSession.shared.data(from: URL(string: url)!)
        return try decoder.decode([Drop].self, from: data)
    }

    func getDrop(id: String) async throws -> Drop {
        let (data, _) = try await URLSession.shared.data(from: URL(string: "\(baseURL)/api/drops/\(id)")!)
        return try decoder.decode(Drop.self, from: data)
    }

    func getHistory(id: String) async throws -> [PricePoint] {
        let (data, _) = try await URLSession.shared.data(from: URL(string: "\(baseURL)/api/drops/\(id)/history")!)
        return try decoder.decode([PricePoint].self, from: data)
    }

    func getStats(id: String) async throws -> Stats {
        let (data, _) = try await URLSession.shared.data(from: URL(string: "\(baseURL)/api/drops/\(id)/stats")!)
        return try decoder.decode(Stats.self, from: data)
    }

    func buy(dropId: String, userId: String, quantity: Int) async throws -> BuyResponse {
        var request = URLRequest(url: URL(string: "\(baseURL)/api/drops/\(dropId)/buy")!)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let body = BuyRequest(userId: userId, quantity: quantity)
        request.httpBody = try JSONEncoder().encode(body)
        let (data, response) = try await URLSession.shared.data(for: request)
        guard (response as? HTTPURLResponse)?.statusCode == 200 else {
            let err = try JSONDecoder().decode(ErrorResponse.self, from: data)
            throw NSError(domain: "TrendBorsa", code: 0, userInfo: [NSLocalizedDescriptionKey: err.error])
        }
        return try decoder.decode(BuyResponse.self, from: data)
    }
}
```

### Swift — WebSocket

```swift
class PriceStreamClient {
    private var task: URLSessionWebSocketTask?

    func connect(dropId: String) -> AsyncStream<WSEvent> {
        AsyncStream { continuation in
            let url = URL(string: "ws://localhost:8080/ws/drops/\(dropId)")!
            task = URLSession.shared.webSocketTask(with: url)
            task?.resume()
            listen(continuation: continuation)
        }
    }

    private func listen(continuation: AsyncStream<WSEvent>.Continuation) {
        task?.receive { [weak self] result in
            switch result {
            case .success(.string(let text)):
                if let event = Self.parse(text) {
                    continuation.yield(event)
                }
                self?.listen(continuation: continuation)
            case .failure:
                continuation.finish()
            default:
                self?.listen(continuation: continuation)
            }
        }
    }

    func disconnect() {
        task?.cancel(with: .goingAway, reason: nil)
    }

    static func parse(_ text: String) -> WSEvent? {
        guard let data = text.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String else { return nil }
        switch type {
        case "PRICE_UPDATE":
            return .priceUpdate(price: json["price"] as? Double ?? 0,
                                prevPrice: json["prev_price"] as? Double ?? 0,
                                direction: json["direction"] as? String ?? "STABLE")
        case "STOCK_UPDATE":
            return .stockUpdate(remainingPct: json["remaining_pct"] as? Double ?? 0)
        case "PURCHASE_FEED":
            return .purchaseFeed(price: json["price"] as? Double ?? 0,
                                message: json["message"] as? String ?? "")
        case "VIEWER_COUNT":
            return .viewerCount(count: json["count"] as? Int ?? 0)
        case "DROP_STATUS":
            return .dropStatus(status: json["status"] as? String ?? "")
        case "DROP_ENDED":
            return .dropEnded(finalPrice: json["final_price"] as? Double ?? 0,
                              totalSold: json["total_sold"] as? Int ?? 0)
        default: return nil
        }
    }
}
```

### Kotlin — Retrofit

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
}
```

### Kotlin — WebSocket

```kotlin
class PriceStreamClient(private val client: OkHttpClient) {
    private val _events = MutableSharedFlow<WSEvent>()
    val events: SharedFlow<WSEvent> = _events

    fun connect(dropId: String) {
        val request = Request.Builder()
            .url("ws://10.0.2.2:8080/ws/drops/$dropId")
            .build()
        client.newWebSocket(request, object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                val json = JSONObject(text)
                val event = when (json.getString("type")) {
                    "PRICE_UPDATE" -> WSEvent.PriceUpdate(json.getDouble("price"), json.getDouble("prev_price"), json.getString("direction"))
                    "STOCK_UPDATE" -> WSEvent.StockUpdate(json.getDouble("remaining_pct"))
                    "PURCHASE_FEED" -> WSEvent.PurchaseFeed(json.getDouble("price"), json.getString("message"))
                    "VIEWER_COUNT" -> WSEvent.ViewerCount(json.getInt("count"))
                    "DROP_STATUS" -> WSEvent.DropStatus(json.getString("status"))
                    "DROP_ENDED" -> WSEvent.DropEnded(json.getDouble("final_price"), json.getInt("total_sold"))
                    else -> null
                }
                event?.let { _events.tryEmit(it) }
            }
        })
    }
}

sealed class WSEvent {
    data class PriceUpdate(val price: Double, val prevPrice: Double, val direction: String) : WSEvent()
    data class StockUpdate(val remainingPct: Double) : WSEvent()
    data class PurchaseFeed(val price: Double, val message: String) : WSEvent()
    data class ViewerCount(val count: Int) : WSEvent()
    data class DropStatus(val status: String) : WSEvent()
    data class DropEnded(val finalPrice: Double, val totalSold: Int) : WSEvent()
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `TICK_INTERVAL` | `10s` | Price recalculation interval |
| `DEFAULT_VOLATILITY` | `0.03` | Default price sensitivity |
| `CORS_ORIGIN` | `*` | CORS allowed origins |

---

## Price Algorithm

```
Every tick (default 10s), for each active drop:

if purchases_in_last_tick > 0:
    UP:   new = current + (purchases * volatility * current)
else:
    DOWN: new = current - (volatility * 0.5 * current)

noise = random(+-0.5%) * current
new = clamp(new + noise, floor_price, ceiling_price)
```

---

## Seed Data

| ID | Product | Start Price | Floor | Ceiling | Stock | Status |
|----|---------|------------|-------|---------|-------|--------|
| drop-1 | Sony WH-1000XM5 Kulaklık | 8.999 TL | 5.399 TL | 12.599 TL | 50 | active |
| drop-2 | iPhone 16 Pro Max Kılıf | 599 TL | 359 TL | 839 TL | 200 | upcoming (5m) |
| drop-3 | Nike Air Max 90 | 4.299 TL | 2.579 TL | 6.019 TL | 30 | active |
| drop-4 | Dyson V15 Süpürge | 24.999 TL | 14.999 TL | 34.999 TL | 15 | upcoming (10m) |
