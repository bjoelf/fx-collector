# FX-Collector Project Guide

**Project Start Date:** November 26, 2025  
**Status:** 🚧 In Progress  
**Current Phase:** Session 2 Complete - Integration Testing Next  
**Last Updated:** Session 2 - Saxo Adapter Integration Complete

## Executive Summary

This project extracts FX spread recording functionality from `pivot-web2` into a standalone service called `fx-collector`. The primary goals are:

1. **Separation of Concerns**: Decouple price collection from trading logic
2. **Broker Abstraction Testing**: Long-term test the `saxo-adapter` library
3. **Stable WebSocket Validation**: Verify connection stability over extended periods
4. **Clean Architecture**: Use `saxo-adapter` generic API instead of broker-specific code

## Project Context

### Why This Refactoring?

**Current State (pivot-web2):**
- Monolithic architecture with spread recording embedded
- Direct Saxo-specific implementation mixed with trading logic
- Spread recording is ancillary to core trading functionality
- Testing broker adapter requires full trading platform deployment

**Target State (fx-collector):**
- Standalone service focused solely on price collection
- Uses generic `saxo-adapter` API (broker-agnostic)
- Lightweight deployment for long-term stability testing
- Prepares for eventual pivot-web2 migration to saxo-adapter

**Strategic Vision:**
```
Step 1: Extract fx-collector → Use saxo-adapter (THIS PROJECT)
Step 2: Test saxo-adapter stability via fx-collector (6-12 months)
Step 3: Migrate pivot-web2 to saxo-adapter (Future project)
```

## Architecture Overview

### Source Components (pivot-web2)

**Domain Objects:**
- `internal/domain/spread_data.go` - PriceData struct (bid/ask/spread)

**Port Interfaces:**
- `internal/ports/spread_recorder.go` - SpreadRecorder interface

**Adapter Implementation:**
- `internal/adapters/storage/csv_spread_recorder.go` - CSV file writer
- `internal/adapters/storage/csv_spread_recorder_test.go` - Unit tests

**Data Storage:**
- `data/spreads/YYYYMMDD/TICKER_HH.csv` - Hourly CSV files
- Example: `data/spreads/20251119/EURUSD_14.csv`

**Configuration (from SPREAD_RECORDING_DEPLOYMENT.md):**
- `ENABLE_SPREAD_RECORDING=true/false` - Master switch
- `SPREAD_RECORDING_DIR=data/spreads` - Output directory
- `SPREAD_FLUSH_INTERVAL=60` - Flush interval in seconds

### Target Architecture (fx-collector)

```
fx-collector/
├── cmd/
│   └── collector/
│       └── main.go              # Entry point with graceful shutdown
├── internal/
│   ├── domain/
│   │   └── price_data.go        # PriceData domain object (from pivot-web2)
│   ├── ports/
│   │   └── spread_recorder.go   # SpreadRecorder interface (from pivot-web2)
│   ├── adapters/
│   │   └── storage/
│   │       ├── csv_spread_recorder.go      # CSV implementation (from pivot-web2)
│   │       └── csv_spread_recorder_test.go # Unit tests
│   └── services/
│       └── collector_service.go # Orchestration: saxo-adapter → recorder
├── config/
│   └── instruments.json         # FX pairs configuration
├── data/
│   └── spreads/                 # CSV output (same structure as pivot-web2)
├── scripts/
│   └── deploy.sh                # Deployment script
├── .env.example                 # Configuration template
├── go.mod                       # Module with saxo-adapter dependency
├── README.md                    # User documentation
└── PROJECT_GUIDE.md            # This file
```

### Saxo-Adapter Integration

**Key Interfaces (from saxo-adapter/adapter/interfaces.go):**

```go
// WebSocketClient - Real-time price streaming
type WebSocketClient interface {
    Connect(ctx context.Context) error
    SubscribeToPrices(ctx context.Context, instruments []string) error
    GetPriceUpdateChannel() <-chan PriceUpdate
    Close() error
}

// PriceUpdate - Streaming price data
type PriceUpdate struct {
    Ticker    string
    Bid       float64
    Ask       float64
    Mid       float64
    Timestamp time.Time
}

// AuthClient - OAuth2 authentication
type AuthClient interface {
    Login(ctx context.Context) error
    GetHTTPClient(ctx context.Context) (*http.Client, error)
    StartTokenRefresh(ctx context.Context, wsConnected <-chan bool, wsContextID <-chan string)
}
```

