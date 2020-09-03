package api

import "time"

// Config contains the API configuration
type Config struct {
	AssetConfigs map[string]AssetConfig `configkey:"api.assets" validate:"required"`
}

// AssetConfig represents one asset configuration delivered by the oracle
type AssetConfig struct {
	Asset       string            `configkey:"asset" validate:"required"`
	Currency    string            `configkey:"currency" validate:"required"`
	HasDecimals bool              `configkey:"hasDecimals"`
	StartDate   time.Time         `configkey:"startDate" validate:"required"`
	Frequency   time.Duration     `configkey:"frequency,duration,iso8601" validate:"required"`
	RangeD      time.Duration     `configkey:"range,duration,iso8601" validate:"required"`
	Test        map[string]string `configkey:"test"`
	EventTypes  map[string]bool   `configkey:"eventTypes"`
}
