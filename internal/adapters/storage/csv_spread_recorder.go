package storage

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/bjoelf/fx-collector/internal/domain"
)

// roundPrice rounds a float64 to the specified number of decimals
// Used for proper FX price formatting (e.g., 4 decimals for EURUSD, 2 for USDJPY)
func roundPrice(price float64, decimals int) float64 {
	if decimals <= 0 {
		return price // No rounding if decimals not specified
	}
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(price*multiplier) / multiplier
}

// CSVSpreadRecorder implements SpreadRecorder interface using CSV files
// File format: data/spreads/YYYYMMDD/TICKER_HH.csv (hourly files)
// Columns: timestamp,uic,ticker,asset_type,bid,ask,spread
// Using hourly files reduces file count from ~40,000/day to ~672/day (60× reduction)
type CSVSpreadRecorder struct {
	baseDir    string
	writers    map[string]*csv.Writer
	files      map[string]*os.File
	buffers    map[string]*bufio.Writer
	mu         sync.Mutex
	bufferSize int // Number of records to buffer before flush
}

// NewCSVSpreadRecorder creates a new CSV-based spread recorder
func NewCSVSpreadRecorder(baseDir string) *CSVSpreadRecorder {
	return &CSVSpreadRecorder{
		baseDir:    baseDir,
		writers:    make(map[string]*csv.Writer),
		files:      make(map[string]*os.File),
		buffers:    make(map[string]*bufio.Writer),
		bufferSize: 100, // Buffer 100 records before auto-flush
	}
}

// Record saves a single price data point
func (r *CSVSpreadRecorder) Record(ctx context.Context, data *domain.PriceData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	writer, err := r.getWriter(data.Ticker, data.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	// Round prices based on instrument decimals (e.g., 4 for EURUSD, 2 for USDJPY)
	bid := roundPrice(data.Bid, data.Decimals)
	ask := roundPrice(data.Ask, data.Decimals)
	spread := roundPrice(data.Spread, data.Decimals)

	record := []string{
		data.Timestamp.Format(time.RFC3339Nano),
		strconv.Itoa(data.Uic),
		data.Ticker,
		data.AssetType,
		strconv.FormatFloat(bid, 'f', data.Decimals, 64),
		strconv.FormatFloat(ask, 'f', data.Decimals, 64),
		strconv.FormatFloat(spread, 'f', data.Decimals, 64),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}

// RecordBatch saves multiple price data points efficiently
func (r *CSVSpreadRecorder) RecordBatch(ctx context.Context, data []*domain.PriceData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, priceData := range data {
		writer, err := r.getWriter(priceData.Ticker, priceData.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to get writer for %s: %w", priceData.Ticker, err)
		}

		// Round prices based on instrument decimals
		bid := roundPrice(priceData.Bid, priceData.Decimals)
		ask := roundPrice(priceData.Ask, priceData.Decimals)
		spread := roundPrice(priceData.Spread, priceData.Decimals)

		record := []string{
			priceData.Timestamp.Format(time.RFC3339Nano),
			strconv.Itoa(priceData.Uic),
			priceData.Ticker,
			priceData.AssetType,
			strconv.FormatFloat(bid, 'f', priceData.Decimals, 64),
			strconv.FormatFloat(ask, 'f', priceData.Decimals, 64),
			strconv.FormatFloat(spread, 'f', priceData.Decimals, 64),
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record for %s: %w", priceData.Ticker, err)
		}
	}

	return nil
}

// Flush ensures all buffered data is written to storage
func (r *CSVSpreadRecorder) Flush(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("CSVSpreadRecorder: Flushing %d writers...", len(r.writers))

	for ticker, writer := range r.writers {
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("failed to flush writer for %s: %w", ticker, err)
		}

		// Flush buffered writer
		if buf, ok := r.buffers[ticker]; ok {
			if err := buf.Flush(); err != nil {
				return fmt.Errorf("failed to flush buffer for %s: %w", ticker, err)
			}
		}
		log.Printf("CSVSpreadRecorder: ✅ Flushed %s", ticker)
	}

	log.Printf("CSVSpreadRecorder: All writers flushed")
	return nil
}

// Close finalizes the recording session and releases resources
func (r *CSVSpreadRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Flush all writers
	for ticker, writer := range r.writers {
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("failed to flush writer for %s during close: %w", ticker, err)
		}
	}

	// Flush and close all buffers
	for ticker, buf := range r.buffers {
		if err := buf.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer for %s during close: %w", ticker, err)
		}
	}

	// Close all files
	for ticker, file := range r.files {
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close file for %s: %w", ticker, err)
		}
	}

	// Clear maps
	r.writers = make(map[string]*csv.Writer)
	r.buffers = make(map[string]*bufio.Writer)
	r.files = make(map[string]*os.File)

	return nil
}

