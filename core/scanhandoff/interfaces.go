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

// ScanHandoffService coordinates the scan handoff between a browser session and the
// companion app. See core/scanhandoff package docs for the full flow.
type ScanHandoffService interface {
	// StartHandoff creates a new pending handoff owned by userId and returns its token.
	StartHandoff(ctx context.Context, userId string) (token string, err error)

	// UploadScan attaches the companion app's encrypted scan to a pending handoff.
	// Fails if the token is unknown, expired, or already has a scan attached (single-use).
	UploadScan(ctx context.Context, token, fileName string, cipherBlob []byte) error

	// GetStatus returns the current state of a handoff owned by userId.
	// Returns a not-found error if the token is unknown, expired, or owned by a different user.
	GetStatus(ctx context.Context, token, userId string) (ScanHandoffState, error)

	// FetchAndConsume decrypts and returns the staged scan using key (the browser-generated,
	// server-never-persisted AES-256-GCM key), then deletes the record so it can only be
	// fetched once. Returns a not-found error under the same conditions as GetStatus.
	FetchAndConsume(ctx context.Context, token, userId, key string) (fileName string, plainData []byte, err error)
}
