package documents

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

// DefaultTagManager implements TagManager using a DocumentUnitOfWorkFactory and Logger
// It handles tag CRUD operations with logging and error handling

type DefaultTagManager struct {
	uowFactory  DocumentUnitOfWorkFactory
	idGenerator TagIdGenerator
	logger      ccc.Logger
}

// NewDefaultTagManager creates a new DefaultTagManager
func NewDefaultTagManager(uowFactory DocumentUnitOfWorkFactory, idGenerator TagIdGenerator, logger ccc.Logger) *DefaultTagManager {
	if logger == nil {
		logger = ccc.NopLogger
	}
	return &DefaultTagManager{
		uowFactory:  uowFactory,
		idGenerator: idGenerator,
		logger:      logger,
	}
}

// validateTagInput validates the tag name and color according to business rules.
func validateTagInput(name, color string) error {
	const maxNameLength = 40

	if len(name) > maxNameLength {
		return ccc.NewInvalidInputErrorWithMessage(
			"name",
			fmt.Sprintf("must not exceed %d characters", maxNameLength),
			fmt.Sprintf("The tag name must not exceed %d characters.", maxNameLength),
		)
	}
	matched, _ := regexp.MatchString(`^#[0-9a-fA-F]{6}$`, color)
	if !matched {
		return ccc.NewInvalidInputErrorWithMessage(
			"color",
			"must be a valid hex code (e.g. #225566)",
			"The color must be a valid hex code, e.g. #225566.",
		)
	}
	return nil
}

// CreateTag creates a new tag for the given user and request, assigning a generated ID.
// The operation is performed in a transaction scope.
func (m *DefaultTagManager) CreateTag(ctx context.Context, userId string, request CreateTagRequest) (*TagDto, error) {
	if err := validateTagInput(request.Name, request.Color); err != nil {
		return nil, err
	}
	now := time.Now()
	uow := m.uowFactory.Create()
	var tag *Tag
	err := uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		tag = &Tag{
			Id:         m.idGenerator.GenerateId(),
			UserId:     userId,
			Name:       request.Name,
			Color:      request.Color,
			CreatedAt:  now,
			ModifiedAt: now,
		}
		if err := uow.TagRepo().Add(ctx, tag); err != nil {
			m.logger.Error("Failed to create tag", "userId", userId, "name", request.Name, "err", err)
			return ccc.NewDatabaseError("add tag", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.logger.Info("Tag created", "userId", userId, "name", request.Name)
	return &TagDto{
		Id:         tag.Id,
		Name:       tag.Name,
		Color:      tag.Color,
		CreatedAt:  tag.CreatedAt,
		ModifiedAt: tag.ModifiedAt,
	}, nil
}

// GetTag retrieves a tag by its ID for the given user.
// Returns a TagDto if found and owned by the user, otherwise a not found error.
func (m *DefaultTagManager) GetTag(ctx context.Context, userId, tagId string) (*TagDto, error) {
	uow := m.uowFactory.Create()
	tag, err := uow.TagRepo().FindById(ctx, tagId)
	if err != nil {
		m.logger.Error("Failed to get tag", "userId", userId, "tagId", tagId, "err", err)
		return nil, ccc.NewResourceNotFoundError(tagId, "Tag")
	}
	if tag == nil || tag.UserId != userId {
		m.logger.Warn("Tag not found or not owned by user", "userId", userId, "tagId", tagId)
		return nil, ccc.NewResourceNotFoundError(tagId, "Tag")
	}
	return &TagDto{
		Id:         tag.Id,
		Name:       tag.Name,
		Color:      tag.Color,
		CreatedAt:  tag.CreatedAt,
		ModifiedAt: tag.ModifiedAt,
	}, nil
}

// GetUserTags retrieves all tags belonging to the given user.
func (m *DefaultTagManager) GetUserTags(ctx context.Context, userId string) ([]*TagDto, error) {
	uow := m.uowFactory.Create()
	tags, err := uow.TagRepo().FindByUserId(ctx, userId)
	if err != nil {
		m.logger.Error("Failed to get user tags", "userId", userId, "err", err)
		return nil, ccc.NewDatabaseError("find user tags", err)
	}
	var dtos []*TagDto
	for _, tag := range tags {
		dtos = append(dtos, &TagDto{
			Id:         tag.Id,
			Name:       tag.Name,
			Color:      tag.Color,
			CreatedAt:  tag.CreatedAt,
			ModifiedAt: tag.ModifiedAt,
		})
	}
	return dtos, nil
}

// UpdateTag updates the name and/or color of a tag for the given user and tag ID.
// The operation is performed in a transaction scope.
func (m *DefaultTagManager) UpdateTag(ctx context.Context, userId, tagId string, request UpdateTagRequest) error {
	if request.Name != "" || request.Color != "" {
		name := request.Name
		color := request.Color
		if name == "" || color == "" {
			tag, _ := m.uowFactory.Create().TagRepo().FindById(ctx, tagId)
			if tag != nil {
				if name == "" {
					name = tag.Name
				}
				if color == "" {
					color = tag.Color
				}
			}
		}
		if err := validateTagInput(name, color); err != nil {
			return err
		}
	}
	uow := m.uowFactory.Create()
	return uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		tag, err := uow.TagRepo().FindById(ctx, tagId)
		if err != nil {
			m.logger.Error("Failed to find tag for update", "userId", userId, "tagId", tagId, "err", err)
			return ccc.NewResourceNotFoundError(tagId, "Tag")
		}
		if tag == nil || tag.UserId != userId {
			m.logger.Warn("Tag not found or not owned by user for update", "userId", userId, "tagId", tagId)
			return ccc.NewResourceNotFoundError(tagId, "Tag")
		}
		if request.Name != "" {
			tag.Name = request.Name
		}
		if request.Color != "" {
			tag.Color = request.Color
		}
		if err := uow.TagRepo().Update(ctx, tag); err != nil {
			m.logger.Error("Failed to update tag", "userId", userId, "tagId", tagId, "err", err)
			return ccc.NewDatabaseError("update tag", err)
		}
		m.logger.Info("Tag updated", "userId", userId, "tagId", tagId)
		return nil
	})
}

