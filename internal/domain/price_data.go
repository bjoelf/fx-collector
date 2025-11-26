package domain

import "time"

// PriceData represents bid/ask price data for spread analysis
type PriceData struct {
Timestamp time.Time `json:"timestamp"`
Uic       int       `json:"uic"`
Ticker    string    `json:"ticker"`
AssetType string    `json:"asset_type"`
Bid       float64   `json:"bid"`
Ask       float64   `json:"ask"`
Spread    float64   `json:"spread"`
Decimals  int       `json:"decimals,omitempty"` // Number of decimals for price rounding
}

// CalculateSpread computes the spread from bid/ask prices
func (p *PriceData) CalculateSpread() {
p.Spread = p.Ask - p.Bid
}
