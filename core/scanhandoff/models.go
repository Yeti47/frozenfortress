package scanhandoff

import (
	"time"
)

// ScanHandoffState represents the lifecycle state of a staged scan handoff.
type ScanHandoffState string

const (
	// ScanHandoffStatePending indicates a handoff has been started but no scan has been uploaded yet.
	ScanHandoffStatePending ScanHandoffState = "pending"
	// ScanHandoffStateReady indicates a scan has been uploaded and is waiting to be fetched.
	ScanHandoffStateReady ScanHandoffState = "ready"
)

// StagedScan represents a single scan handoff record.
// CipherBlob is the companion app's AES-256-GCM output (12-byte nonce followed by the
// sealed ciphertext) and is opaque to the server: the encryption key never reaches it,
// so the server can persist this record without ever holding the plaintext scan.
type StagedScan struct {
	Token      string
	UserId     string
	State      ScanHandoffState
	FileName   string
	CipherBlob []byte
	CreatedAt  time.Time
}
