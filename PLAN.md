# Stock Market Flash Sale - Technical Plan

## Concept

A time-limited (15-20 min) flash sale where product price fluctuates like a stock market based on real-time demand. Users see a live price chart, stock progress bar, and can "lock in" the current price to buy.

**Key mechanics:**
- Price rises when more people buy, decreases when fewer buy
- Stock progress bar shows remaining inventory (no exact numbers)
- Price lock: system fixes the price for 30s when user decides to buy
- Per-user purchase limit to prevent abuse

---

## System Architecture

```
┌─────────────────────────────────────────────────────┐
│              Go Backend (REST + WebSocket)            │
│                                                       │
│  ├── GET  /api/session          Active session info    │
│  ├── POST /api/session/join     Join & get ticket      │
│  ├── POST /api/lock-price       Lock price (30s)       │
│  ├── POST /api/purchase         Confirm buy            │
│  └── WS   /ws/price-stream      Live price+stock       │
│                                                       │
│  Price Engine: goroutine recalculating every 2-3s      │
│  Storage: In-memory (sync.RWMutex)                     │
└──────────────────────┬────────────────────────────────┘
                       │
              WebSocket + REST
                       │
┌──────────────────────┴────────────────────────────────┐
│           Android App (Jetpack Compose)                │
│                                                       │
│  flashauction/                                        │
│  ├── data/                                            │
│  │   ├── model/         DTOs, domain models            │
│  │   ├── remote/        Retrofit + OkHttp WS           │
│  │   └── repository/    FlashAuctionRepository         │
│  ├── domain/                                          │
│  │   └── usecase/       JoinSession, LockPrice, Buy    │
│  ├── presentation/                                    │
│  │   ├── FlashAuctionViewModel.kt                     │
│  │   └── screen/                                      │
│  │       ├── FlashAuctionScreen.kt                    │
│  │       ├── PriceChartComposable.kt                  │
│  │       ├── StockProgressBar.kt                      │
│  │       ├── PriceLockSheet.kt                        │
│  │       └── CountdownTimer.kt                        │
│  └── di/                Hilt modules                   │
└───────────────────────────────────────────────────────┘
```

---

## Part 1: Go Backend

### Project Structure

```
server/
├── main.go
├── go.mod
├── handler/
│   ├── session.go          REST handlers
│   └── websocket.go        WS upgrade + broadcast
├── engine/
│   └── price_engine.go     Price calculation goroutine
├── model/
│   └── models.go           Shared structs
└── store/
    └── store.go            In-memory state
```

### Price Engine (`engine/price_engine.go`)

Core goroutine that recalculates price every 2 seconds based on demand:

```go
type PriceEngine struct {
    mu                sync.RWMutex
    basePrice         float64
    currentPrice      float64
    minPrice          float64  // basePrice * 0.7
    maxPrice          float64  // basePrice * 1.5
    totalStock        int
    remainingStock    int
    priceHistory      []PricePoint
    purchasesInWindow int
    subscribers       map[string]chan PriceUpdate
}

func (e *PriceEngine) Run(ctx context.Context) {
    ticker := time.NewTicker(2 * time.Second)
    for {
        select {
        case <-ticker.C:
            e.recalculate()
            e.broadcast()
        case <-ctx.Done():
            return
        }
    }
}
```

**Price Algorithm:**

```
newPrice = basePrice * (1 + demandFactor)

demandFactor = (purchasesInLastInterval / expectedPurchaseRate) - 1
             * volatilityMultiplier

Constraints:
  minPrice = basePrice * 0.7   (30% below base)
  maxPrice = basePrice * 1.5   (50% above base)
  priceChangeInterval = 2-3 seconds
```

### WebSocket Handler

Pushes price updates to all connected clients every tick:

```json
{
    "currentPrice": 149.90,
    "previousPrice": 147.50,
    "trend": "up",
    "stockPercentage": 0.65,
    "remainingSeconds": 842,
    "priceHistory": [{"price": 145.0, "timestamp": 1715100000}, ...]
}
```