**Mapping Strategy:**
```
saxo.PriceUpdate → domain.PriceData
├── Ticker    → Ticker
├── Bid       → Bid
├── Ask       → Ask
├── Timestamp → Timestamp
├── (calculate) → Spread = Ask - Bid
└── (lookup)  → Uic, AssetType, Decimals from instruments.json
```

## Detailed Implementation Sessions

### Session 1: Foundation Setup ✅ COMPLETED
**Duration:** ~45 minutes  
**Completed:** November 26, 2025  
**Dependencies:** None

#### Tasks Completed:
1. ✅ **Initialize Go Module**
   - [x] Create `fx-collector/go.mod`
   - [x] Add dependency: `github.com/bjoelf/saxo-adapter`
   - [x] Set Go version: 1.21+

2. ✅ **Create Directory Structure**
   - [x] `cmd/collector/`
   - [x] `internal/domain/`
   - [x] `internal/ports/`
   - [x] `internal/adapters/storage/`
   - [x] `internal/services/`
   - [x] `config/`
   - [x] `data/spreads/`
   - [x] `scripts/`

3. ✅ **Port Domain Objects**
   - [x] Copy `spread_data.go` from pivot-web2
   - [x] Update package to `domain`
   - [x] Verify PriceData struct matches requirements

4. ✅ **Port Interfaces**
   - [x] Copy `spread_recorder.go` from pivot-web2
   - [x] Update package to `ports`
   - [x] Verify interface methods

5. ✅ **Port CSV Recorder**
   - [x] Copy `csv_spread_recorder.go` from pivot-web2
   - [x] Update imports to fx-collector paths
   - [x] Copy unit tests
   - [x] Run tests: `go test ./internal/adapters/storage/...`

6. ✅ **Configuration Files**
   - [x] Create `.env.example` with Saxo credentials template
   - [x] Create `config/instruments.json` with 17 FX pairs from pivot-web2/data/fx.json
   - [x] Create comprehensive `README.md`
   - [x] Create `.gitignore`

#### Deliverables:
- ✅ Working Go module with saxo-adapter dependency
- ✅ Spread recording code ported and tested
- ✅ Configuration framework ready
- ✅ All tests pass
- ✅ All code compiles

#### Validation Results:
```bash
$ go test ./...
ok   github.com/bjoelf/fx-collector/internal/adapters/storage  0.004s

$ go build ./...
# Success - all packages build without errors
```

---

### Session 2: Saxo-Adapter Integration ⏳ NEXT
**Duration:** ~45-60 minutes  
**Dependencies:** Session 1 complete

#### Tasks:

1. **Instrument Configuration Loader**
   - [ ] Create `internal/config/loader.go`
   - [ ] Parse `config/instruments.json`
   - [ ] Map ticker → Uic/AssetType/Decimals
   - [ ] Validate instrument data completeness

2. **Collector Service Implementation**
   - [ ] Create `internal/services/collector_service.go`
   - [ ] Initialize saxo-adapter clients: `saxo.CreateBrokerServices()`
   - [ ] Map `saxo.PriceUpdate` → `domain.PriceData`
   - [ ] Connect price channel → SpreadRecorder
   - [ ] Implement graceful shutdown

3. **Main Entry Point**
   - [ ] Create `cmd/collector/main.go`
   - [ ] Load environment variables
   - [ ] Initialize logging
   - [ ] Create collector service
   - [ ] Handle OS signals (SIGINT, SIGTERM)

4. **WebSocket Lifecycle**
   - [ ] Connection establishment
   - [ ] Price subscription for all instruments
   - [ ] Channel consumption loop
   - [ ] Reconnection handling (delegate to saxo-adapter)

5. **Integration Testing**
   - [ ] Create `tests/integration/` directory
   - [ ] Test with SIM environment
   - [ ] Verify CSV file creation
   - [ ] Validate price data accuracy