// DeleteTag deletes a tag and all its document-tag relations for the given user and tag ID.
// The operation is idempotent and performed in a transaction scope.
func (m *DefaultTagManager) DeleteTag(ctx context.Context, userId, tagId string) error {
	uow := m.uowFactory.Create()
	alreadyDeleted := false
	err := uow.Execute(ctx, func(uow DocumentUnitOfWork) error {
		tag, err := uow.TagRepo().FindById(ctx, tagId)
		if err != nil {
			m.logger.Error("Failed to find tag for delete", "userId", userId, "tagId", tagId, "err", err)
			return ccc.NewResourceNotFoundError(tagId, "Tag")
		}
		if tag == nil {
			// Already deleted, treat as success (idempotent)
			alreadyDeleted = true
			m.logger.Info("Tag already deleted (idempotent)", "userId", userId, "tagId", tagId)
			return nil
		}
		if tag.UserId != userId {
			m.logger.Warn("Tag not owned by user for delete", "userId", userId, "tagId", tagId)
			return ccc.NewResourceNotFoundError(tagId, "Tag")
		}
		if err := uow.DocumentTagRepo().RemoveAllDocumentTags(ctx, tagId); err != nil {
			m.logger.Error("Failed to remove document tags for tag delete", "userId", userId, "tagId", tagId, "err", err)
			return ccc.NewDatabaseError("remove document tags for tag delete", err)
		}
		if err := uow.TagRepo().Delete(ctx, tagId); err != nil {
			m.logger.Error("Failed to delete tag", "userId", userId, "tagId", tagId, "err", err)
			return ccc.NewDatabaseError("delete tag", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if alreadyDeleted {
		return nil
	}
	m.logger.Info("Tag and all document tags deleted", "userId", userId, "tagId", tagId)
	return nil
}
