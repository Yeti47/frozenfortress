package documents

import (
	"context"
	"strings"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultDocumentListService implements DocumentListService by delegating to DocumentManager and DocumentSearchEngine
type DefaultDocumentListService struct {
	documentManager      DocumentManager
	documentSearchEngine DocumentSearchEngine
	logger               ccc.Logger
}

// NewDefaultDocumentListService creates a new DefaultDocumentListService instance
func NewDefaultDocumentListService(
	documentManager DocumentManager,
	documentSearchEngine DocumentSearchEngine,
	logger ccc.Logger,
) *DefaultDocumentListService {
	return &DefaultDocumentListService{
		documentManager:      documentManager,
		documentSearchEngine: documentSearchEngine,
		logger:               logger,
	}
}

// GetDocumentList returns either regular document listing or search results based on the request
func (s *DefaultDocumentListService) GetDocumentList(
	ctx context.Context,
	userId string,
	request DocumentListRequest,
	dataProtector dataprotection.DataProtector,
) (*DocumentListResponse, error) {
	// Validate pagination parameters and reset page to 1 if needed
	request = s.validateAndNormalizePagination(request)

	// Determine if this is a search request
	isSearchRequest := strings.TrimSpace(request.SearchTerm) != ""

	if isSearchRequest {
		return s.performSearch(ctx, userId, request, dataProtector)
	} else {
		return s.performRegularListing(ctx, userId, request, dataProtector)
	}
}

// performSearch delegates to DocumentSearchEngine and converts results to unified format
func (s *DefaultDocumentListService) performSearch(
	ctx context.Context,
	userId string,
	request DocumentListRequest,
	dataProtector dataprotection.DataProtector,
) (*DocumentListResponse, error) {
	s.logger.Debug("Performing document search", "userId", userId, "searchTerm", request.SearchTerm)

	// Convert to search request
	searchRequest := DocumentSearchRequest{
		SearchTerm: request.SearchTerm,
		DeepSearch: request.DeepSearch,
		Filters:    request.Filters,
		Page:       request.Page,
		PageSize:   request.PageSize,
		SortBy:     request.SortBy,
		SortAsc:    request.SortAsc,
	}

	// Perform search
	searchResponse, err := s.documentSearchEngine.SearchDocuments(ctx, userId, searchRequest, dataProtector)
	if err != nil {
		s.logger.Error("Failed to search documents", "error", err, "userId", userId)
		return nil, err
	}

	// Convert search results to unified format
	items := make([]*DocumentListItem, len(searchResponse.Results))
	for i, result := range searchResponse.Results {
		items[i] = &DocumentListItem{
			DocumentDto: &DocumentDto{
				Id:          result.DocumentId,
				Title:       result.DocumentTitle,
				Description: "", // Search results don't include description
				FileCount:   result.FileCount,
				Tags:        result.Tags,
				Preview:     result.Preview,
				CreatedAt:   result.CreatedAt,
				ModifiedAt:  result.ModifiedAt,
			},
			HighlightedText: result.HighlightedText,
			RelevanceScore:  result.RelevanceScore,
			OcrConfidence:   result.OcrConfidence,
			MatchTypes:      result.MatchTypes,
			IsSearchResult:  true,
		}
	}

	return &DocumentListResponse{
		Items:      items,
		TotalCount: searchResponse.TotalCount,
		Page:       searchResponse.Page,
		PageSize:   searchResponse.PageSize,
		TotalPages: searchResponse.TotalPages,
	}, nil
}

// performRegularListing delegates to DocumentManager and converts results to unified format
func (s *DefaultDocumentListService) performRegularListing(
	ctx context.Context,
	userId string,
	request DocumentListRequest,
	dataProtector dataprotection.DataProtector,
) (*DocumentListResponse, error) {
	s.logger.Debug("Performing regular document listing", "userId", userId)

	// Convert to regular get documents request
	getRequest := GetDocumentsRequest{
		Filters:  request.Filters,
		Page:     request.Page,
		PageSize: request.PageSize,
		SortBy:   request.SortBy,
		SortAsc:  request.SortAsc,
	}

	// Get regular documents
	response, err := s.documentManager.GetDocuments(ctx, userId, getRequest, dataProtector)
	if err != nil {
		s.logger.Error("Failed to get documents", "error", err, "userId", userId)
		return nil, err
	}

	// Convert regular documents to unified format
	items := make([]*DocumentListItem, len(response.Documents))
	for i, doc := range response.Documents {
		items[i] = &DocumentListItem{
			DocumentDto:     doc,
			HighlightedText: "",  // Regular documents don't have highlighted text
			RelevanceScore:  0,   // Regular documents don't have relevance scores
			OcrConfidence:   0,   // Regular documents don't have OCR confidence
			MatchTypes:      nil, // Regular documents don't have match types
			IsSearchResult:  false,
		}
	}

	return &DocumentListResponse{
		Items:      items,
		TotalCount: response.TotalCount,
		Page:       response.Page,
		PageSize:   response.PageSize,
		TotalPages: response.TotalPages,
	}, nil
}

// validateAndNormalizePagination ensures pagination parameters are valid and handles edge cases
func (s *DefaultDocumentListService) validateAndNormalizePagination(request DocumentListRequest) DocumentListRequest {
	// Ensure page is at least 1
	if request.Page < 1 {
		request.Page = 1
	}

	// Set default page size if not provided or invalid
	if request.PageSize < 1 {
		request.PageSize = 20 // Default page size
	}

	// Limit max page size to prevent excessive load
	if request.PageSize > 100 {
		request.PageSize = 100
	}

	// Set default sort field if not provided
	if request.SortBy == "" {
		if strings.TrimSpace(request.SearchTerm) != "" {
			request.SortBy = "relevance" // Default for search
		} else {
			request.SortBy = "title" // Default for regular listing
		}
	}

	return request
}