#### Deliverables:
- [ ] Working WebSocket price subscription
- [ ] Live spread data recording to CSV
- [ ] Clean separation: no Saxo-specific code in business logic

#### Validation:
```bash
# Set .env with SIM credentials
export SAXO_ENVIRONMENT=sim
export SIM_CLIENT_ID=...
export SIM_CLIENT_SECRET=...

# Run collector
go run ./cmd/collector

# Verify output
ls -la data/spreads/$(date +%Y%m%d)/
tail -f data/spreads/$(date +%Y%m%d)/EURUSD_$(date +%H).csv
```

---

### Session 3: Production Readiness ⏸️ PENDING
**Duration:** ~30-45 minutes  
**Dependencies:** Session 2 complete

#### Tasks:

1. **Connection Timing** *(Optional - can start simple)*
   - [ ] Evaluate: Run 24/5 or business hours only?
   - [ ] If market-driven: Implement schedule lookup
   - [ ] If simple: Connect on startup, disconnect on shutdown

2. **Health Monitoring**
   - [ ] Add metrics: messages received, files written, uptime
   - [ ] Create health check endpoint (HTTP server)
   - [ ] Add structured logging (JSON format)
   - [ ] Monitor WebSocket connection state

3. **Deployment Preparation**
   - [ ] Create systemd service file: `scripts/fx-collector.service`
   - [ ] Create deployment script: `scripts/deploy.sh`
   - [ ] Add logrotate configuration
   - [ ] Document VM setup requirements

4. **Error Handling & Resilience**
   - [ ] Graceful degradation on recorder errors
   - [ ] CSV write failure handling
   - [ ] Disk space monitoring
   - [ ] Alert on prolonged disconnection

5. **Documentation**
   - [ ] Update README.md with deployment guide
   - [ ] Document environment variables
   - [ ] Add troubleshooting section
   - [ ] Create example systemd setup

#### Deliverables:
- [ ] Production-ready service
- [ ] Health monitoring
- [ ] Deployment automation
- [ ] Operational documentation

#### Validation:
```bash
# Build release binary
go build -o fx-collector ./cmd/collector

# Install systemd service
sudo cp scripts/fx-collector.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable fx-collector
sudo systemctl start fx-collector

# Monitor
sudo journalctl -u fx-collector -f
curl http://localhost:8080/health
```

---

### Session 4: Cleanup pivot-web2 ⏸️ PENDING
**Duration:** ~20-30 minutes  
**Dependencies:** Session 2 validated in production

#### Tasks:

1. **Remove Spread Recording Code**
   - [ ] Delete `internal/domain/spread_data.go`
   - [ ] Delete `internal/ports/spread_recorder.go`
   - [ ] Delete `internal/adapters/storage/csv_spread_recorder.go`
   - [ ] Delete `internal/adapters/storage/csv_spread_recorder_test.go`

2. **Remove Service Integration**
   - [ ] Search for SpreadRecorder references in services
   - [ ] Remove from SchedulerService if present
   - [ ] Remove from WebSocket integrator if present
   - [ ] Clean up environment variable handling

3. **Update Documentation**
   - [ ] Remove spread recording from ARCHITECTURE.md
   - [ ] Delete SPREAD_RECORDING_DEPLOYMENT.md
   - [ ] Update README.md to reference fx-collector
   - [ ] Add migration note in changelog

4. **Validation**
   - [ ] Run all tests: `go test ./...`
   - [ ] Build application: `go build ./cmd/server`
   - [ ] Verify no compilation errors
   - [ ] Check no broken imports

#### Deliverables:
- [ ] Cleaner pivot-web2 codebase
- [ ] Separation of concerns achieved
- [ ] All tests passing

#### Validation:
```bash
cd /home/bjorn/source/pivot-web2
go test ./...
go build ./cmd/server
# Should compile with no errors
```

---

## Configuration Reference

### Environment Variables (fx-collector)

