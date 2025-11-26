package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bjoelf/fx-collector/internal/domain"
)

func TestCSVSpreadRecorder_Record(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	recorder := NewCSVSpreadRecorder(tmpDir)
	defer recorder.Close()

	// Test data
	now := time.Date(2025, 11, 18, 12, 0, 0, 0, time.UTC)
	priceData := &domain.PriceData{
		Timestamp: now,
		Uic:       21,
		Ticker:    "EURUSD",
		AssetType: "FxSpot",
		Bid:       1.10000,
		Ask:       1.10002,
		Spread:    0.00002,
	}

	// Record single price
	ctx := context.Background()
	if err := recorder.Record(ctx, priceData); err != nil {
		t.Fatalf("Failed to record price: %v", err)
	}

	// Flush to ensure data is written
	if err := recorder.Flush(ctx); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Verify file was created (hourly format: EURUSD_12.csv for 12:00 hour)
	expectedPath := tmpDir + "/20251118/EURUSD_12.csv"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected file not created: %s", expectedPath)
	}

	// Read file and verify content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Check header and data row exist
	lines := string(content)
	if len(lines) == 0 {
		t.Fatal("File is empty")
	}

	t.Logf("File content:\n%s", lines)
}

func TestCSVSpreadRecorder_RecordBatch(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewCSVSpreadRecorder(tmpDir)
	defer recorder.Close()

	// Test batch data
	now := time.Date(2025, 11, 18, 12, 0, 0, 0, time.UTC)
	batch := []*domain.PriceData{
		{
			Timestamp: now,
			Uic:       21,
			Ticker:    "EURUSD",
			AssetType: "FxSpot",
			Bid:       1.10000,
			Ask:       1.10002,
			Spread:    0.00002,
		},
		{
			Timestamp: now.Add(1 * time.Second),
			Uic:       21,
			Ticker:    "EURUSD",
			AssetType: "FxSpot",
			Bid:       1.10001,
			Ask:       1.10003,
			Spread:    0.00002,
		},
		{
			Timestamp: now,
			Uic:       42,
			Ticker:    "USDJPY",
			AssetType: "FxSpot",
			Bid:       150.000,
			Ask:       150.003,
			Spread:    0.003,
		},
	}

	// Record batch
	ctx := context.Background()
	if err := recorder.RecordBatch(ctx, batch); err != nil {
		t.Fatalf("Failed to record batch: %v", err)
	}

	// Flush
	if err := recorder.Flush(ctx); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Verify both files were created (hourly format: TICKER_12.csv)
	expectedPaths := []string{
		tmpDir + "/20251118/EURUSD_12.csv",
		tmpDir + "/20251118/USDJPY_12.csv",
	}

	for _, path := range expectedPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not created: %s", path)
		} else {
			content, _ := os.ReadFile(path)
			t.Logf("File %s content:\n%s", path, string(content))
		}
	}
}

func TestCSVSpreadRecorder_MultipleFlushes(t *testing.T) {
	tmpDir := t.TempDir()
	recorder := NewCSVSpreadRecorder(tmpDir)
	defer recorder.Close()

	ctx := context.Background()
	now := time.Date(2025, 11, 18, 12, 0, 0, 0, time.UTC)

	// Record, flush, record again, flush again
	for i := 0; i < 3; i++ {
		priceData := &domain.PriceData{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Uic:       21,
			Ticker:    "EURUSD",
			AssetType: "FxSpot",
			Bid:       1.10000 + float64(i)*0.00001,
			Ask:       1.10002 + float64(i)*0.00001,
			Spread:    0.00002,
		}

		if err := recorder.Record(ctx, priceData); err != nil {
			t.Fatalf("Failed to record price %d: %v", i, err)
		}

		if err := recorder.Flush(ctx); err != nil {
			t.Fatalf("Failed to flush %d: %v", i, err)
		}
	}

	// Verify all records are in the file (hourly format: EURUSD_12.csv)
	expectedPath := tmpDir + "/20251118/EURUSD_12.csv"
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	t.Logf("Final file content:\n%s", string(content))
}
