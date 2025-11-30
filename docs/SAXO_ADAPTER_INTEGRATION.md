# Saxo Adapter Integration - Reference Implementation

This document shows how `fx-collector` demonstrates proper usage of the `saxo-adapter` library. Use this as a template for your own projects integrating with Saxo Bank.

## Overview

`fx-collector` is a **reference implementation** showing best practices for:

- Authentication and token management
- WebSocket connection lifecycle
- Price streaming subscriptions
- Graceful shutdown handling

## Key Integration Points

### 1. Service Initialization

```go
// internal/services/collector_service.go

func NewCollectorService(
    oauthConfig *oauth2.Config,       // Injected OAuth config
    provider string,                   // Injected provider name
    brokerBaseURL string,              // Injected API URL
    brokerWebsocketURL string,         // Injected WebSocket URL
    instruments *config.InstrumentConfig,
    spreadRecorder ports.SpreadRecorder,
    flushInterval time.Duration,
    logger *log.Logger,
) (*CollectorService, error) {

    // Create OAuth config map (following legacy pattern)
    providerConfigs := map[string]*oauth2.Config{
        provider: oauthConfig,
    }

    // Create auth client with injected config (dependency injection)
    tokenStorage := saxo.NewTokenStorage()
    authClient := saxo.NewSaxoAuthClient(
        providerConfigs,
        brokerBaseURL,
        brokerWebsocketURL,
        tokenStorage,
        saxo.SaxoSIM,
        logger,
    )

    brokerClient := saxo.NewSaxoBrokerClient(authClient, brokerBaseURL, logger)
    wsClient := websocket.NewSaxoWebSocketClient(authClient, brokerBaseURL, brokerWebsocketURL, logger)

    return &CollectorService{
        authClient:   authClient,
        brokerClient: brokerClient,
        wsClient:     wsClient,
        // ... other fields
    }, nil
}
```

**Key Pattern**: Use **dependency injection** - config is loaded early and passed to constructor, not read from environment at runtime.

### 2. Authentication Flow

```go
func (cs *CollectorService) Start() error {
    // Check if already authenticated (e.g., valid cached token)
    if !cs.authClient.IsAuthenticated() {
        cs.logger.Println("Not authenticated - attempting login...")
        if err := cs.authClient.Login(cs.ctx); err != nil {
            return fmt.Errorf("authentication failed: %w", err)
        }
        cs.logger.Println("Authentication successful")
    }
    
    // ... continue with WebSocket setup
}
```

**Key Pattern**: Always check `IsAuthenticated()` before calling `Login()` to avoid unnecessary re-authentication.

### 3. Token Refresh Management

```go
// Step 1: Create channels for WebSocket state tracking
wsStateChannel := make(chan bool, 1)
wsContextIDChannel := make(chan string, 1)

// Step 2: Register channels with WebSocket client
cs.wsClient.SetStateChannels(wsStateChannel, wsContextIDChannel)

// Step 3: Start token refresh goroutine (type assertion for Saxo-specific functionality)
if saxoAuth, ok := cs.authClient.(interface {
    StartTokenEarlyRefresh(ctx context.Context, wsConnected <-chan bool, wsContextID <-chan string)
}); ok {
    go saxoAuth.StartTokenEarlyRefresh(cs.ctx, wsStateChannel, wsContextIDChannel)
    cs.logger.Println("Token refresh manager started")
}
```

**Why This Matters**: 

- Saxo access tokens expire after **20 minutes**
- `StartTokenEarlyRefresh()` proactively refreshes tokens **before expiration**
- WebSocket state channels coordinate refresh with active connections
- Prevents connection interruption due to token expiry

### 4. WebSocket Connection

```go
cs.logger.Println("Connecting to Saxo WebSocket...")
if err := cs.wsClient.Connect(cs.ctx); err != nil {
    return fmt.Errorf("websocket connection failed: %w", err)
}
cs.logger.Println("WebSocket connected")
```

**Key Pattern**: Use context for cancellation control throughout the lifecycle.

### 5. Price Subscriptions

```go
tickers := cs.instruments.GetAllTickers() // ["EURUSD", "USDJPY", ...]
cs.logger.Printf("Subscribing to %d instruments", len(tickers))

if err := cs.wsClient.SubscribeToPrices(cs.ctx, tickers); err != nil {
    return fmt.Errorf("price subscription failed: %w", err)
}
cs.logger.Println("Price subscriptions established")
```