**Saxo Credentials:**
```bash
SAXO_ENVIRONMENT=sim              # or "live"
SIM_CLIENT_ID=your_sim_id
SIM_CLIENT_SECRET=your_sim_secret
LIVE_CLIENT_ID=your_live_id       # Production only
LIVE_CLIENT_SECRET=your_live_secret
```

**Application Settings:**
```bash
SPREAD_RECORDING_DIR=data/spreads  # Output directory
SPREAD_FLUSH_INTERVAL=60           # Flush to disk every N seconds
LOG_LEVEL=info                     # debug, info, warn, error
HEALTH_CHECK_PORT=8080            # HTTP health endpoint
```

**Optional:**
```bash
TOKEN_STORAGE_PATH=./data         # OAuth token storage
INSTRUMENTS_CONFIG=config/instruments.json
```

### Instruments Configuration

**Format (config/instruments.json):**
```json
{
  "instruments": [
    {
      "ticker": "EURUSD",
      "uic": 21,
      "assetType": "FxSpot",
      "decimals": 5,
      "description": "Euro vs US Dollar"
    },
    {
      "ticker": "USDJPY",
      "uic": 23,
      "assetType": "FxSpot",
      "decimals": 3,
      "description": "US Dollar vs Japanese Yen"
    }
  ]
}
```

**Source Data:**
- Copy from `pivot-web2/data/fx.json` or `saxo-fx-instruments.json`
- Select only FX pairs needed for monitoring
- Validate UIC and AssetType match Saxo API

---

## Data Flow Architecture

### End-to-End Flow

```
[Saxo Streaming API]
        ↓ WebSocket
[saxo-adapter WebSocketClient]
        ↓ PriceUpdate channel
[CollectorService.consumePrices()]
        ↓ Map to PriceData
[SpreadRecorder.Record()]
        ↓ CSV Writer
[data/spreads/YYYYMMDD/TICKER_HH.csv]
```

### Detailed Mapping

```go
// Input from saxo-adapter
type PriceUpdate struct {
    Ticker    string    // "EURUSD"
    Bid       float64   // 1.08345
    Ask       float64   // 1.08355
    Mid       float64   // 1.08350
    Timestamp time.Time // 2025-11-26T14:30:45Z
}

// Enrichment from config
type InstrumentConfig struct {
    Ticker    string // "EURUSD"
    Uic       int    // 21
    AssetType string // "FxSpot"
    Decimals  int    // 5
}

// Output to recorder
type PriceData struct {
    Timestamp time.Time // From PriceUpdate
    Uic       int       // From config lookup
    Ticker    string    // From PriceUpdate
    AssetType string    // From config lookup
    Bid       float64   // From PriceUpdate
    Ask       float64   // From PriceUpdate
    Spread    float64   // Calculated: Ask - Bid
    Decimals  int       // From config lookup
}
```

---

## Testing Strategy

### Unit Tests
- [x] CSV Recorder: `internal/adapters/storage/*_test.go`
- [ ] Instrument Loader: `internal/config/*_test.go`
- [ ] Price Mapping: `internal/services/*_test.go`

### Integration Tests
- [ ] WebSocket connection (SIM environment)
- [ ] End-to-end price flow
- [ ] CSV file creation and content validation
- [ ] Token refresh handling

### Production Validation
- [ ] Run for 24 hours continuously
- [ ] Verify reconnection after network interruption
- [ ] Monitor disk usage
- [ ] Validate CSV data accuracy vs Saxo web platform

---

## Deployment Strategy

### Development Environment
```bash
# Local testing with .env file
cd fx-collector
cp .env.example .env
# Edit .env with SIM credentials
go run ./cmd/collector
```

### Production Deployment (VM)
```bash
# Build binary
GOOS=linux GOARCH=amd64 go build -o fx-collector ./cmd/collector

# Upload to VM
scp fx-collector user@vm:/opt/fx-collector/
scp .env user@vm:/opt/fx-collector/
scp -r config/ user@vm:/opt/fx-collector/

# Setup systemd
sudo cp scripts/fx-collector.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable fx-collector
sudo systemctl start fx-collector

# Monitor
sudo journalctl -u fx-collector -f
```

---

## Open Questions & Decisions Needed