### REST Endpoints

| Endpoint | Method | Body | Response | Logic |
|----------|--------|------|----------|-------|
| `/api/session` | GET | — | `FlashAuctionSession` | Return active session info |
| `/api/session/join` | POST | `{userId}` | `{ticket, session}` | Register user, return WS ticket |
| `/api/lock-price` | POST | `{userId, quantity}` | `{lockId, lockedPrice, expiresAt}` | Fix price for 30s, reserve stock |
| `/api/purchase` | POST | `{lockId}` | `{success, orderSummary}` | Validate lock, confirm purchase |

### Price Lock Logic

```go
func (s *Store) LockPrice(userId string, qty int) (*PurchaseLock, error) {
    // 1. Check user hasn't exceeded max purchase limit
    // 2. Check enough stock remaining
    // 3. Tentatively reserve stock
    // 4. Create lock with 30s TTL
    // 5. Goroutine to auto-release stock if not confirmed within TTL
    return lock, nil
}
```

---

## Part 2: Android App (Jetpack Compose)

### Models

```kotlin
data class FlashAuctionSession(
    val id: String,
    val productName: String,
    val productImageUrl: String,
    val basePrice: Double,
    val currentPrice: Double,
    val minPrice: Double,
    val maxPrice: Double,
    val stockPercentage: Double,   // 0.0 - 1.0
    val sessionEndTime: Long,      // epoch millis
    val maxPurchasePerUser: Int,
    val status: SessionStatus
)

enum class SessionStatus { UPCOMING, ACTIVE, ENDED, SOLD_OUT }
enum class PriceTrend { UP, DOWN, STABLE }

data class PricePoint(val price: Double, val timestamp: Long)

data class PriceUpdate(
    val currentPrice: Double,
    val previousPrice: Double,
    val trend: PriceTrend,
    val stockPercentage: Double,
    val remainingSeconds: Int,
    val priceHistory: List<PricePoint>
)

data class PurchaseLock(
    val lockId: String,
    val lockedPrice: Double,
    val expiresAt: Long,
    val quantity: Int
)
```

### WebSocket Client

```kotlin
class PriceStreamClient(private val okHttpClient: OkHttpClient) {
    private val _priceUpdates = MutableSharedFlow<PriceUpdate>()
    val priceUpdates: SharedFlow<PriceUpdate> = _priceUpdates

    fun connect(sessionId: String) {
        val request = Request.Builder()
            .url("ws://SERVER/ws/price-stream?session=$sessionId")
            .build()
        okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onMessage(webSocket: WebSocket, text: String) {
                val update = Json.decodeFromString<PriceUpdate>(text)
                _priceUpdates.tryEmit(update)
            }
        })
    }
}
```

### ViewModel

```kotlin
@HiltViewModel
class FlashAuctionViewModel @Inject constructor(
    private val repository: FlashAuctionRepository,
    private val priceStream: PriceStreamClient
) : ViewModel() {

    var uiState by mutableStateOf<UiState>(UiState.Loading)
        private set
    var currentPrice by mutableStateOf(0.0)
    var priceTrend by mutableStateOf(PriceTrend.STABLE)
    var stockPercentage by mutableStateOf(1.0)
    var priceHistory = mutableStateListOf<PricePoint>()
    var remainingSeconds by mutableIntStateOf(0)
    var priceLock by mutableStateOf<PurchaseLock?>(null)

    init {
        viewModelScope.launch {
            priceStream.priceUpdates.collect { update ->
                currentPrice = update.currentPrice
                priceTrend = update.trend
                stockPercentage = update.stockPercentage
                remainingSeconds = update.remainingSeconds
                priceHistory.clear()
                priceHistory.addAll(update.priceHistory)
            }
        }
    }

    fun lockPrice(quantity: Int) { /* POST /lock-price */ }
    fun confirmPurchase() { /* POST /purchase */ }
}
```

