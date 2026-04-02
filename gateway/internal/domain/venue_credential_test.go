package domain

import (
	"testing"
	"time"
)

func TestVenueCredential_AllFieldsPopulated(t *testing.T) {
	now := time.Now()
	cred := VenueCredential{
		VenueID:    "coinbase-pro",
		APIKey:     "key-123",
		APISecret:  "secret-456",
		Passphrase: "pass-789",
		Metadata: map[string]string{
			"sandbox": "true",
			"region":  "us-east",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if cred.VenueID != "coinbase-pro" {
		t.Errorf("VenueID = %q, want %q", cred.VenueID, "coinbase-pro")
	}
	if cred.APIKey != "key-123" {
		t.Errorf("APIKey = %q, want %q", cred.APIKey, "key-123")
	}
	if cred.APISecret != "secret-456" {
		t.Errorf("APISecret = %q, want %q", cred.APISecret, "secret-456")
	}
	if cred.Passphrase != "pass-789" {
		t.Errorf("Passphrase = %q, want %q", cred.Passphrase, "pass-789")
	}
	if len(cred.Metadata) != 2 {
		t.Errorf("Metadata length = %d, want 2", len(cred.Metadata))
	}
	if !cred.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", cred.CreatedAt, now)
	}
	if !cred.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", cred.UpdatedAt, now)
	}
}

func TestVenueCredential_MetadataSetAndRetrieve(t *testing.T) {
	cred := VenueCredential{
		VenueID:  "binance",
		Metadata: map[string]string{},
	}

	cred.Metadata["testnet"] = "true"
	cred.Metadata["rate_limit"] = "1200"

	if got := cred.Metadata["testnet"]; got != "true" {
		t.Errorf("Metadata[testnet] = %q, want %q", got, "true")
	}
	if got := cred.Metadata["rate_limit"]; got != "1200" {
		t.Errorf("Metadata[rate_limit] = %q, want %q", got, "1200")
	}
	if got := cred.Metadata["nonexistent"]; got != "" {
		t.Errorf("Metadata[nonexistent] = %q, want empty string", got)
	}
}

func TestVenueCredential_WithoutOptionalFields(t *testing.T) {
	now := time.Now()
	cred := VenueCredential{
		VenueID:   "alpaca",
		APIKey:    "AKXYZ",
		APISecret: "secret",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if cred.Passphrase != "" {
		t.Errorf("Passphrase = %q, want empty string", cred.Passphrase)
	}
	if cred.Metadata != nil {
		t.Errorf("Metadata = %v, want nil", cred.Metadata)
	}
	if cred.VenueID != "alpaca" {
		t.Errorf("VenueID = %q, want %q", cred.VenueID, "alpaca")
	}
}
