// Package store provides Postgres-backed storage for API keys and tenants.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

// ErrKeyNotFound is returned when the key does not exist or has been revoked.
var ErrKeyNotFound = errors.New("key not found")

// KeyRecord holds the stored metadata for an API key (never the plaintext).
type KeyRecord struct {
	ID         string
	TenantID   string
	Prefix     string
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// KeyStore manages API key lifecycle backed by Postgres.
type KeyStore struct {
	pool *pgxpool.Pool
}

// New creates a KeyStore using the given connection pool.
func New(pool *pgxpool.Pool) *KeyStore {
	return &KeyStore{pool: pool}
}

// CreateKey generates a new API key, persists the hash, and returns the
// plaintext exactly once. The plaintext is never stored.
//
// Flow:
//  1. Generate tenant_id (ULID) and key_id (ULID).
//  2. Generate 32 random bytes; base62-encode them with "ev1_" prefix.
//  3. Compute SHA-256(plaintext) for storage.
//  4. INSERT tenant and api_key rows in a transaction.
//  5. Return plaintext, prefix, and tenant_id.
func (s *KeyStore) CreateKey(ctx context.Context, label string) (plaintext string, rec KeyRecord, err error) {
	tenantID := ulid.Make().String()
	keyID := ulid.Make().String()

	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: rand: %w", err)
	}

	plaintext = "ev1_" + base62Encode(raw)
	prefix := plaintext[:12] // "ev1_" + first 8 base62 chars
	hash := sha256.Sum256([]byte(plaintext))
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx,
		`INSERT INTO tenants (id, label, created_at) VALUES ($1, $2, $3)`,
		tenantID, label, now,
	)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: insert tenant: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO api_keys (id, tenant_id, key_hash, prefix, label, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		keyID, tenantID, hash[:], prefix, label, now,
	)
	if err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: insert key: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", KeyRecord{}, fmt.Errorf("store.CreateKey: commit: %w", err)
	}

	rec = KeyRecord{
		ID:        keyID,
		TenantID:  tenantID,
		Prefix:    prefix,
		Label:     label,
		CreatedAt: now,
	}
	return plaintext, rec, nil
}

// LookupKey hashes the plaintext and looks up the corresponding active key record.
// Returns (tenantID, keyID, prefix) for the matching key, or ErrKeyNotFound if
// the key does not exist or has been revoked.
func (s *KeyStore) LookupKey(ctx context.Context, plaintext string) (tenantID, keyID, prefix string, err error) {
	hash := sha256.Sum256([]byte(plaintext))

	err = s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, prefix
		 FROM api_keys
		 WHERE key_hash = $1 AND revoked_at IS NULL`,
		hash[:],
	).Scan(&keyID, &tenantID, &prefix)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", ErrKeyNotFound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("store.LookupKey: %w", err)
	}
	return tenantID, keyID, prefix, nil
}

// TouchKey updates last_used_at for the given key ID.
// Called asynchronously after successful auth; errors are logged, not returned.
func (s *KeyStore) TouchKey(keyID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := s.pool.Exec(ctx,
			`UPDATE api_keys SET last_used_at = now() WHERE id = $1`,
			keyID,
		); err != nil {
			slog.Warn("store.TouchKey: update failed", "key_id", keyID, "error", err)
		}
	}()
}

// base62Encode encodes bytes using the base62 alphabet [0-9A-Za-z].
func base62Encode(b []byte) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// Each byte contributes ~1.34 base62 chars; 32 bytes → ~43 chars.
	result := make([]byte, 0, 44)
	// Process as a big integer via repeated division.
	// Use a simple byte-at-a-time approach for correctness.
	n := make([]byte, len(b))
	copy(n, b)
	for leading := true; ; {
		if allZero(n) {
			break
		}
		remainder := 0
		for i := range n {
			val := remainder*256 + int(n[i])
			n[i] = byte(val / 62)
			remainder = val % 62
		}
		result = append(result, alphabet[remainder])
		if leading {
			leading = false
		}
	}
	// Reverse.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