**Key Pattern**: Subscribe to all instruments in one call for efficiency.

### 6. Processing Price Updates

```go
func (cs *CollectorService) processPriceUpdates() {
    priceChannel := cs.wsClient.GetPriceUpdateChannel()
    
    for {
        select {
        case <-cs.ctx.Done():
            cs.logger.Println("Price processor stopping")
            return

        case priceUpdate, ok := <-priceChannel:
            if !ok {
                cs.logger.Println("Price channel closed")
                return
            }

            // Process the update
            priceData, err := cs.mapPriceUpdate(&priceUpdate)
            if err != nil {
                cs.logger.Printf("Error: %v", err)
                continue
            }

            // Do something with the data
            cs.spreadRecorder.Record(cs.ctx, priceData)
        }
    }
}
```

**Key Patterns**:

- Always check channel closed (`ok`) status
- Handle context cancellation for graceful shutdown
- Process errors without stopping the loop
- Use select for responsive cancellation

### 7. Mapping Generic Updates to Domain Models

```go
func (cs *CollectorService) mapPriceUpdate(update *saxo.PriceUpdate) (*domain.PriceData, error) {
    // Enrich generic price update with instrument metadata
    instrument, err := cs.instruments.GetInstrumentByTicker(update.Ticker)
    if err != nil {
        return nil, fmt.Errorf("instrument not found: %w", err)
    }

    return &domain.PriceData{
        Timestamp: update.Timestamp,
        Uic:       instrument.Uic,        // From config
        Ticker:    update.Ticker,         // From update
        AssetType: instrument.AssetType,  // From config
        Bid:       update.Bid,            // From update
        Ask:       update.Ask,            // From update
        Decimals:  instrument.Decimals,   // From config
    }, nil
}
```

**Key Pattern**: Combine generic `saxo.PriceUpdate` with configuration metadata to create rich domain objects.

### 8. Graceful Shutdown

```go
func (cs *CollectorService) Stop() error {
    cs.logger.Println("Stopping FX Collector Service...")

    // Step 1: Cancel context (stops all goroutines)
    cs.cancel()

    // Step 2: Perform final data flush
    cs.logger.Println("Performing final flush...")
    if err := cs.spreadRecorder.Flush(cs.ctx); err != nil {
        cs.logger.Printf("Final flush error: %v", err)
    }

    // Step 3: Close WebSocket connection
    cs.logger.Println("Closing WebSocket connection...")
    if err := cs.wsClient.Close(); err != nil {
        cs.logger.Printf("WebSocket close error: %v", err)
    }

    // Step 4: Close other resources
    cs.logger.Println("Closing spread recorder...")
    if err := cs.spreadRecorder.Close(); err != nil {
        cs.logger.Printf("Recorder close error: %v", err)
    }

    cs.logger.Println("FX Collector Service stopped")
    return nil
}
```

**Key Pattern**: Always close resources in reverse order of initialization, with error logging (not failure).

## Environment Configuration

Required environment variables (`.env` file):

```bash
# Saxo Bank OAuth Configuration
BROKER_CLIENT_ID=your_client_id
BROKER_CLIENT_SECRET=your_secret
AUTH_URL=https://sim.logonvalidation.net/authorize
TOKEN_URL=https://sim.logonvalidation.net/token
PROVIDER=saxo

# Saxo Bank API URLs
BROKER_BASE_URL=https://gateway.saxobank.com/sim/openapi
BROKER_WEBSOCKET_URL=wss://streaming.saxobank.com/sim/openapi/streamingws/connect

# Application-specific
INSTRUMENTS_PATH=config/instruments.json
SPREAD_RECORDING_DIR=data/spreads
SPREAD_FLUSH_INTERVAL=30s
```

**Configuration Loading Pattern** (following legacy main.go):

```go
// Load config early with fail-fast validation
envConfig, err := config.LoadEnvConfig()
if err != nil {
    return fmt.Errorf("configuration error: %w", err)
}

// Inject config into service constructor
service, err := services.NewCollectorService(
    envConfig.OAuth.ToOAuth2Config(),  // Injected!
    envConfig.Provider,                // Injected!
    envConfig.BrokerBaseURL,           // Injected!
    envConfig.BrokerWebsocketURL,      // Injected!
    instruments,
    recorder,
    envConfig.FlushInterval,
    logger,
)
```

