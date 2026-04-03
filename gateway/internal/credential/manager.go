package credential

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/store"
)

// Sentinel errors for credential operations.
var (
	ErrNotFound          = errors.New("credential not found")
	ErrPassphraseMismatch = errors.New("old passphrase does not match current passphrase")
	ErrEmptyPassphrase    = errors.New("passphrase must not be empty")
)

// CredentialStore abstracts the persistence layer for encrypted credentials.
// Implemented by store.PostgresStore and by in-memory fakes in tests.
type CredentialStore interface {
	StoreCredential(ctx context.Context, cred *store.CredentialRow) error
	GetCredential(ctx context.Context, venueID string) (*store.CredentialRow, error)
	DeleteCredential(ctx context.Context, venueID string) error
	HasCredential(ctx context.Context, venueID string) (bool, error)
	ListVenueIDs(ctx context.Context) ([]string, error)
}

// CredentialManager encrypts and decrypts venue credentials using AES-256-GCM
// with per-credential Argon2id key derivation. The master passphrase is held
// only in memory and is never persisted or logged.
type CredentialManager struct {
	passphrase string          // held in memory; used to derive per-credential keys
	store      CredentialStore // persistence backend
	kdfParams  KDFParams       // configurable Argon2id parameters
}

// NewCredentialManager creates a new manager bound to the given passphrase
// and credential store. Optional KDFParams may be provided; if omitted,
// DefaultKDFParams() is used.
func NewCredentialManager(passphrase string, cs CredentialStore, opts ...KDFParams) (*CredentialManager, error) {
	if passphrase == "" {
		return nil, ErrEmptyPassphrase
	}
	if cs == nil {
		return nil, errors.New("credential store must not be nil")
	}
	params := DefaultKDFParams()
	if len(opts) > 0 {
		params = opts[0]
	}
	return &CredentialManager{
		passphrase: passphrase,
		store:      cs,
		kdfParams:  params,
	}, nil
}

