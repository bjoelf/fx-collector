# FX Collector

**Standalone FX price collection service** - Real-time spread recording using the [saxo-adapter](https://github.com/bjoelf/saxo-adapter) library.

> **📚 Demo Project:** This project also serves as a demonstration of how to consume the [saxo-adapter](https://github.com/bjoelf/saxo-adapter) library for Saxo Bank API integration with OAuth2 authentication and WebSocket streaming.

## Purpose

FX Collector is a lightweight service that:

- Collects real-time FX price data (bid/ask/spread) via WebSocket
- Records spread data to CSV files for analysis
- Tests the saxo-adapter library for long-term stability
- Runs independently from trading logic

## Quick Start

### 1. Configuration

Edit `.env` with your Saxo credentials:

```env
# Saxo credentials (SIM environment)
SAXO_ENVIRONMENT=sim
SAXO_CLIENT_ID=your_client_id
SAXO_CLIENT_SECRET=your_client_secret

# Application settings
SPREAD_RECORDING_DIR=data/spreads
SPREAD_FLUSH_INTERVAL=30s
```

**Important:** Configure the OAuth callback URL in your Saxo Bank application settings:

<http://localhost:8080/oauth/callback>

This callback URL must be registered in your Saxo OpenAPI application configuration.

If URL or port is changed this must be reflected in your Saxo OpenAPI application configuration.

### 2. Run Locally

```bash
# Install dependencies
go mod tidy

# Run the collector
go run ./cmd/collector
```

### 3. Verify Data Collection

```bash
# Check spread data files
ls -la data/spreads/$(date +%Y%m%d)/

# View live data
tail -f data/spreads/$(date +%Y%m%d)/EURUSD_$(date +%H).csv
```

## Features

- ✅ **24-hour operation** - Continuous price collection
- ✅ **17 FX pairs** - Major currency pairs (EURUSD, USDJPY, etc.)
- ✅ **Hourly CSV files** - Organized by date and hour
- ✅ **Automatic reconnection** - Handles network interruptions
- ✅ **Token refresh** - OAuth2 automatic token management
- ✅ **Minimal UI** - Simple login page at <http://localhost:8080>

## Data Format

CSV files: `data/spreads/YYYYMMDD/TICKER_HH.csv`

```csv
timestamp,uic,ticker,asset_type,bid,ask,spread
2025-11-26T14:30:45.123Z,21,EURUSD,FxSpot,1.0834,1.0835,0.0001
```

## Architecture

main.go → LoadInstruments() → saxo.CreateSaxoAuthClient() → CollectorService → WebSocket → CSV Files

**Simplified Design:**

- **No config package** - Everything in main.go

- **saxo-adapter handles OAuth** - Uses `LoadSaxoEnvironmentConfig()`
- **Direct instrument map** - Simple `map[string]Instrument` lookup
- **Minimal dependencies** - Clean and straightforward

Flow:

- **saxo-adapter (WebSocket)** → CollectorService → CSVSpreadRecorder → CSV Files

## Configuration Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `SAXO_ENVIRONMENT` | `sim` | Trading environment (`sim` or `live`) |
| `SAXO_CLIENT_ID` | - | Saxo OAuth client ID (required) |
| `SAXO_CLIENT_SECRET` | - | Saxo OAuth secret (required) |
| `SPREAD_RECORDING_DIR` | `data/spreads` | Output directory for CSV files |
| `SPREAD_FLUSH_INTERVAL` | `30s` | How often to flush data to disk |
| `INSTRUMENTS_PATH` | `data/instruments.json` | Path to instruments configuration |

## Instruments Monitored

17 FX spot pairs from `config/instruments.json`:

- **Major Pairs**: EURUSD, USDJPY, GBPUSD
- **Cross Pairs**: EURJPY, GBPJPY, AUDJPY, CHFJPY
- **Others**: AUDUSD, USDCAD, USDCHF, and more

Edit `config/instruments.json` to customize monitored instruments.

## Development

```bash
# Run tests
go test ./...

# Build binary
go build -o fx-collector ./cmd/collector

# Run with custom config
INSTRUMENTS_CONFIG=custom.json go run ./cmd/collector
```

## Deployment

See [PROJECT_GUIDE.md](PROJECT_GUIDE.md) for detailed deployment instructions.

## Troubleshooting

**No data appearing:**

- Check OAuth credentials are correct in `.env`
- Verify `BROKER_CLIENT_ID` and `BROKER_CLIENT_SECRET` are set
- Verify all required URLs (`AUTH_URL`, `TOKEN_URL`, `BROKER_BASE_URL`, `BROKER_WEBSOCKET_URL`)
- Check WebSocket connection in logs

**Connection drops:**

- Network interruptions are handled automatically
- Check logs for reconnection attempts
- Verify token refresh is working

**File errors:**

- Ensure `data/spreads/` directory is writable
- Check disk space availability

## Project Status

This is part of the pivot-web2 refactoring project. See [PROJECT_GUIDE.md](PROJECT_GUIDE.md) for overall architecture and roadmap.

## License

See parent project license.
