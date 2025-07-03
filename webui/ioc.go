package main

import (
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/documents"
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
	TagManager              documents.TagManager
	DocumentManager         documents.DocumentManager
	DocumentFileManager     documents.DocumentFileManager
	DocumentSearchEngine    documents.DocumentSearchEngine
	DocumentListService     documents.DocumentListService
	NoteManager             documents.NoteManager
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

	// Create document unit of work factory and tag manager
	uowFactory := documents.NewDocumentUnitOfWorkFactory(db)
	tagManager := documents.NewDefaultTagManager(uowFactory, idGenerator, logger)

	// Create document file processor factory
	pdfProcessor := documents.NewPDFFileProcessor()
	ocrService := createOCRService(config, logger)
	imageProcessor := documents.NewImageFileProcessor(ocrService)
	processorFactory := documents.NewDefaultDocumentFileProcessorFactory(pdfProcessor, imageProcessor)

	// Create document file creator
	fileCreator := documents.NewDefaultDocumentFileCreator(idGenerator, processorFactory, logger)

	// Create document sorter
	documentSorter := documents.NewDefaultDocumentSorter[*documents.DocumentDetails]()

	// Create document manager
	documentManager := documents.NewDefaultDocumentManager(uowFactory, idGenerator, fileCreator, logger, documentSorter)

	// Create document file manager
	documentFileManager := documents.NewDefaultDocumentFileManager(uowFactory, fileCreator, logger)

	// Create document search engine
	searchSorter := documents.NewSearchDocumentSorter()
	documentSearchEngine := documents.NewDefaultDocumentSearchEngine(uowFactory, logger, searchSorter)

	// Create document list service (facade)
	documentListService := documents.NewDefaultDocumentListService(documentManager, documentSearchEngine, logger)

	// Create note manager
	noteManager := documents.NewDefaultNoteManager(uowFactory, idGenerator, logger)

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
		TagManager:              tagManager,
		DocumentManager:         documentManager,
		DocumentFileManager:     documentFileManager,
		DocumentSearchEngine:    documentSearchEngine,
		DocumentListService:     documentListService,
		NoteManager:             noteManager,
	}
}