// Store encrypts the credential fields and persists them.
func (m *CredentialManager) Store(ctx context.Context, cred domain.VenueCredential) error {
	if cred.VenueID == "" {
		return errors.New("venue ID must not be empty")
	}

	salt, err := generateSalt()
	if err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	key := deriveKeyWithParams(m.passphrase, salt, m.kdfParams)
	defer ZeroBytes(key)

	encKey, nonce, err := encrypt(key, []byte(cred.APIKey))
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	// Reuse the same derived key but generate fresh nonces for each field.
	// All fields share the same salt (one key derivation per credential)
	// but each field gets its own nonce for GCM uniqueness.
	encSecret, nonceSecret, err := encrypt(key, []byte(cred.APISecret))
	if err != nil {
		return fmt.Errorf("encrypting API secret: %w", err)
	}

	var encPassphrase []byte
	var noncePassphrase []byte
	if cred.Passphrase != "" {
		encPassphrase, noncePassphrase, err = encrypt(key, []byte(cred.Passphrase))
		if err != nil {
			return fmt.Errorf("encrypting passphrase: %w", err)
		}
	}

	// Pack multiple nonces: key_nonce | secret_nonce | passphrase_nonce
	// Each is exactly nonceLen bytes. If passphrase is absent, only 2 nonces.
	combinedNonce := make([]byte, 0, nonceLen*3)
	combinedNonce = append(combinedNonce, nonce...)
	combinedNonce = append(combinedNonce, nonceSecret...)
	if noncePassphrase != nil {
		combinedNonce = append(combinedNonce, noncePassphrase...)
	}

	now := time.Now().UTC()
	row := &store.CredentialRow{
		VenueID:             cred.VenueID,
		EncryptedAPIKey:     encKey,
		EncryptedAPISecret:  encSecret,
		EncryptedPassphrase: encPassphrase,
		Salt:                salt,
		Nonce:               combinedNonce,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := m.store.StoreCredential(ctx, row); err != nil {
		return fmt.Errorf("persisting credential: %w", err)
	}
	return nil
}

// Retrieve loads and decrypts a credential for the given venue.
func (m *CredentialManager) Retrieve(ctx context.Context, venueID string) (*domain.VenueCredential, error) {
	row, err := m.store.GetCredential(ctx, venueID)
	if err != nil {
		return nil, fmt.Errorf("loading credential: %w", err)
	}
	if row == nil {
		return nil, ErrNotFound
	}

	key := deriveKeyWithParams(m.passphrase, row.Salt, m.kdfParams)
	defer ZeroBytes(key)

	// Unpack nonces.
	if len(row.Nonce) < nonceLen*2 {
		return nil, errors.New("invalid nonce data: too short")
	}
	nonceKey := row.Nonce[:nonceLen]
	nonceSecret := row.Nonce[nonceLen : nonceLen*2]

	apiKey, err := decrypt(key, row.EncryptedAPIKey, nonceKey)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	apiSecret, err := decrypt(key, row.EncryptedAPISecret, nonceSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypting API secret: %w", err)
	}

	var passphrase string
	if len(row.EncryptedPassphrase) > 0 {
		if len(row.Nonce) < nonceLen*3 {
			return nil, errors.New("invalid nonce data: passphrase nonce missing")
		}
		noncePassphrase := row.Nonce[nonceLen*2 : nonceLen*3]
		plain, err := decrypt(key, row.EncryptedPassphrase, noncePassphrase)
		if err != nil {
			return nil, fmt.Errorf("decrypting passphrase: %w", err)
		}
		passphrase = string(plain)
	}

	return &domain.VenueCredential{
		VenueID:    row.VenueID,
		APIKey:     string(apiKey),
		APISecret:  string(apiSecret),
		Passphrase: passphrase,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}

// Delete removes a credential from the store.
func (m *CredentialManager) Delete(ctx context.Context, venueID string) error {
	return m.store.DeleteCredential(ctx, venueID)
}

// HasCredential checks whether a credential exists for the given venue ID
// without loading or decrypting the credential data.
func (m *CredentialManager) HasCredential(ctx context.Context, venueID string) (bool, error) {
	return m.store.HasCredential(ctx, venueID)
}

// ValidateAll attempts to decrypt every credential in the provided list of
// venue IDs. It returns a map of venueID to error; a nil error means the
// credential decrypted successfully.
func (m *CredentialManager) ValidateAll(ctx context.Context, venueIDs []string) map[string]error {
	results := make(map[string]error, len(venueIDs))
	for _, id := range venueIDs {
		_, err := m.Retrieve(ctx, id)
		results[id] = err
	}
	return results
}

// ListVenueIDs returns all venue IDs that have stored credentials.
func (m *CredentialManager) ListVenueIDs(ctx context.Context) ([]string, error) {
	return m.store.ListVenueIDs(ctx)
}

// RotatePassphrase re-encrypts all credentials with a new passphrase.
// It decrypts each credential with the old passphrase, re-encrypts with
// the new passphrase, and updates the store.
func (m *CredentialManager) RotatePassphrase(ctx context.Context, oldPassphrase, newPassphrase string) error {
	if oldPassphrase != m.passphrase {
		return ErrPassphraseMismatch
	}
	if newPassphrase == "" {
		return ErrEmptyPassphrase
	}

	venueIDs, err := m.store.ListVenueIDs(ctx)
	if err != nil {
		return fmt.Errorf("listing venue IDs: %w", err)
	}

	for _, venueID := range venueIDs {
		if err := m.rotateCredential(ctx, venueID, newPassphrase); err != nil {
			return fmt.Errorf("rotating credential for %s: %w", venueID, err)
		}
	}

	m.passphrase = newPassphrase
	return nil
}

// rotateCredential decrypts a single credential with the current passphrase
// and re-encrypts it with the new passphrase.
func (m *CredentialManager) rotateCredential(ctx context.Context, venueID, newPassphrase string) error {
	cred, err := m.Retrieve(ctx, venueID)
	if err != nil {
		return fmt.Errorf("retrieving: %w", err)
	}

	salt, err := generateSalt()
	if err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	newKey := deriveKeyWithParams(newPassphrase, salt, m.kdfParams)
	defer ZeroBytes(newKey)

	encKey, nonce, err := encrypt(newKey, []byte(cred.APIKey))
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	encSecret, nonceSecret, err := encrypt(newKey, []byte(cred.APISecret))
	if err != nil {
		return fmt.Errorf("encrypting API secret: %w", err)
	}

	var encPassphrase, noncePassphrase []byte
	if cred.Passphrase != "" {
		encPassphrase, noncePassphrase, err = encrypt(newKey, []byte(cred.Passphrase))
		if err != nil {
			return fmt.Errorf("encrypting passphrase: %w", err)
		}
	}

	combinedNonce := make([]byte, 0, nonceLen*3)
	combinedNonce = append(combinedNonce, nonce...)
	combinedNonce = append(combinedNonce, nonceSecret...)
	if noncePassphrase != nil {
		combinedNonce = append(combinedNonce, noncePassphrase...)
	}

	now := time.Now().UTC()
	row := &store.CredentialRow{
		VenueID:             venueID,
		EncryptedAPIKey:     encKey,
		EncryptedAPISecret:  encSecret,
		EncryptedPassphrase: encPassphrase,
		Salt:                salt,
		Nonce:               combinedNonce,
		CreatedAt:           cred.CreatedAt,
		UpdatedAt:           now,
	}

	return m.store.StoreCredential(ctx, row)
}
