# WebSocket Integration TL;DR

## How fx-collector consumes saxo-adapter WebSocket for real-time FX price streaming**

## Quick Start

```go
// 1. Create auth client
authClient, _ := saxo.CreateSaxoAuthClient(logger)

// 2. Create WebSocket client
wsClient := websocket.NewSaxoWebSocketClient(
    authClient,
    authClient.GetBaseURL(),
    authClient.GetWebSocketURL(),
    logger,
)

// 3. Connect and subscribe
wsClient.Connect(ctx)
wsClient.SubscribeToPrices(ctx, []string{"EURUSD", "GBPUSD"})

// 4. Consume price updates
priceChannel := wsClient.GetPriceUpdateChannel()
for priceUpdate := range priceChannel {
    // Process priceUpdate.Ticker, priceUpdate.Bid, priceUpdate.Ask
}
```

## Architecture

saxo-adapter/websocket
    ↓
WebSocket Connection → Read Goroutine → Message Parser
    ↓
PriceUpdate Channel
    ↓
fx-collector → processPriceUpdates() → SpreadRecorder

## Key Components

| Component | File | Purpose |
|-----------|------|---------|
| **WebSocket Client** | `saxo-adapter/adapter/websocket/` | Connection lifecycle, subscriptions |
| **Auth Client** | `saxo-adapter/adapter/oauth.go` | OAuth2 tokens, auto-refresh |
| **Collector Service** | `internal/services/collector_service.go` | Business logic orchestration |
| **Spread Recorder** | `internal/adapters/storage/` | CSV data persistence |

## Integration Pattern

### 1. Setup (in `main.go`)

```go
authClient := saxo.CreateSaxoAuthClient(logger)
brokerClient := saxo.CreateBrokerServices(authClient, logger)
collectorService := services.NewCollectorService(authClient, brokerClient, ...)
```

### 2. Start Flow (in `collector_service.go`)

```go
func (cs *CollectorService) Start() error {
    // Authenticate
    cs.authClient.Login(ctx)
    
    // Setup token refresh monitoring
    wsStateChannel := make(chan bool, 1)
    wsContextIDChannel := make(chan string, 1)
    cs.wsClient.SetStateChannels(wsStateChannel, wsContextIDChannel)
    go saxoAuth.StartTokenEarlyRefresh(ctx, wsStateChannel, wsContextIDChannel)
    
    // Connect WebSocket
    cs.wsClient.Connect(ctx)
    
    // Register instruments (ticker→UIC mapping)
    saxoWS.RegisterInstruments(saxoInstruments)
    
    // Subscribe to prices
    cs.wsClient.SubscribeToPrices(ctx, tickers)
    
    // Start processing
    go cs.processPriceUpdates()
}
```

### 3. Consume Prices

```go
func (cs *CollectorService) processPriceUpdates() {
    priceChannel := cs.wsClient.GetPriceUpdateChannel()
    
    for priceUpdate := range priceChannel {
        priceData := cs.mapPriceUpdate(&priceUpdate)
        cs.spreadRecorder.Record(ctx, priceData)
    }
}
```

### 4. Shutdown

```go
func (cs *CollectorService) Stop() error {
    cs.cancel()                      // Stop context
    cs.spreadRecorder.Flush(ctx)     // Flush buffered data
    cs.wsClient.Close()              // Close WebSocket
}
```

## Critical Details

### Token Management

- **Access tokens expire in 20 minutes**
- `StartTokenEarlyRefresh()` auto-refreshes before expiry
- Monitors WebSocket state via channels to trigger refresh

### Instrument Registration

```go
// MUST call before SubscribeToPrices()
saxoWS.RegisterInstruments([]*saxo.Instrument{
    {Ticker: "EURUSD", Identifier: 21, AssetType: "FxSpot"},
})
```

Maps ticker strings to UICs for WebSocket message routing.

### Price Update Structure

```go
type PriceUpdate struct {
    Ticker    string
    Uic       int
    Bid       float64
    Ask       float64
    Timestamp time.Time
}
```

## Dependencies