### Main Screen Layout

```
┌─────────────────────────────┐
│  [Product Image] [Name]     │
│  Base: ₺120.00              │
├─────────────────────────────┤
│        ₺149.90  ▲           │  <- Big price, colored trend
│   +24.9% from base          │
├─────────────────────────────┤
│  ╭─────────────────╮        │
│  │  Price Chart     │        │  <- Line chart (last 5 min)
│  │  (Canvas / Vico) │        │
│  ╰─────────────────╯        │
├─────────────────────────────┤
│  Stock: ████████░░░░  65%   │  <- Gradient progress bar
├─────────────────────────────┤
│       ⏱ 14:02 remaining     │  <- Countdown
├─────────────────────────────┤
│  ┌─────────────────────┐    │
│  │   BUY NOW @ ₺149.90 │    │  <- Animated buy button
│  └─────────────────────┘    │
└─────────────────────────────┘
```

### Stock Progress Bar

```kotlin
@Composable
fun StockProgressBar(percentage: Float) {
    val animatedProgress by animateFloatAsState(targetValue = percentage)
    val color = when {
        percentage > 0.5f -> Color.Green
        percentage > 0.2f -> Color.Yellow
        else -> Color.Red
    }
    LinearProgressIndicator(
        progress = animatedProgress,
        color = color,
        modifier = Modifier
            .fillMaxWidth()
            .height(12.dp)
            .clip(RoundedCornerShape(6.dp))
    )
}
```

### Price Chart

Using Compose Canvas (no external dependency):

```kotlin
@Composable
fun PriceChart(points: List<PricePoint>, modifier: Modifier) {
    Canvas(modifier = modifier) {
        // Draw line path through price points
        // Area fill below with gradient
        // Animate new points appearing
    }
}
```

### Purchase Flow

```
User taps "BUY NOW"
       |
       v
POST /lock-price -> returns PurchaseLock (price fixed for 30s)
       |
       v
Confirmation bottom sheet:
  - Locked price with countdown ("Price locked for 28s")
  - Quantity selector (1 to maxPerUser)
  - "Confirm Purchase" button
       |
       v
POST /purchase -> success  -> navigate to success screen
               -> expired  -> "Price lock expired, try again"
               -> sold out -> show sold out state
```

---

## Part 3: Development Phases

| Phase | Task | Priority |
|-------|------|----------|
| **1** | Go: Price engine + in-memory store + `/api/session` | Do first |
| **2** | Go: WebSocket handler broadcasting price ticks | Do first |
| **3** | Android: Models + WebSocket client + ViewModel | Do second |
| **4** | Android: Main screen UI (price, chart, stock bar, timer) | Do second |
| **5** | Go: `/lock-price` + `/purchase` endpoints with TTL | Do third |
| **6** | Android: Buy flow (lock -> confirm sheet -> result) | Do third |
| **7** | Polish: Animations, haptics, error states | If time |

> Phases 1-2 (Go) and 3-4 (Android) can be developed in parallel.

---

## Key Technical Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Real-time transport | WebSocket | gorilla/websocket trivial in Go, OkHttp has built-in WS |
| Price recalculation | Server-side goroutine | Single source of truth, no client manipulation |
| Storage | In-memory with `sync.RWMutex` | Hackathon speed, no DB setup |
| Chart library | Compose Canvas or Vico | Canvas = zero deps, Vico = feature-rich |
| DI | Hilt | Standard Android, minimal boilerplate |
| Serialization | kotlinx.serialization | Lightweight, Compose-friendly |

---

## UX Details

- **Price color**: Green when below base price, red when above, animated transitions
- **Trend arrow**: Animated up/down/stable indicator next to current price
- **Haptic feedback**: Light impact on significant price changes
- **Stock bar gradient**: Green (plenty) -> Yellow (going fast) -> Red (almost gone)
- **Countdown**: MM:SS format, urgency animation when < 1 minute remaining
- **Price lock countdown**: Circular progress around the buy button during lock period