### Before Starting Session 1:
- [x] **Project structure approved?** → Yes, proceed as outlined

### Before Starting Session 2:
- [x] **WebSocket connection lifecycle:** 24-hour continuous operation
- [x] **Instrument selection:** Use full pivot-web2 FX list (17 pairs from fx.json)
- [x] **Authentication:** Same SIM credentials as pivot-web2 (shared)
- [x] **Data storage:** fx-collector/data/spreads/ (independent)

### Before Starting Session 3:
- [ ] **Health monitoring:** HTTP endpoint or log-based?
- [ ] **Alerting:** Email, Slack, or just logs?
- [ ] **Disk management:** Auto-cleanup old files or manual?

---

## Risk Assessment

### Low Risk ✅
- **Code Porting:** Spread recording proven in production
- **saxo-adapter API:** Already tested and stable
- **CSV Format:** Simple, no schema migrations needed

### Medium Risk ⚠️
- **WebSocket Stability:** Long-term connection testing needed
- **Token Refresh:** Relies on saxo-adapter implementation
- **Disk Space:** Need monitoring for extended runs

### Mitigation Strategies
- Start with Session 1 (lowest risk, no external dependencies)
- Test Session 2 in SIM environment extensively
- Deploy Session 3 to non-critical VM first
- Keep pivot-web2 spread recording until fx-collector validated

---

## Success Criteria

### Session 1 Complete When:
- [x] `go test ./...` passes
- [x] `go build ./...` succeeds
- [x] CSV recorder creates valid files

### Session 2 Complete When:
- [ ] WebSocket connects to Saxo SIM
- [ ] Price updates flow to CSV files
- [ ] Data matches Saxo web platform prices
- [ ] Runs for 1+ hour without errors

### Session 3 Complete When:
- [ ] Service runs 24+ hours in production
- [ ] Health checks return success
- [ ] Reconnection works after network disruption
- [ ] Logs are structured and useful

### Session 4 Complete When:
- [ ] pivot-web2 builds without spread code
- [ ] All pivot-web2 tests pass
- [ ] Documentation updated

### Overall Project Success:
- [ ] fx-collector runs for 7+ days continuously
- [ ] Data quality validated
- [ ] saxo-adapter stability proven
- [ ] Clean architecture demonstrated
- [ ] Ready for pivot-web2 migration planning

---

## Progress Tracking

### Completed ✅
- [x] Project planning and architecture design
- [x] Documentation structure created
- [x] **SESSION 1 COMPLETED** - Foundation setup with all tests passing
- [x] Configuration decisions finalized

### In Progress 🚧
- [x] Session 1 - Foundation ✅ Completed
- [ ] Session 2 - Saxo-adapter integration (Next)

### Blocked 🚫
- [ ] None yet

### Next Actions 📋
1. ✅ Review this guide and answer open questions
2. ✅ Start Session 1: Foundation Setup
3. ✅ Validate with `go test` and `go build`
4. **START SESSION 2:** Saxo-adapter integration

---

## Related Documentation

### pivot-web2 References:
- `docs/operations/SPREAD_RECORDING_DEPLOYMENT.md` - Current implementation
- `docs/architecture/ARCHITECTURE.md` - Overall system architecture
- `docs/architecture/WEBSOCKET_TIMING_IMPLEMENTATION.md` - WebSocket lifecycle patterns
- `docs/architecture/TOKEN_REFRESH_IMPLEMENTATION.md` - OAuth token management

### saxo-adapter References:
- `README.md` - Quick start and API overview
- `adapter/interfaces.go` - Interface definitions
- `docs/ARCHITECTURE.md` - Adapter architecture
- `docs/COMPLETION_STATUS.md` - Current implementation status

### fx-collector References:
- `PROJECT_GUIDE.md` - This document
- `README.md` - User-facing documentation (to be created)

---

## Contact & Support

**Project Owner:** Bjorn  
**Repository:** `/home/bjorn/source/fx-collector`  
**Start Date:** November 26, 2025  
**Target Completion:** Session 1-2 within 1 week, Session 3-4 after validation period

---

**Last Updated:** November 26, 2025  
**Next Review:** After Session 1 completion
