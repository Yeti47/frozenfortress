package main

import (
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/secrets"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/workers"
)

type services struct {
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
}

// configureServices configures the services used by the web UI.
func configureServices(config ccc.AppConfig, db *sql.DB) services {

	logger := ccc.CreateLogger(config)

	userRepo, err := auth.NewSQLiteUserRepository(db)
	if err != nil {
		logger.Error("Failed to create user repository", "error", err)
		panic("Failed to create user repository: " + err.Error())
	}

	signInHistoryRepo, err := auth.NewSQLiteSignInHistoryItemRepository(db)
	if err != nil {
		logger.Error("Failed to create sign-in history repository", "error", err)
		panic("Failed to create sign-in history repository: " + err.Error())
	}

	secretRepo, err := secrets.NewSQLiteSecretRepository(db)
	if err != nil {
		logger.Error("Failed to create secret repository", "error", err)
		panic("Failed to create secret repository: " + err.Error())
	}

	encryptionService := encryption.NewDefaultEncryptionService()

	securityService := auth.NewDefaultSecurityService(userRepo, encryptionService, logger)

	signInHandler := auth.NewDefaultSignInHandler(
		userRepo,
		signInHistoryRepo,
		securityService,
		encryptionService,
		config,
		logger,
	)

	keyProvider := auth.NewConfigSessionKeyProvider(config, encryptionService)

	redisStore, err := auth.CreateRedisStore(config, keyProvider, logger)
	if err != nil {
		logger.Error("Failed to create Redis store", "error", err)
		panic("Failed to create Redis store: " + err.Error())
	}

	mekStore := auth.NewSessionMekStore(redisStore, logger)

	signInManager := auth.NewSessionSignInManager(
		userRepo,
		signInHandler,
		redisStore,
		mekStore,
		logger,
	)

	idGenerator := ccc.NewUuidGenerator()

	secretManager := secrets.NewDefaultSecretManager(
		secretRepo,
		idGenerator,
		userRepo,
		logger,
	)

	userManager := auth.NewDefaultUserManager(
		userRepo,
		idGenerator,
		encryptionService,
		securityService,
		logger,
	)

	// Create backup service
	backupService := backup.NewFileBasedBackupService(config, logger)

	// Create backup worker
	backupWorker := workers.NewDefaultBackupWorker(backupService, config, logger)

	return services{
		SignInManager:           signInManager,
		EncryptionService:       encryptionService,
		SecretRepository:        secretRepo,
		UserRepository:          userRepo,
		SignInHistoryRepository: signInHistoryRepo,
		MekStore:                mekStore,
		SecretManager:           secretManager,
		UserManager:             userManager,
		BackupService:           backupService,
		BackupWorker:            backupWorker,
		Logger:                  logger,
	}
}
