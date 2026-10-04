package bootstrap

import (
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/workers"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/scanhandoff"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/secrets"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
)

// Services is the composition root's container: everything the application wires up.
// Handler constructors take the individual services they need, not this struct.
type Services struct {
	DB                      *sql.DB
	SignInManager           auth.SignInManager
	EncryptionService       encryption.EncryptionService
	SecretRepository        secrets.SecretRepository
	UserRepository          auth.UserRepository
	SignInHistoryRepository auth.SignInHistoryItemRepository
	MekStore                auth.MekStore
	SecretManager           secrets.SecretManager
	UserManager             auth.UserManager
	BackupService           backup.BackupService
	BackupWorker            workers.BackupWorker
	Logger                  ccc.Logger
	TagManager              documents.TagManager
	DocumentManager         documents.DocumentManager
	DocumentFileManager     documents.DocumentFileManager
	DocumentSearchEngine    documents.DocumentSearchEngine
	DocumentListService     documents.DocumentListService
	NoteManager             documents.NoteManager
	ScanHandoffService      scanhandoff.ScanHandoffService
	UpdateChecker           updates.UpdateChecker
	UpdateCheckWorker       workers.UpdateCheckWorker
}
