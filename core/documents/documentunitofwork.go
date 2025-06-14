package documents

import (
	"context"
	"database/sql"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// DefaultDocumentUnitOfWork implements DocumentUnitOfWork interface.
// It acts as both a transaction manager and a repository factory.
type DefaultDocumentUnitOfWork struct {
	db *sql.DB
	tx *sql.Tx

	// Cached repository instances - created lazily
	documentRepo     DocumentRepository
	documentFileRepo DocumentFileRepository
	metadataRepo     DocumentFileMetadataRepository
	tagRepo          TagRepository
	documentTagRepo  DocumentTagRepository
}

// NewDocumentUnitOfWork creates a new DefaultDocumentUnitOfWork instance.
func NewDocumentUnitOfWork(db *sql.DB) *DefaultDocumentUnitOfWork {
	return &DefaultDocumentUnitOfWork{
		db: db,
	}
}

// Begin starts a new database transaction.
func (uow *DefaultDocumentUnitOfWork) Begin(ctx context.Context) error {
	if uow.tx != nil {
		return ccc.NewOperationFailedError("begin transaction", "transaction already active")
	}

	tx, err := uow.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	uow.tx = tx

	// Clear cached repositories so they get recreated with transaction
	uow.clearRepositoryCache()

	return nil
}

// Commit commits the current transaction.
func (uow *DefaultDocumentUnitOfWork) Commit(ctx context.Context) error {
	if !uow.IsTransactionActive() {
		return ccc.NewOperationFailedError("commit transaction", "no active transaction to commit")
	}

	err := uow.tx.Commit()
	uow.cleanup()
	return err
}

// Rollback rolls back the current transaction.
func (uow *DefaultDocumentUnitOfWork) Rollback(ctx context.Context) error {
	if !uow.IsTransactionActive() {
		return ccc.NewOperationFailedError("rollback transaction", "no active transaction to rollback")
	}

	err := uow.tx.Rollback()
	uow.cleanup()
	return err
}

// DocumentRepo returns a DocumentRepository instance.
// If a transaction is active, the repository will use the transaction.
// Otherwise, it will use the regular database connection.
func (uow *DefaultDocumentUnitOfWork) DocumentRepo() DocumentRepository {
	if uow.documentRepo == nil {
		executor := uow.getExecutor()
		uow.documentRepo = newSQLiteDocumentRepository(executor)
	}
	return uow.documentRepo
}

// DocumentFileRepo returns a DocumentFileRepository instance.
func (uow *DefaultDocumentUnitOfWork) DocumentFileRepo() DocumentFileRepository {
	if uow.documentFileRepo == nil {
		executor := uow.getExecutor()
		uow.documentFileRepo = newSQLiteDocumentFileRepository(executor)
	}
	return uow.documentFileRepo
}

// DocumentFileMetadataRepo returns a DocumentFileMetadataRepository instance.
func (uow *DefaultDocumentUnitOfWork) DocumentFileMetadataRepo() DocumentFileMetadataRepository {
	if uow.metadataRepo == nil {
		executor := uow.getExecutor()
		uow.metadataRepo = newSQLiteDocumentFileMetadataRepository(executor)
	}
	return uow.metadataRepo
}

// TagRepo returns a TagRepository instance.
func (uow *DefaultDocumentUnitOfWork) TagRepo() TagRepository {
	if uow.tagRepo == nil {
		executor := uow.getExecutor()
		uow.tagRepo = newSQLiteTagRepository(executor)
	}
	return uow.tagRepo
}

// DocumentTagRepo returns a DocumentTagRepository instance.
func (uow *DefaultDocumentUnitOfWork) DocumentTagRepo() DocumentTagRepository {
	if uow.documentTagRepo == nil {
		executor := uow.getExecutor()
		uow.documentTagRepo = newSQLiteDocumentTagRepository(executor)
	}
	return uow.documentTagRepo
}

// getExecutor returns the appropriate database executor.
// If a transaction is active, it returns the transaction.
// Otherwise, it returns the regular database connection.
func (uow *DefaultDocumentUnitOfWork) getExecutor() ccc.DBExecutor {
	if uow.tx != nil {
		return uow.tx
	}
	return uow.db
}

// clearRepositoryCache clears all cached repository instances.
// This forces them to be recreated with the new executor (transaction or database).
func (uow *DefaultDocumentUnitOfWork) clearRepositoryCache() {
	uow.documentRepo = nil
	uow.documentFileRepo = nil
	uow.metadataRepo = nil
	uow.tagRepo = nil
	uow.documentTagRepo = nil
}

// cleanup resets the transaction state and clears repository cache.
func (uow *DefaultDocumentUnitOfWork) cleanup() {
	uow.tx = nil
	uow.clearRepositoryCache()
}

// IsTransactionActive returns true if a transaction is currently active.
func (uow *DefaultDocumentUnitOfWork) IsTransactionActive() bool {
	return uow.tx != nil
}

// Execute runs a function within this Unit of Work's transaction context
func (uow *DefaultDocumentUnitOfWork) Execute(ctx context.Context, fn func(uow DocumentUnitOfWork) error) error {
	if !uow.IsTransactionActive() {
		if err := uow.Begin(ctx); err != nil {
			return err
		}
		defer func() {
			if r := recover(); r != nil {
				uow.Rollback(ctx)
				panic(r)
			}
		}()

		if err := fn(uow); err != nil {
			uow.Rollback(ctx)
			return err
		}

		return uow.Commit(ctx)
	}

	// Already in transaction, just execute
	return fn(uow)
}

// ExecuteWithResult runs a function within this Unit of Work's transaction context and returns a result
func (uow *DefaultDocumentUnitOfWork) ExecuteWithResult(ctx context.Context, fn func(uow DocumentUnitOfWork) (interface{}, error)) (interface{}, error) {
	var result any

	if !uow.IsTransactionActive() {
		if err := uow.Begin(ctx); err != nil {
			return nil, err
		}
		defer func() {
			if r := recover(); r != nil {
				uow.Rollback(ctx)
				panic(r)
			}
		}()

		var err error
		result, err = fn(uow)
		if err != nil {
			uow.Rollback(ctx)
			return nil, err
		}

		if err := uow.Commit(ctx); err != nil {
			return nil, err
		}

		return result, nil
	}

	// Already in transaction, just execute
	return fn(uow)
}

// DefaultDocumentUnitOfWorkFactory implements DocumentUnitOfWorkFactory interface.
type DefaultDocumentUnitOfWorkFactory struct {
	db *sql.DB
}

// NewDocumentUnitOfWorkFactory creates a new DefaultDocumentUnitOfWorkFactory instance.
func NewDocumentUnitOfWorkFactory(db *sql.DB) *DefaultDocumentUnitOfWorkFactory {
	return &DefaultDocumentUnitOfWorkFactory{
		db: db,
	}
}

// Create creates a new DocumentUnitOfWork instance.
func (factory *DefaultDocumentUnitOfWorkFactory) Create() DocumentUnitOfWork {
	return NewDocumentUnitOfWork(factory.db)
}
