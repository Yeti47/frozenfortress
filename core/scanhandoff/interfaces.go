package scanhandoff

import (
	"context"
	"time"
)

// ScanHandoffStore persists StagedScan records with a TTL. It is a plain CRUD store -
// business rules (single-use, ownership, state transitions) live in ScanHandoffService.
// Implementations must not interpret CipherBlob in any way - it is opaque, encrypted data.
type ScanHandoffStore interface {
	// Save writes or overwrites the record for record.Token, expiring after ttl.
	Save(ctx context.Context, record *StagedScan, ttl time.Duration) error

	// Get returns the record for token, or nil if it doesn't exist or has expired.
	Get(ctx context.Context, token string) (*StagedScan, error)

	// Delete removes the record for token. It is not an error if the record doesn't exist.
	Delete(ctx context.Context, token string) error
}

// ScanKeyStore persists the one-time scan handoff encryption key with its own native
// TTL, entirely separate from ScanHandoffStore's ciphertext record - the two are kept in
// different Redis keys so that no single record ever holds both the key and what it
// decrypts. Unlike the MEK, this key is not tied to any particular browser session or
// cookie: the token is already the bearer credential for the handoff, and ownership is
// enforced by ScanHandoffService against the caller's authenticated user, not by which
// session originally stored the key.
type ScanKeyStore interface {
	// Store saves key for token, expiring after ttl.
	Store(ctx context.Context, token, key string, ttl time.Duration) error

	// Retrieve returns the key for token, or "" if it doesn't exist or has expired.
	Retrieve(ctx context.Context, token string) (string, error)

	// Refresh extends token's expiry to ttl without needing to know its current value.
	// Used to keep the key's expiry in step with the ciphertext record's own refreshed
	// TTL after upload, without the uploader (which never has the key) needing to resend it.
	Refresh(ctx context.Context, token string, ttl time.Duration) error

	// Delete removes the key for token. It is not an error if it doesn't exist.
	Delete(ctx context.Context, token string) error
}

// ScanHandoffService coordinates the scan handoff between a browser session and the
// companion app. See core/scanhandoff package docs for the full flow.
type ScanHandoffService interface {
	// StartHandoff creates a new pending handoff owned by userId, stores key (the
	// browser-generated, server-never-persisted-alongside-its-ciphertext AES-256-GCM
	// key) for later decryption, and returns the handoff's token.
	StartHandoff(ctx context.Context, userId, key string) (token string, err error)

	// UploadScan attaches the companion app's encrypted scan to a pending handoff.
	// Fails if the token is unknown, expired, or already has a scan attached (single-use).
	UploadScan(ctx context.Context, token, fileName string, cipherBlob []byte) error

	// GetStatus returns the current state of a handoff owned by userId.
	// Returns a not-found error if the token is unknown, expired, or owned by a different user.
	GetStatus(ctx context.Context, token, userId string) (ScanHandoffState, error)

	// FetchAndConsume decrypts and returns the staged scan, then deletes both the
	// ciphertext record and the key so it can only be fetched once. Returns a not-found
	// error under the same conditions as GetStatus.
	FetchAndConsume(ctx context.Context, token, userId string) (fileName string, plainData []byte, err error)
}
