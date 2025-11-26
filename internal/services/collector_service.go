package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bjoelf/fx-collector/internal/domain"
	"github.com/bjoelf/fx-collector/internal/ports"
	saxo "github.com/bjoelf/saxo-adapter/adapter"
	"github.com/bjoelf/saxo-adapter/adapter/websocket"
)

type Instrument struct {
	Ticker    string
	Uic       int
	AssetType string
	Decimals  int
}

type CollectorService struct {
	authClient     saxo.AuthClient
	brokerClient   saxo.BrokerClient
	wsClient       saxo.WebSocketClient
	instruments    map[string]Instrument
	spreadRecorder ports.SpreadRecorder
	logger         *log.Logger
	flushInterval  time.Duration
	flushTicker    *time.Ticker
	stopFlush      chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewCollectorService(
	authClient saxo.AuthClient,
	brokerClient saxo.BrokerClient,
	instruments map[string]Instrument,
	spreadRecorder ports.SpreadRecorder,
	flushInterval time.Duration,
	logger *log.Logger,
) (*CollectorService, error) {

	// Create WebSocket client
	wsClient := websocket.NewSaxoWebSocketClient(
		authClient,
		authClient.GetBaseURL(),
		authClient.GetWebSocketURL(),
		logger,
	)

	ctx, cancel := context.WithCancel(context.Background())

	return &CollectorService{
		authClient:     authClient,
		brokerClient:   brokerClient,
		wsClient:       wsClient,
		instruments:    instruments,
		spreadRecorder: spreadRecorder,
		logger:         logger,
		flushInterval:  flushInterval,
		stopFlush:      make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

func (cs *CollectorService) Start() error {
	cs.logger.Println("Starting FX Collector Service...")

	if !cs.authClient.IsAuthenticated() {
		cs.logger.Println("Not authenticated - attempting login...")
		if err := cs.authClient.Login(cs.ctx); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		cs.logger.Println("Authentication successful")
	}

	wsStateChannel := make(chan bool, 1)
	wsContextIDChannel := make(chan string, 1)
	cs.wsClient.SetStateChannels(wsStateChannel, wsContextIDChannel)

	if saxoAuth, ok := cs.authClient.(interface {
		StartTokenEarlyRefresh(ctx context.Context, wsConnected <-chan bool, wsContextID <-chan string)
	}); ok {
		go saxoAuth.StartTokenEarlyRefresh(cs.ctx, wsStateChannel, wsContextIDChannel)
		cs.logger.Println("Token refresh manager started")
	}

	cs.logger.Println("Connecting to Saxo WebSocket...")
	if err := cs.wsClient.Connect(cs.ctx); err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}
	cs.logger.Println("WebSocket connected")

	tickers := cs.getAllTickers()
	cs.logger.Printf("Subscribing to %d instruments", len(tickers))

	if err := cs.wsClient.SubscribeToPrices(cs.ctx, tickers); err != nil {
		return fmt.Errorf("price subscription failed: %w", err)
	}
	cs.logger.Println("Price subscriptions established")

	go cs.processPriceUpdates()
	cs.startPeriodicFlush()

	cs.logger.Println("FX Collector Service started successfully")
	return nil
}

func (cs *CollectorService) processPriceUpdates() {
	cs.logger.Println("Starting price update processor...")

	priceChannel := cs.wsClient.GetPriceUpdateChannel()
	updateCount := 0

	for {
		select {
		case <-cs.ctx.Done():
			cs.logger.Printf("Price processor stopping (received %d updates)", updateCount)
			return

		case priceUpdate, ok := <-priceChannel:
			if !ok {
				cs.logger.Println("Price channel closed")
				return
			}

			priceData, err := cs.mapPriceUpdate(&priceUpdate)
			if err != nil {
				cs.logger.Printf("Error mapping price for %s: %v", priceUpdate.Ticker, err)
				continue
			}

			if err := cs.spreadRecorder.Record(cs.ctx, priceData); err != nil {
				cs.logger.Printf("Error recording price for %s: %v", priceUpdate.Ticker, err)
				continue
			}

			updateCount++
			if updateCount%100 == 0 {
				cs.logger.Printf("Processed %d price updates", updateCount)
			}
		}
	}
}

func (cs *CollectorService) mapPriceUpdate(update *saxo.PriceUpdate) (*domain.PriceData, error) {
	instrument, ok := cs.instruments[update.Ticker]
	if !ok {
		return nil, fmt.Errorf("instrument not found: %s", update.Ticker)
	}

	priceData := &domain.PriceData{
		Timestamp: update.Timestamp,
		Uic:       instrument.Uic,
		Ticker:    update.Ticker,
		AssetType: instrument.AssetType,
		Bid:       update.Bid,
		Ask:       update.Ask,
		Decimals:  instrument.Decimals,
	}

	priceData.CalculateSpread()
	return priceData, nil
}

func (cs *CollectorService) getAllTickers() []string {
	tickers := make([]string, 0, len(cs.instruments))
	for ticker := range cs.instruments {
		tickers = append(tickers, ticker)
	}
	return tickers
}

func (cs *CollectorService) startPeriodicFlush() {
	cs.flushTicker = time.NewTicker(cs.flushInterval)

	go func() {
		cs.logger.Printf("Starting periodic flush (every %v)", cs.flushInterval)

		for {
			select {
			case <-cs.ctx.Done():
				return
			case <-cs.stopFlush:
				return
			case <-cs.flushTicker.C:
				if err := cs.spreadRecorder.Flush(cs.ctx); err != nil {
					cs.logger.Printf("Flush error: %v", err)
				}
			}
		}
	}()
}

func (cs *CollectorService) Stop() error {
	cs.logger.Println("Stopping FX Collector Service...")

	if cs.flushTicker != nil {
		cs.flushTicker.Stop()
		close(cs.stopFlush)
	}

	cs.cancel()

	cs.logger.Println("Performing final flush...")
	if err := cs.spreadRecorder.Flush(cs.ctx); err != nil {
		cs.logger.Printf("Final flush error: %v", err)
	}

	cs.logger.Println("Closing WebSocket connection...")
	if err := cs.wsClient.Close(); err != nil {
		cs.logger.Printf("WebSocket close error: %v", err)
	}

	cs.logger.Println("Closing spread recorder...")
	if err := cs.spreadRecorder.Close(); err != nil {
		cs.logger.Printf("Recorder close error: %v", err)
	}

	cs.logger.Println("FX Collector Service stopped")
	return nil
}
