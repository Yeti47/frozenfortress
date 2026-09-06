package scanhandoff

import (
	"context"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// handoffTTL is how long a pending handoff, and a completed-but-unfetched scan,
// stays valid before expiring.
const handoffTTL = 5 * time.Minute

// DefaultScanHandoffService implements ScanHandoffService.
type DefaultScanHandoffService struct {
	store             ScanHandoffStore
	encryptionService encryption.EncryptionService
	logger            ccc.Logger
}

// NewDefaultScanHandoffService creates a new DefaultScanHandoffService.
func NewDefaultScanHandoffService(store ScanHandoffStore, encryptionService encryption.EncryptionService, logger ccc.Logger) *DefaultScanHandoffService {
	if logger == nil {
		logger = ccc.NopLogger
	}

	return &DefaultScanHandoffService{
		store:             store,
		encryptionService: encryptionService,
		logger:            logger,
	}
}

// StartHandoff creates a new pending handoff owned by userId and returns its token.
func (s *DefaultScanHandoffService) StartHandoff(ctx context.Context, userId string) (string, error) {
	if userId == "" {
		return "", ccc.NewInvalidInputError("userId", "must not be empty")
	}

	token, err := GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate scan handoff token", "error", err)
		return "", ccc.NewInternalError("failed to generate scan handoff token", err)
	}

	record := &StagedScan{
		Token:     token,
		UserId:    userId,
		State:     ScanHandoffStatePending,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.store.Save(ctx, record, handoffTTL); err != nil {
		s.logger.Error("Failed to save new scan handoff", "token", token, "error", err)
		return "", ccc.NewInternalError("failed to start scan handoff", err)
	}

	s.logger.Info("Scan handoff started", "token", token, "user_id", userId)

	return token, nil
}

// UploadScan attaches the companion app's encrypted scan to a pending handoff.
func (s *DefaultScanHandoffService) UploadScan(ctx context.Context, token, fileName string, cipherBlob []byte) error {
	if token == "" {
		return ccc.NewInvalidInputError("token", "must not be empty")
	}
	if len(cipherBlob) == 0 {
		return ccc.NewInvalidInputError("cipherBlob", "must not be empty")
	}

	record, err := s.store.Get(ctx, token)
	if err != nil {
		s.logger.Error("Failed to look up scan handoff for upload", "token", token, "error", err)
		return ccc.NewInternalError("failed to look up scan handoff", err)
	}
	if record == nil {
		s.logger.Warn("Scan upload attempted for an unknown or expired token", "token", token)
		return ccc.NewResourceNotFoundError(token, "scan handoff")
	}
	if record.State != ScanHandoffStatePending {
		// Enforces single-use: a handoff that already has a scan attached cannot receive another.
		s.logger.Warn("Scan upload attempted for a handoff that is not pending", "token", token, "state", record.State)
		return ccc.NewResourceAlreadyExistsError(token, "scan handoff upload")
	}

	record.FileName = fileName
	record.CipherBlob = cipherBlob
	record.State = ScanHandoffStateReady

	if err := s.store.Save(ctx, record, handoffTTL); err != nil {
		s.logger.Error("Failed to save uploaded scan", "token", token, "error", err)
		return ccc.NewInternalError("failed to save uploaded scan", err)
	}

	s.logger.Info("Scan uploaded for handoff", "token", token)

	return nil
}

// GetStatus returns the current state of a handoff owned by userId.
func (s *DefaultScanHandoffService) GetStatus(ctx context.Context, token, userId string) (ScanHandoffState, error) {
	record, err := s.getOwnedRecord(ctx, token, userId)
	if err != nil {
		return "", err
	}

	return record.State, nil
}

// FetchAndConsume decrypts and returns the staged scan, then deletes the record.
func (s *DefaultScanHandoffService) FetchAndConsume(ctx context.Context, token, userId, key string) (string, []byte, error) {
	record, err := s.getOwnedRecord(ctx, token, userId)
	if err != nil {
		return "", nil, err
	}
	if record.State != ScanHandoffStateReady {
		s.logger.Warn("Scan fetch attempted before upload completed", "token", token)
		return "", nil, ccc.NewResourceNotFoundError(token, "scan handoff")
	}

	plainData, err := s.encryptionService.DecryptBytes(record.CipherBlob, key)
	if err != nil {
		s.logger.Error("Failed to decrypt staged scan", "token", token, "error", err)
		return "", nil, ccc.NewOperationFailedError("decrypt scan", "the scan could not be decrypted")
	}

	if err := s.store.Delete(ctx, token); err != nil {
		// The scan was already decrypted and is about to be returned to the caller;
		// log but don't fail the request over a best-effort cleanup step.
		s.logger.Error("Failed to delete consumed scan handoff", "token", token, "error", err)
	}

	s.logger.Info("Scan handoff fetched and consumed", "token", token)

	return record.FileName, plainData, nil
}

// getOwnedRecord fetches the record for token and verifies it is owned by userId,
// returning a not-found error for both missing and mismatched-owner cases so that
// token existence isn't confirmed to the wrong session.
func (s *DefaultScanHandoffService) getOwnedRecord(ctx context.Context, token, userId string) (*StagedScan, error) {
	record, err := s.store.Get(ctx, token)
	if err != nil {
		s.logger.Error("Failed to look up scan handoff", "token", token, "error", err)
		return nil, ccc.NewInternalError("failed to look up scan handoff", err)
	}
	if record == nil || record.UserId != userId {
		return nil, ccc.NewResourceNotFoundError(token, "scan handoff")
	}

	return record, nil
}
