package credential

import (
	"context"
	"sync"
	"testing"

	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/store"
)

// ---------- in-memory mock store ----------

type memStore struct {
	mu   sync.RWMutex
	rows map[string]*store.CredentialRow
}

func newMemStore() *memStore {
	return &memStore{rows: make(map[string]*store.CredentialRow)}
}

func (s *memStore) StoreCredential(_ context.Context, cred *store.CredentialRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Deep-copy byte slices to avoid aliasing issues.
	cp := *cred
	cp.EncryptedAPIKey = clone(cred.EncryptedAPIKey)
	cp.EncryptedAPISecret = clone(cred.EncryptedAPISecret)
	cp.EncryptedPassphrase = clone(cred.EncryptedPassphrase)
	cp.Salt = clone(cred.Salt)
	cp.Nonce = clone(cred.Nonce)
	s.rows[cred.VenueID] = &cp
	return nil
}

func (s *memStore) GetCredential(_ context.Context, venueID string) (*store.CredentialRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rows[venueID]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (s *memStore) DeleteCredential(_ context.Context, venueID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, venueID)
	return nil
}

func (s *memStore) HasCredential(_ context.Context, venueID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.rows[venueID]
	return ok, nil
}

func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// ---------- tests ----------

func TestRoundTrip(t *testing.T) {
	ms := newMemStore()
	mgr, err := NewCredentialManager("test-passphrase", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager: %v", err)
	}

	cred := domain.VenueCredential{
		VenueID:    "binance",
		APIKey:     "ak-12345",
		APISecret:  "secret-67890",
		Passphrase: "optional-pass",
	}

	ctx := context.Background()
	if err := mgr.Store(ctx, cred); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := mgr.Retrieve(ctx, "binance")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if got.VenueID != cred.VenueID {
		t.Errorf("VenueID = %q, want %q", got.VenueID, cred.VenueID)
	}
	if got.APIKey != cred.APIKey {
		t.Errorf("APIKey = %q, want %q", got.APIKey, cred.APIKey)
	}
	if got.APISecret != cred.APISecret {
		t.Errorf("APISecret = %q, want %q", got.APISecret, cred.APISecret)
	}
	if got.Passphrase != cred.Passphrase {
		t.Errorf("Passphrase = %q, want %q", got.Passphrase, cred.Passphrase)
	}
}

func TestRoundTripWithoutPassphrase(t *testing.T) {
	ms := newMemStore()
	mgr, err := NewCredentialManager("test-passphrase", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager: %v", err)
	}

	cred := domain.VenueCredential{
		VenueID:   "alpaca",
		APIKey:    "ak-alpaca",
		APISecret: "secret-alpaca",
		// No passphrase
	}

	ctx := context.Background()
	if err := mgr.Store(ctx, cred); err != nil {
		t.Fatalf("Store: %v", err)
	}

	got, err := mgr.Retrieve(ctx, "alpaca")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if got.APIKey != cred.APIKey {
		t.Errorf("APIKey = %q, want %q", got.APIKey, cred.APIKey)
	}
	if got.APISecret != cred.APISecret {
		t.Errorf("APISecret = %q, want %q", got.APISecret, cred.APISecret)
	}
	if got.Passphrase != "" {
		t.Errorf("Passphrase = %q, want empty", got.Passphrase)
	}
}

func TestWrongPassphrase(t *testing.T) {
	ms := newMemStore()
	mgrA, err := NewCredentialManager("passphrase-A", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager A: %v", err)
	}

	ctx := context.Background()
	if err := mgrA.Store(ctx, domain.VenueCredential{
		VenueID:   "coinbase",
		APIKey:    "ak-coinbase",
		APISecret: "secret-coinbase",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	mgrB, err := NewCredentialManager("passphrase-B", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager B: %v", err)
	}

	_, err = mgrB.Retrieve(ctx, "coinbase")
	if err == nil {
		t.Fatal("expected decryption error with wrong passphrase, got nil")
	}
}

func TestDelete(t *testing.T) {
	ms := newMemStore()
	mgr, err := NewCredentialManager("test-passphrase", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager: %v", err)
	}

	ctx := context.Background()
	if err := mgr.Store(ctx, domain.VenueCredential{
		VenueID:   "kraken",
		APIKey:    "ak-kraken",
		APISecret: "secret-kraken",
	}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := mgr.Delete(ctx, "kraken"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := mgr.Retrieve(ctx, "kraken")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got err=%v, cred=%v", err, got)
	}
}

func TestValidateAll(t *testing.T) {
	ms := newMemStore()
	mgr, err := NewCredentialManager("shared-pass", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager: %v", err)
	}

	ctx := context.Background()

	// Store two credentials with the same manager.
	for _, id := range []string{"venue-a", "venue-b"} {
		if err := mgr.Store(ctx, domain.VenueCredential{
			VenueID:   id,
			APIKey:    "key-" + id,
			APISecret: "secret-" + id,
		}); err != nil {
			t.Fatalf("Store %s: %v", id, err)
		}
	}

	// ValidateAll with the correct manager should return no errors.
	results := mgr.ValidateAll(ctx, []string{"venue-a", "venue-b"})
	for id, e := range results {
		if e != nil {
			t.Errorf("ValidateAll (correct pass) %s: unexpected error: %v", id, e)
		}
	}

	// ValidateAll with wrong passphrase should return errors for both.
	badMgr, err := NewCredentialManager("wrong-pass", ms)
	if err != nil {
		t.Fatalf("NewCredentialManager bad: %v", err)
	}
	badResults := badMgr.ValidateAll(ctx, []string{"venue-a", "venue-b"})
	for id, e := range badResults {
		if e == nil {
			t.Errorf("ValidateAll (wrong pass) %s: expected error, got nil", id)
		}
	}
}

func TestNewCredentialManagerValidation(t *testing.T) {
	_, err := NewCredentialManager("", newMemStore())
	if err == nil {
		t.Error("expected error for empty passphrase")
	}

	_, err = NewCredentialManager("ok", nil)
	if err == nil {
		t.Error("expected error for nil store")
	}
}