// getWriter returns a CSV writer for the given ticker and timestamp
// Creates directory structure and file if they don't exist
// Uses hourly files: TICKER_HH.csv (e.g., EURUSD_14.csv for 14:00-14:59)
// Automatically closes old hourly files to prevent resource leaks
func (r *CSVSpreadRecorder) getWriter(ticker string, timestamp time.Time) (*csv.Writer, error) {
	dateStr := timestamp.Format("20060102")
	hourStr := timestamp.Format("15") // HH format (hour only)
	key := fmt.Sprintf("%s_%s_%s", ticker, dateStr, hourStr)

	// Return existing writer if available
	if writer, ok := r.writers[key]; ok {
		return writer, nil
	}

	// Close old hourly files for this ticker to prevent resource leaks
	// Search for keys with same ticker but different hour/date
	for oldKey, oldWriter := range r.writers {
		if len(oldKey) > len(ticker) && oldKey[:len(ticker)] == ticker && oldKey != key {
			// Flush and close the old writer
			oldWriter.Flush()
			if err := oldWriter.Error(); err != nil {
				log.Printf("Warning: Error flushing old writer for %s: %v", oldKey, err)
			}

			// Flush and close buffer
			if buf, ok := r.buffers[oldKey]; ok {
				if err := buf.Flush(); err != nil {
					log.Printf("Warning: Error flushing old buffer for %s: %v", oldKey, err)
				}
			}

			// Close file
			if file, ok := r.files[oldKey]; ok {
				if err := file.Close(); err != nil {
					log.Printf("Warning: Error closing old file for %s: %v", oldKey, err)
				}
			}

			// Remove from maps
			delete(r.writers, oldKey)
			delete(r.buffers, oldKey)
			delete(r.files, oldKey)

			log.Printf("CSVSpreadRecorder: ✅ Closed old hourly file: %s", oldKey)
		}
	} // Create directory: data/spreads/YYYYMMDD/
	dirPath := filepath.Join(r.baseDir, dateStr)
	log.Printf("CSVSpreadRecorder: Creating directory: %s", dirPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Create file: TICKER_HH.csv (hourly file)
	filename := fmt.Sprintf("%s_%s.csv", ticker, hourStr)
	filePath := filepath.Join(dirPath, filename)

	// Check if file exists to determine if we need to write header
	fileExists := false
	if _, err := os.Stat(filePath); err == nil {
		fileExists = true
	}

	log.Printf("CSVSpreadRecorder: Opening file: %s (exists=%v)", filePath, fileExists)

	// Open file in append mode
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	// Create buffered writer
	buffer := bufio.NewWriter(file)
	writer := csv.NewWriter(buffer)

	// Write header if new file
	if !fileExists {
		header := []string{"timestamp", "uic", "ticker", "asset_type", "bid", "ask", "spread"}
		if err := writer.Write(header); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write header: %w", err)
		}
		log.Printf("CSVSpreadRecorder: Header written to %s", filePath)
	}

	// Store references
	r.files[key] = file
	r.buffers[key] = buffer
	r.writers[key] = writer

	log.Printf("CSVSpreadRecorder: ✅ Writer created for %s -> %s", ticker, filePath)

	return writer, nil
}