```go
// go.mod
replace github.com/bjoelf/saxo-adapter => ../saxo-adapter

require (
    github.com/bjoelf/saxo-adapter v0.0.0
    github.com/gorilla/websocket v1.5.0
)
```

## Configuration

```bash
# .env file
BROKER_BASE_URL=https://gateway.saxobank.com/sim/openapi
BROKER_WEBSOCKET_URL=wss://sim-streaming.saxobank.com/sim/oapi/streaming/ws
BROKER_CLIENT_ID=your_client_id
BROKER_CLIENT_SECRET=your_secret
TOKEN_URL=https://sim.logonvalidation.net/token
AUTH_URL=https://sim.logonvalidation.net/authorize
```

## Data Flow

1. **Saxo WebSocket** sends JSON messages
2. **Message Parser** converts to `PriceUpdate` struct
3. **Price Channel** delivers updates to consumer
4. **Collector Service** maps to domain model
5. **Spread Recorder** persists to CSV

## Error Handling

- **Connection failures**: Auto-reconnect with exponential backoff (max 10 attempts)
- **Token expiry**: Auto-refresh before expiration
- **Subscription failures**: Logged and retried on reconnect
- **Message parse errors**: Logged, skipped, processing continues

## Testing

```go
// Use mock WebSocket server for testing
mockServer := adapter.NewMockSaxoServer()
authClient := saxo.NewSaxoAuthClient(/* ... */, mockServer.URL)
wsClient := websocket.NewSaxoWebSocketClient(authClient, ...)
```

## Key Files

fx-collector/
├── cmd/collector/main.go                    # Entry point, config loading
├── internal/
│   ├── services/collector_service.go        # WebSocket consumption logic
│   ├── adapters/storage/csv_spread_recorder.go  # Data persistence
│   └── domain/price_data.go                 # Domain models
└── docs/
    ├── SAXO_ADAPTER_INTEGRATION.md          # Full integration guide
    └── WEBSOCKET_INTEGRATION_TLDR.md        # This file

saxo-adapter/
├── adapter/
│   ├── interfaces.go                        # WebSocketClient interface
│   ├── oauth.go                             # AuthClient implementation
│   └── websocket/
│       ├── connection_manager.go            # Connection lifecycle
│       ├── message_handler.go               # Message processing
│       └── message_parser.go                # JSON parsing

## Common Patterns

### Dependency Injection

```go
// Good: Inject dependencies
func NewService(authClient AuthClient, wsClient WebSocketClient) *Service

// Bad: Create dependencies internally
func NewService() *Service {
    authClient := saxo.NewSaxoAuthClient(...)  // ❌ Hard to test
}
```

### Interface Usage

```go
// Use interfaces, not concrete types
type CollectorService struct {
    wsClient saxo.WebSocketClient  // ✅ Interface
}

// Not this
type CollectorService struct {
    wsClient *websocket.SaxoWebSocketClient  // ❌ Concrete type
}
```

### Channel-based Communication

```go
// WebSocket client provides channels
priceChannel := wsClient.GetPriceUpdateChannel()

// Consumer reads from channel
for update := range priceChannel {
    process(update)
}
```

## Reference Implementation

See `fx-collector` as the **canonical example** of saxo-adapter WebSocket integration:

- Clean architecture (ports/adapters pattern)
- Proper dependency injection
- Interface-based design
- Graceful shutdown handling
- Production-ready error handling

## Quick Reference

| Action | Method |
|--------|--------|
| Connect | `wsClient.Connect(ctx)` |
| Subscribe prices | `wsClient.SubscribeToPrices(ctx, tickers)` |
| Subscribe orders | `wsClient.SubscribeToOrders(ctx)` |
| Subscribe portfolio | `wsClient.SubscribeToPortfolio(ctx)` |
| Get price updates | `wsClient.GetPriceUpdateChannel()` |
| Get order updates | `wsClient.GetOrderUpdateChannel()` |
| Close connection | `wsClient.Close()` |
| Check auth | `authClient.IsAuthenticated()` |
| Login | `authClient.Login(ctx)` |

---

**Last Updated**: 30 November 2025  
**Version**: Based on saxo-adapter v0.0.0 and fx-collector integration
