package domain

import "time"

// VenueCredential holds plaintext API credentials for a trading venue.
// Encryption is handled by the credential manager, not the domain layer.
// This type is used as the parameter for LiquidityProvider.Connect().
type VenueCredential struct {
	VenueID    string
	APIKey     string
	APISecret  string
	Passphrase string            // Optional (e.g. Coinbase Pro requires this, Alpaca/Binance do not)
	Metadata   map[string]string // Extra venue-specific fields
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
