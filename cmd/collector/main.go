package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bjoelf/fx-collector/internal/adapters/storage"
	"github.com/bjoelf/fx-collector/internal/services"
	saxo "github.com/bjoelf/saxo-adapter/adapter"
	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	InstrumentsPath string
	SpreadDir       string
	FlushInterval   time.Duration
	Instruments     map[string]services.Instrument
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run() error {
	logger := log.New(os.Stdout, "[FX-COLLECTOR] ", log.LstdFlags|log.Lmsgprefix)
	logger.Println("=== FX Collector Starting ===")

	// Load configuration from .env file and environment
	config, err := loadConfig(logger)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create Saxo auth client (handles OAuth automatically)
	logger.Println("Creating Saxo authentication client...")
	authClient, err := saxo.CreateSaxoAuthClient(logger)
	if err != nil {
		return fmt.Errorf("failed to create auth client: %w", err)
	}

	// If you arrive here from examples/basic_auth,
	// and wonder where the authentication step is:
	// the authClient.Login() happens in NewCollectorService.Start()

	// Create broker services (inject authClient)
	logger.Println("Creating broker services...")
	brokerClient, err := saxo.CreateBrokerServices(authClient, logger)
	if err != nil {
		return fmt.Errorf("failed to create broker services: %w", err)
	}

	// Create spread recorder
	spreadRecorder := storage.NewCSVSpreadRecorder(config.SpreadDir)

	// Create collector service
	collectorService, err := services.NewCollectorService(
		authClient,
		brokerClient,
		config.Instruments,
		spreadRecorder,
		config.FlushInterval,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create collector service: %w", err)
	}

	// Start collector service
	if err := collectorService.Start(); err != nil {
		return fmt.Errorf("failed to start collector service: %w", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	logger.Println("=== FX Collector Running (press Ctrl+C to stop) ===")
	<-sigChan
	logger.Println("\n=== Shutdown Signal Received ===")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownComplete := make(chan error, 1)
	go func() {
		shutdownComplete <- collectorService.Stop()
	}()

	select {
	case err := <-shutdownComplete:
		if err != nil {
			logger.Printf("Shutdown completed with errors: %v", err)
			return err
		}
		logger.Println("=== Shutdown Complete ===")
		return nil
	case <-shutdownCtx.Done():
		logger.Println("=== Shutdown Timeout - Forcing Exit ===")
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// loadConfig loads all configuration from .env file and environment variables
func loadConfig(logger *log.Logger) (*Config, error) {
	// Load .env file following pivot-web2 pattern (supports debug run from cmd/collector/ and run from root)
	envPaths := []string{
		".env",       // Current directory (root)
		"../../.env", // From cmd/collector/ to project root
		"../.env",    // From cmd/ to project root
	}

	loaded := false
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			if err := godotenv.Load(envPath); err == nil {
				loaded = true
				logger.Printf("Loaded .env from: %s", envPath)
				break
			}
		}
	}

	if !loaded {
		logger.Println("Warning: .env file not found in any expected location, using system environment variables")
	}

	// Read configuration values from environment with multiple relative path support for instruments
	instrumentsPaths := []string{
		getEnv("INSTRUMENTS_PATH", "data/instruments.json"), // Default from env or "data/instruments.json"
		"../../data/instruments.json",                       // From cmd/collector/ to project root
		"../data/instruments.json",                          // From cmd/ to project root
		"data/instruments.json",                             // Current directory
	}

	var instrumentsPath string
	for _, path := range instrumentsPaths {
		if _, err := os.Stat(path); err == nil {
			instrumentsPath = path
			logger.Printf("Found instruments file at: %s", path)
			break
		}
	}

	if instrumentsPath == "" {
		return nil, fmt.Errorf("instruments file not found in any expected location: %v", instrumentsPaths)
	}

	spreadDir := getEnv("SPREAD_RECORDING_DIR", "data/spreads")
	flushIntervalStr := getEnv("SPREAD_FLUSH_INTERVAL", "30s")

	// Parse flush interval
	flushInterval, err := time.ParseDuration(flushIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SPREAD_FLUSH_INTERVAL '%s': %w", flushIntervalStr, err)
	}

	// Load instruments from JSON file
	logger.Printf("Loading instruments from: %s", instrumentsPath)
	instruments, err := loadInstruments(instrumentsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load instruments: %w", err)
	}
	logger.Printf("Loaded %d instruments", len(instruments))

	return &Config{
		InstrumentsPath: instrumentsPath,
		SpreadDir:       spreadDir,
		FlushInterval:   flushInterval,
		Instruments:     instruments,
	}, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// instrument represents a trading instrument from JSON
type instrument struct {
	Ticker    string `json:"ticker"`
	Uic       int    `json:"uic"`
	AssetType string `json:"assetType"`
	Decimals  int    `json:"decimals"`
}

// loadInstruments loads trading instruments from a JSON file
func loadInstruments(filepath string) (map[string]services.Instrument, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config struct {
		Instruments []instrument `json:"instruments"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(config.Instruments) == 0 {
		return nil, fmt.Errorf("no instruments found")
	}

	// Convert to map for easy lookup
	instruments := make(map[string]services.Instrument)
	for _, inst := range config.Instruments {
		instruments[inst.Ticker] = services.Instrument{
			Ticker:    inst.Ticker,
			Uic:       inst.Uic,
			AssetType: inst.AssetType,
			Decimals:  inst.Decimals,
		}
	}

	return instruments, nil
}
