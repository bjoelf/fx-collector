package ports

import (
"context"

"github.com/bjoelf/fx-collector/internal/domain"
)

// SpreadRecorder handles recording of spread data to persistent storage
type SpreadRecorder interface {
// Record saves a single price data point
Record(ctx context.Context, data *domain.PriceData) error

// RecordBatch saves multiple price data points efficiently
RecordBatch(ctx context.Context, data []*domain.PriceData) error

// Flush ensures all buffered data is written to storage
Flush(ctx context.Context) error

// Close finalizes the recording session and releases resources
Close() error
}