## Complete Lifecycle Example

```go
func main() {
    if err := run(); err != nil {
        log.Fatalf("Application error: %v", err)
    }
}

func run() error {
    logger := log.New(os.Stdout, "[FX-COLLECTOR] ", log.LstdFlags)

    // 1. Load environment config with fail-fast validation
    envConfig, err := config.LoadEnvConfig()
    if err != nil {
        return fmt.Errorf("configuration error: %w", err)
    }

    // 2. Load instruments
    instruments, err := config.LoadInstruments(envConfig.InstrumentsPath)
    if err != nil {
        return fmt.Errorf("failed to load instruments: %w", err)
    }

    // 3. Create recorder
    recorder := storage.NewCSVSpreadRecorder(envConfig.SpreadRecordingDir)

    // 4. Create service with injected config
    service, err := services.NewCollectorService(
        envConfig.OAuth.ToOAuth2Config(),
        envConfig.Provider,
        envConfig.BrokerBaseURL,
        envConfig.BrokerWebsocketURL,
        instruments,
        recorder,
        envConfig.FlushInterval,
        logger,
    )
    if err != nil {
        return fmt.Errorf("failed to create service: %w", err)
    }

    // 5. Start service (authenticates, connects WebSocket, subscribes)
    if err := service.Start(); err != nil {
        return fmt.Errorf("failed to start service: %w", err)
    }

    // 6. Run until interrupted
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    // 7. Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() { done <- service.Stop() }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return fmt.Errorf("shutdown timeout")
    }
}
```

## Common Patterns Summary

| Pattern | Implementation |
|---------|---------------|
| **Early Config Loading** | `config.LoadEnvConfig()` with fail-fast validation |
| **Dependency Injection** | OAuth config injected into service constructor |
| **run() Pattern** | `main()` calls `run()` for testability and error propagation |
| **Service Creation** | `NewCollectorService(oauthConfig, provider, baseURL, wsURL, ...)` |
| **Authentication Check** | `authClient.IsAuthenticated()` before `Login()` |
| **Token Refresh** | `StartTokenEarlyRefresh()` with WebSocket state channels |
| **WebSocket Connect** | `wsClient.Connect(ctx)` |
| **Price Subscription** | `wsClient.SubscribeToPrices(ctx, tickers)` |
| **Price Consumption** | `wsClient.GetPriceUpdateChannel()` with select loop |
| **Graceful Shutdown** | Context cancellation → flush → close WebSocket → close resources |

## Testing Your Integration

```bash
# 1. Build the collector
go build ./cmd/collector

# 2. Configure .env with your Saxo SIM credentials
cp .env.example .env
# Edit .env with real credentials

# 3. Run the collector
./collector

# Expected output:
# [FX-COLLECTOR] === FX Collector Starting ===
# [FX-COLLECTOR] Loading instruments from: config/instruments.json
# [FX-COLLECTOR] Loaded 17 instruments
# [FX-COLLECTOR] Authentication successful
# [FX-COLLECTOR] Token refresh manager started
# [FX-COLLECTOR] WebSocket connected
# [FX-COLLECTOR] Subscribing to 17 instruments
# [FX-COLLECTOR] Price subscriptions established
# [FX-COLLECTOR] Starting price update processor...
# [FX-COLLECTOR] Processed 100 price updates
# [FX-COLLECTOR] Processed 200 price updates
# ...
```

## Next Steps

1. **Review Source Code**: 
   - `internal/services/collector_service.go` - Complete integration example
   - `cmd/collector/main.go` - Application lifecycle and configuration

2. **Adapt for Your Use Case**:
   - Replace `SpreadRecorder` with your domain logic
   - Add order placement using `brokerClient` (not shown in fx-collector)
   - Implement portfolio monitoring or other broker operations

3. **Explore saxo-adapter**:
   - See `../saxo-adapter/adapter/interfaces.go` for full API
   - Check `../saxo-adapter/examples/` for other usage patterns

## Questions or Issues?

- **fx-collector**: Focus on price streaming and data recording
- **saxo-adapter**: Generic broker interface library
- **pivot-web2**: Full trading system using similar patterns

This reference implementation demonstrates production-ready patterns for 24/7 operation with Saxo Bank's API.
