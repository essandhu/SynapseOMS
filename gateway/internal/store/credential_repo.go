package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CredentialRow represents a row in the venue_credentials table.
type CredentialRow struct {
	VenueID             string
	EncryptedAPIKey     []byte
	EncryptedAPISecret  []byte
	EncryptedPassphrase []byte
	Salt                []byte
	Nonce               []byte
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StoreCredential inserts or updates a venue credential using INSERT ON CONFLICT.
func (s *PostgresStore) StoreCredential(ctx context.Context, c *CredentialRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO venue_credentials (
			venue_id, encrypted_api_key, encrypted_api_secret,
			encrypted_passphrase, salt, nonce,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8
		)
		ON CONFLICT (venue_id) DO UPDATE SET
			encrypted_api_key = EXCLUDED.encrypted_api_key,
			encrypted_api_secret = EXCLUDED.encrypted_api_secret,
			encrypted_passphrase = EXCLUDED.encrypted_passphrase,
			salt = EXCLUDED.salt,
			nonce = EXCLUDED.nonce,
			updated_at = EXCLUDED.updated_at`,
		c.VenueID,
		c.EncryptedAPIKey,
		c.EncryptedAPISecret,
		c.EncryptedPassphrase,
		c.Salt,
		c.Nonce,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("storing credential: %w", err)
	}
	return nil
}

// GetCredential retrieves a venue credential by venue ID.
func (s *PostgresStore) GetCredential(ctx context.Context, venueID string) (*CredentialRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT venue_id, encrypted_api_key, encrypted_api_secret,
			encrypted_passphrase, salt, nonce,
			created_at, updated_at
		FROM venue_credentials
		WHERE venue_id = $1`, venueID)

	return scanCredential(row)
}

// DeleteCredential removes a venue credential by venue ID.
func (s *PostgresStore) DeleteCredential(ctx context.Context, venueID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM venue_credentials WHERE venue_id = $1`, venueID)
	if err != nil {
		return fmt.Errorf("deleting credential: %w", err)
	}
	return nil
}

// HasCredential checks whether a credential exists for the given venue ID
// without retrieving or decrypting the actual credential data.
func (s *PostgresStore) HasCredential(ctx context.Context, venueID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM venue_credentials WHERE venue_id = $1
		)`, venueID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking credential existence: %w", err)
	}
	return exists, nil
}

// scanCredential scans a single credential from a pgx.Row.
func scanCredential(row pgx.Row) (*CredentialRow, error) {
	var c CredentialRow

	err := row.Scan(
		&c.VenueID, &c.EncryptedAPIKey, &c.EncryptedAPISecret,
		&c.EncryptedPassphrase, &c.Salt, &c.Nonce,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning credential: %w", err)
	}

	return &c, nil
}
