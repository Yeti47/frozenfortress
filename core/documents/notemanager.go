package documents

import (
	"context"
	"fmt"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultNoteManager implements NoteManager using a DocumentUnitOfWorkFactory and Logger
// It handles note CRUD operations with logging and error handling

type DefaultNoteManager struct {
	uowFactory  DocumentUnitOfWorkFactory
	idGenerator NoteIdGenerator
	logger      ccc.Logger
}

// NewDefaultNoteManager creates a new DefaultNoteManager
func NewDefaultNoteManager(uowFactory DocumentUnitOfWorkFactory, idGenerator NoteIdGenerator, logger ccc.Logger) *DefaultNoteManager {
	if logger == nil {
		logger = ccc.NopLogger
	}
	return &DefaultNoteManager{
		uowFactory:  uowFactory,
		idGenerator: idGenerator,
		logger:      logger,
	}
}

// validateNoteInput validates the note content according to business rules.
func validateNoteInput(content string) error {
	const maxContentLength = 250

	if content == "" {
		return ccc.NewInvalidInputErrorWithMessage(
			"content",
			"cannot be empty",
			"The note content cannot be empty.",
		)
	}
	if len(content) > maxContentLength {
		return ccc.NewInvalidInputErrorWithMessage(
			"content",
			fmt.Sprintf("must not exceed %d characters", maxContentLength),
			fmt.Sprintf("The note content must not exceed %d characters.", maxContentLength),
		)
	}
	return nil
}

// CreateNote creates a new note for the given request, assigning a generated ID.
// The operation is performed in a transaction scope.
func (m *DefaultNoteManager) CreateNote(ctx context.Context, request CreateNoteRequest, dataProtector dataprotection.DataProtector) (*CreateNoteResponse, error) {
	if err := validateNoteInput(request.Content); err != nil {
		return nil, err
	}

	if request.UserId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if request.DocumentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	// Encrypt the note content
	encryptedContent, err := dataProtector.Protect(request.Content)
	if err != nil {
		m.logger.Error("Failed to encrypt note content", "userId", request.UserId, "documentId", request.DocumentId, "err", err)
		return nil, ccc.NewInternalError("failed to encrypt note content", err)
	}

	now := time.Now()
	uow := m.uowFactory.Create()
	var note *Note
	err = uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		// Verify that the document exists and belongs to the user
		document, err := uow.DocumentRepo().FindById(ctx, request.DocumentId)
		if err != nil {
			m.logger.Error("Failed to find document for note creation", "userId", request.UserId, "documentId", request.DocumentId, "err", err)
			return ccc.NewDatabaseError("find document", err)
		}
		if document == nil || document.UserId != request.UserId {
			m.logger.Warn("Document not found or not owned by user for note creation", "userId", request.UserId, "documentId", request.DocumentId)
			return ccc.NewResourceNotFoundError(request.DocumentId, "Document")
		}

		note = &Note{
			Id:         m.idGenerator.GenerateId(),
			DocumentId: request.DocumentId,
			UserId:     request.UserId,
			Content:    encryptedContent,
			CreatedAt:  now,
			ModifiedAt: now,
		}
		if err := uow.NoteRepo().Add(ctx, note); err != nil {
			m.logger.Error("Failed to create note", "userId", request.UserId, "documentId", request.DocumentId, "err", err)
			return ccc.NewDatabaseError("add note", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.logger.Info("Note created", "userId", request.UserId, "documentId", request.DocumentId, "noteId", note.Id)
	return &CreateNoteResponse{
		NoteId: note.Id,
	}, nil
}

// GetDocumentNotes retrieves all notes for a specific document belonging to the given user.
func (m *DefaultNoteManager) GetDocumentNotes(ctx context.Context, userId, documentId string, dataProtector dataprotection.DataProtector) ([]*NoteDto, error) {
	if userId == "" {
		return nil, ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if documentId == "" {
		return nil, ccc.NewInvalidInputError("documentId", "cannot be empty")
	}

	uow := m.uowFactory.Create()

	// First verify that the document exists and belongs to the user
	document, err := uow.DocumentRepo().FindById(ctx, documentId)
	if err != nil {
		m.logger.Error("Failed to find document for notes retrieval", "userId", userId, "documentId", documentId, "err", err)
		return nil, ccc.NewDatabaseError("find document", err)
	}
	if document == nil || document.UserId != userId {
		m.logger.Warn("Document not found or not owned by user for notes retrieval", "userId", userId, "documentId", documentId)
		return nil, ccc.NewResourceNotFoundError(documentId, "Document")
	}

	// Get all notes for the document
	notes, err := uow.NoteRepo().FindByDocumentId(ctx, documentId)
	if err != nil {
		m.logger.Error("Failed to get document notes", "userId", userId, "documentId", documentId, "err", err)
		return nil, ccc.NewDatabaseError("find document notes", err)
	}

	var dtos []*NoteDto
	for _, note := range notes {
		// Decrypt the note content
		decryptedContent, err := dataProtector.Unprotect(note.Content)
		if err != nil {
			m.logger.Warn("Failed to decrypt note content", "userId", userId, "noteId", note.Id, "err", err)
			// Skip notes that can't be decrypted
			continue
		}

		dtos = append(dtos, &NoteDto{
			Id:         note.Id,
			DocumentId: note.DocumentId,
			Content:    decryptedContent,
			CreatedAt:  note.CreatedAt,
			ModifiedAt: note.ModifiedAt,
		})
	}
	return dtos, nil
}

// UpdateNote updates the content of an existing note for the given user and note ID.
// The operation is performed in a transaction scope.
func (m *DefaultNoteManager) UpdateNote(ctx context.Context, request UpdateNoteRequest, dataProtector dataprotection.DataProtector) error {
	if err := validateNoteInput(request.Content); err != nil {
		return err
	}

	if request.UserId == "" {
		return ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if request.NoteId == "" {
		return ccc.NewInvalidInputError("noteId", "cannot be empty")
	}

	// Encrypt the new note content
	encryptedContent, err := dataProtector.Protect(request.Content)
	if err != nil {
		m.logger.Error("Failed to encrypt note content for update", "userId", request.UserId, "noteId", request.NoteId, "err", err)
		return ccc.NewInternalError("failed to encrypt note content", err)
	}

	uow := m.uowFactory.Create()
	return uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		note, err := uow.NoteRepo().FindById(ctx, request.NoteId)
		if err != nil {
			m.logger.Error("Failed to find note for update", "userId", request.UserId, "noteId", request.NoteId, "err", err)
			return ccc.NewResourceNotFoundError(request.NoteId, "Note")
		}
		if note == nil || note.UserId != request.UserId {
			m.logger.Warn("Note not found or not owned by user for update", "userId", request.UserId, "noteId", request.NoteId)
			return ccc.NewResourceNotFoundError(request.NoteId, "Note")
		}

		note.Content = encryptedContent
		note.ModifiedAt = time.Now()

		if err := uow.NoteRepo().Update(ctx, note); err != nil {
			m.logger.Error("Failed to update note", "userId", request.UserId, "noteId", request.NoteId, "err", err)
			return ccc.NewDatabaseError("update note", err)
		}
		m.logger.Info("Note updated", "userId", request.UserId, "noteId", request.NoteId)
		return nil
	})
}

// DeleteNote deletes a note for the given user and note ID.
// The operation is idempotent and performed in a transaction scope.
func (m *DefaultNoteManager) DeleteNote(ctx context.Context, userId, noteId string) error {
	if userId == "" {
		return ccc.NewInvalidInputError("userId", "cannot be empty")
	}
	if noteId == "" {
		return ccc.NewInvalidInputError("noteId", "cannot be empty")
	}

	uow := m.uowFactory.Create()
	alreadyDeleted := false
	err := uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		note, err := uow.NoteRepo().FindById(ctx, noteId)
		if err != nil {
			m.logger.Error("Failed to find note for delete", "userId", userId, "noteId", noteId, "err", err)
			return ccc.NewResourceNotFoundError(noteId, "Note")
		}
		if note == nil || note.UserId != userId {
			// Already deleted or not owned by user, treat as success (idempotent)
			alreadyDeleted = true
			m.logger.Info("Note already deleted or not found (idempotent)", "userId", userId, "noteId", noteId)
			return nil
		}
		if err := uow.NoteRepo().Delete(ctx, noteId); err != nil {
			m.logger.Error("Failed to delete note", "userId", userId, "noteId", noteId, "err", err)
			return ccc.NewDatabaseError("delete note", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if alreadyDeleted {
		return nil
	}
	m.logger.Info("Note deleted", "userId", userId, "noteId", noteId)
	return nil
}
