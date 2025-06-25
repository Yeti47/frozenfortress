package documents

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/dataprotection"
)

// DefaultDocumentSearchEngine implements DocumentSearchEngine interface.
// It performs application-level text search by decrypting data first and then searching.
type DefaultDocumentSearchEngine struct {
	uowFactory DocumentUnitOfWorkFactory
	logger     ccc.Logger
	sorter     DocumentSorter[*DocumentSearchResult]
}

// NewDefaultDocumentSearchEngine creates a new DefaultDocumentSearchEngine instance.
func NewDefaultDocumentSearchEngine(uowFactory DocumentUnitOfWorkFactory, logger ccc.Logger, sorter DocumentSorter[*DocumentSearchResult]) *DefaultDocumentSearchEngine {
	return &DefaultDocumentSearchEngine{
		uowFactory: uowFactory,
		logger:     logger,
		sorter:     sorter,
	}
}

// SearchDocuments performs text search across documents and their content.
// It decrypts document data before performing the search operation and returns paginated results.
func (s *DefaultDocumentSearchEngine) SearchDocuments(
	ctx context.Context,
	userId string,
	request DocumentSearchRequest,
	dataProtector dataprotection.DataProtector,
) (*PaginatedDocumentSearchResponse, error) {
	if request.SearchTerm == "" {
		return nil, ccc.NewInvalidInputError("searchTerm", "cannot be empty")
	}

	s.logger.Info("Starting document search", "userId", userId, "searchTerm", request.SearchTerm, "deepSearch", request.DeepSearch)

	// Validate and normalize pagination parameters
	page, pageSize := s.validatePaginationParams(request.Page, request.PageSize)

	// Parse search terms (support for multiple terms and quoted phrases)
	searchTerms := s.parseSearchTerms(strings.ToLower(strings.TrimSpace(request.SearchTerm)))
	s.logger.Debug("Parsed search terms", "terms", searchTerms)

	// Create unit of work
	uow := s.uowFactory.Create()

	// Get documents with tags based on filters
	documentDetails, err := uow.DocumentRepo().FindDetailed(ctx, userId, request.Filters)
	if err != nil {
		s.logger.Error("Failed to retrieve detailed documents", "userId", userId, "error", err)
		return nil, ccc.NewDatabaseError("find detailed documents", err)
	}

	s.logger.Debug("Retrieved detailed documents for search", "count", len(documentDetails))

	// Use map to aggregate results by document ID
	resultsByDoc := make(map[string]*DocumentSearchResult)

	// If deep search is requested, collect file matches in batches for better performance
	var allFileMatches map[string]*fileMatchInfo
	if request.DeepSearch {
		s.logger.Debug("Starting deep search file processing", "documentCount", len(documentDetails))

		var err error
		allFileMatches, err = s.collectAllFileMatches(ctx, documentDetails, searchTerms, dataProtector, uow)
		if err != nil {
			// Log error but continue with document-level search
			s.logger.Warn("Failed to collect file matches, continuing with document-level search only", "error", err)
			allFileMatches = make(map[string]*fileMatchInfo)
		} else {
			s.logger.Debug("Deep search completed", "documentsWithFileMatches", len(allFileMatches))
		}
	}

	for _, docDetail := range documentDetails {
		doc := docDetail.Document
		// Decrypt document title and description
		decryptedTitle, err := dataProtector.Unprotect(doc.Title)
		if err != nil {
			// Skip documents we can't decrypt
			continue
		}

		decryptedDescription, err := dataProtector.Unprotect(doc.Description)
		if err != nil {
			// Use empty description if decryption fails
			decryptedDescription = ""
		}

		// Check for matches and collect match information
		var matchTypes []string
		var highlightParts []string
		var maxOcrConfidence float32

		// Search in document title and description
		titleMatches := s.findMatchesForTerms(decryptedTitle, searchTerms)
		descriptionMatches := s.findMatchesForTerms(decryptedDescription, searchTerms)

		// Document-level matches
		if len(titleMatches) > 0 || len(descriptionMatches) > 0 {
			if len(titleMatches) > 0 {
				matchTypes = append(matchTypes, "title")
				highlighted := s.highlightMatchesForTerms(decryptedTitle, titleMatches, searchTerms)
				highlightParts = append(highlightParts, fmt.Sprintf("Title: %s", highlighted))
			}
			if len(descriptionMatches) > 0 {
				matchTypes = append(matchTypes, "description")
				snippet := s.createSnippet(decryptedDescription, descriptionMatches[0], searchTerms[0], 100)
				highlighted := s.highlightMatchesForTerms(snippet, s.findMatchesForTerms(snippet, searchTerms), searchTerms)
				highlightParts = append(highlightParts, fmt.Sprintf("Description: %s", highlighted))
			}
		}

		// If deep search is requested, search in file content and OCR text
		if request.DeepSearch {
			// Use pre-loaded file matches from batch operation
			if fileMatchInfo, exists := allFileMatches[doc.Id]; exists {
				// Aggregate file-level matches
				if len(fileMatchInfo.FileNameMatches) > 0 {
					matchTypes = append(matchTypes, "filename")
					for _, match := range fileMatchInfo.FileNameMatches {
						highlightParts = append(highlightParts, fmt.Sprintf("File: %s", match))
					}
				}
				if len(fileMatchInfo.OcrMatches) > 0 {
					matchTypes = append(matchTypes, "content")
					for _, match := range fileMatchInfo.OcrMatches {
						highlightParts = append(highlightParts, match.HighlightedText)
						if match.OcrConfidence > maxOcrConfidence {
							maxOcrConfidence = match.OcrConfidence
						}
					}
				}
			}
		}

		// Only add result if we found any matches
		if len(matchTypes) > 0 {
			// Build tag DTOs from DocumentDetails
			tagDtos := make([]*TagDto, 0, len(docDetail.Tags))
			for _, tag := range docDetail.Tags {
				tagDtos = append(tagDtos, &TagDto{
					Id:         tag.Id,
					Name:       tag.Name,
					Color:      tag.Color,
					CreatedAt:  tag.CreatedAt,
					ModifiedAt: tag.ModifiedAt,
				})
			}

			result := &DocumentSearchResult{
				DocumentId:      doc.Id,
				DocumentTitle:   decryptedTitle,
				HighlightedText: strings.Join(highlightParts, " | "),
				OcrConfidence:   maxOcrConfidence,
				CreatedAt:       doc.CreatedAt,
				ModifiedAt:      doc.ModifiedAt,
				MatchTypes:      matchTypes,
				Tags:            tagDtos,
			}
			resultsByDoc[doc.Id] = result
		}
	}

	// Convert map to slice
	var allResults []*DocumentSearchResult
	for _, result := range resultsByDoc {
		allResults = append(allResults, result)
	}

	// Sort results by specified criteria or default to relevance
	s.sortSearchResults(allResults, searchTerms, request.SortBy, request.SortAsc)

	// Apply pagination
	totalCount := len(allResults)
	totalPages := (totalCount + pageSize - 1) / pageSize // Ceiling division

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	if startIndex >= totalCount {
		// Page is beyond available results
		return &PaginatedDocumentSearchResponse{
			Results:    []*DocumentSearchResult{},
			TotalCount: totalCount,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}, nil
	}

	if endIndex > totalCount {
		endIndex = totalCount
	}

	pagedResults := allResults[startIndex:endIndex]

	// Load preview data for the paged results
	if len(pagedResults) > 0 {
		documentIds := make([]string, len(pagedResults))
		for i, result := range pagedResults {
			documentIds[i] = result.DocumentId
		}

		previews, err := uow.DocumentFileRepo().FindOldestPreviewsByDocumentIds(ctx, documentIds)
		if err != nil {
			s.logger.Warn("Failed to load document previews for search results", "error", err)
			// Continue without previews if loading fails
		} else {
			// Decrypt and attach preview data to results
			for _, result := range pagedResults {
				if preview, exists := previews[result.DocumentId]; exists && preview != nil {
					decryptedPreviewData, err := dataProtector.Unprotect(string(preview.PreviewData))
					if err != nil {
						s.logger.Warn("Failed to decrypt preview data for search result", "documentId", result.DocumentId, "error", err)
						continue
					}
					result.Preview = &DocumentPreviewDto{
						DocumentFileId: preview.DocumentFileId,
						PreviewData:    []byte(decryptedPreviewData),
						PreviewType:    preview.PreviewType,
						Width:          preview.Width,
						Height:         preview.Height,
					}
				}
			}
		}
	}

	s.logger.Info("Document search completed",
		"userId", userId,
		"totalResults", totalCount,
		"returnedResults", len(pagedResults),
		"page", page,
		"totalPages", totalPages,
		"deepSearch", request.DeepSearch)

	return &PaginatedDocumentSearchResponse{
		Results:    pagedResults,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// fileMatchInfo holds aggregated file match information for a document
type fileMatchInfo struct {
	FileNameMatches []string
	OcrMatches      []ocrMatch
}

// ocrMatch represents a match found in OCR content
type ocrMatch struct {
	HighlightedText string
	OcrConfidence   float32
}

const (
	// Maximum number of documents to process in a single batch for file details loading
	// Limited by SQLite's SQLITE_MAX_VARIABLE_NUMBER (default 999 parameters)
	// We use a conservative limit to leave room for other query parameters
	maxBatchSize = 50

	// SQLite parameter limit safety check
	sqliteMaxParams = 999
)

// collectAllFileMatches collects file matches for all documents using batch loading for optimal performance
func (s *DefaultDocumentSearchEngine) collectAllFileMatches(
	ctx context.Context,
	documentDetails []*DocumentDetails,
	searchTerms []string,
	dataProtector dataprotection.DataProtector,
	uow DocumentUnitOfWork,
) (map[string]*fileMatchInfo, error) {
	allMatches := make(map[string]*fileMatchInfo)

	s.logger.Debug("Starting batch file collection", "totalDocuments", len(documentDetails), "batchSize", maxBatchSize)

	// Process documents in batches to optimize database queries
	batchCount := 0
	for i := 0; i < len(documentDetails); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(documentDetails) {
			end = len(documentDetails)
		}

		batch := documentDetails[i:end]
		batchCount++
		s.logger.Debug("Processing batch", "batchNumber", batchCount, "documentsInBatch", len(batch))

		batchMatches, err := s.collectFileMatchesBatch(ctx, batch, searchTerms, dataProtector, uow)
		if err != nil {
			s.logger.Warn("Batch processing failed, continuing with remaining batches", "batchNumber", batchCount, "error", err)
			// Continue with other batches if one fails
			continue
		}

		// Merge batch results
		for docId, matches := range batchMatches {
			allMatches[docId] = matches
		}
	}

	s.logger.Debug("Completed batch file collection", "totalBatches", batchCount, "documentsWithMatches", len(allMatches))
	return allMatches, nil
}

// collectFileMatchesBatch processes a batch of documents to collect file matches efficiently
func (s *DefaultDocumentSearchEngine) collectFileMatchesBatch(
	ctx context.Context,
	documentDetails []*DocumentDetails,
	searchTerms []string,
	dataProtector dataprotection.DataProtector,
	uow DocumentUnitOfWork,
) (map[string]*fileMatchInfo, error) {
	if len(documentDetails) == 0 {
		return make(map[string]*fileMatchInfo), nil
	}

	// Safety check: ensure we don't exceed SQLite parameter limits
	if len(documentDetails) > sqliteMaxParams {
		return nil, ccc.NewInvalidInputError("documentIds", fmt.Sprintf("batch size %d exceeds SQLite parameter limit of %d", len(documentDetails), sqliteMaxParams))
	}

	// Extract document IDs for batch query
	documentIds := make([]string, len(documentDetails))
	for i, docDetail := range documentDetails {
		documentIds[i] = docDetail.Document.Id
	}

	// Get all extended file metadata for all documents in a single query (without file data)
	allExtendedMetadata, err := uow.DocumentFileMetadataRepo().FindExtended(ctx, documentIds)
	if err != nil {
		return nil, ccc.NewDatabaseError("find extended document file metadata for batch", err)
	}

	// Group extended metadata by document ID
	metadataByDoc := make(map[string][]*ExtendedDocumentFileMetadata)
	for _, extended := range allExtendedMetadata {
		docId := extended.DocumentId
		metadataByDoc[docId] = append(metadataByDoc[docId], extended)
	}

	// Process each document's files
	results := make(map[string]*fileMatchInfo)
	for _, docDetail := range documentDetails {
		doc := docDetail.Document
		matchInfo := &fileMatchInfo{
			FileNameMatches: []string{},
			OcrMatches:      []ocrMatch{},
		}

		extendedMetadataList := metadataByDoc[doc.Id]
		for _, extended := range extendedMetadataList {
			// Decrypt file name
			decryptedFileName, err := dataProtector.Unprotect(extended.FileName)
			if err != nil {
				// Skip files we can't decrypt
				continue
			}

			// Search in file name
			fileNameMatches := s.findMatchesForTerms(decryptedFileName, searchTerms)
			if len(fileNameMatches) > 0 {
				highlighted := s.highlightMatchesForTerms(decryptedFileName, fileNameMatches, searchTerms)
				matchInfo.FileNameMatches = append(matchInfo.FileNameMatches, highlighted)
			}

			// Search in OCR extracted text if available
			if extended.ExtractedText != "" {
				decryptedOcrText, err := dataProtector.Unprotect(extended.ExtractedText)
				if err != nil {
					// Skip if we can't decrypt OCR text
					continue
				}

				// Search in OCR text
				ocrMatches := s.findMatchesForTerms(decryptedOcrText, searchTerms)
				if len(ocrMatches) > 0 {
					// Create a snippet around the first match with context
					snippet := s.createSnippet(decryptedOcrText, ocrMatches[0], searchTerms[0], 150)
					snippetMatches := s.findMatchesForTerms(snippet, searchTerms)
					highlighted := s.highlightMatchesForTerms(snippet, snippetMatches, searchTerms)

					ocrMatch := ocrMatch{
						HighlightedText: fmt.Sprintf("Content: %s", highlighted),
						OcrConfidence:   extended.OcrConfidence,
					}
					matchInfo.OcrMatches = append(matchInfo.OcrMatches, ocrMatch)
				}
			}
		}

		// Only add to results if we found matches
		if len(matchInfo.FileNameMatches) > 0 || len(matchInfo.OcrMatches) > 0 {
			results[doc.Id] = matchInfo
		}
	}

	return results, nil
}

// parseSearchTerms splits search term into individual words and handles quoted phrases
func (s *DefaultDocumentSearchEngine) parseSearchTerms(searchTerm string) []string {
	var terms []string
	inQuotes := false
	currentTerm := ""

	for _, char := range searchTerm {
		if char == '"' {
			if inQuotes {
				// End of quoted phrase
				if currentTerm != "" {
					terms = append(terms, strings.TrimSpace(currentTerm))
					currentTerm = ""
				}
				inQuotes = false
			} else {
				// Start of quoted phrase
				if currentTerm != "" {
					// Add any accumulated term before starting quote
					words := strings.Fields(currentTerm)
					terms = append(terms, words...)
					currentTerm = ""
				}
				inQuotes = true
			}
		} else if char == ' ' && !inQuotes {
			if currentTerm != "" {
				terms = append(terms, strings.TrimSpace(currentTerm))
				currentTerm = ""
			}
		} else {
			currentTerm += string(char)
		}
	}

	// Add final term
	if currentTerm != "" {
		if inQuotes {
			terms = append(terms, strings.TrimSpace(currentTerm))
		} else {
			words := strings.Fields(currentTerm)
			terms = append(terms, words...)
		}
	}

	// Filter out empty terms and normalize to lowercase
	var cleanTerms []string
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" {
			cleanTerms = append(cleanTerms, term)
		}
	}

	return cleanTerms
}

// findMatches finds all occurrences of searchTerm in text (case-insensitive)
func (s *DefaultDocumentSearchEngine) findMatches(text, searchTerm string) []int {
	if text == "" || searchTerm == "" {
		return nil
	}

	lowerText := strings.ToLower(text)
	var matches []int

	start := 0
	for {
		index := strings.Index(lowerText[start:], searchTerm)
		if index == -1 {
			break
		}
		matches = append(matches, start+index)
		start = start + index + 1
	}

	return matches
}

// findMatchesForTerms finds matches for multiple search terms (AND logic)
func (s *DefaultDocumentSearchEngine) findMatchesForTerms(text string, searchTerms []string) []int {
	if text == "" || len(searchTerms) == 0 {
		return nil
	}

	lowerText := strings.ToLower(text)
	var allMatches []int

	// Check if all terms are present
	for _, term := range searchTerms {
		termMatches := s.findSingleTermMatches(lowerText, term)
		if len(termMatches) == 0 {
			// If any term is not found, return no matches (AND logic)
			return nil
		}
		allMatches = append(allMatches, termMatches...)
	}

	// Sort matches by position
	sort.Ints(allMatches)

	return allMatches
}

// findSingleTermMatches finds all occurrences of a single term in text
func (s *DefaultDocumentSearchEngine) findSingleTermMatches(lowerText, searchTerm string) []int {
	var matches []int
	start := 0

	for {
		index := strings.Index(lowerText[start:], searchTerm)
		if index == -1 {
			break
		}
		matches = append(matches, start+index)
		start = start + index + 1
	}

	return matches
}

// calculateRelevanceScore calculates a relevance score for a search result
func (s *DefaultDocumentSearchEngine) calculateRelevanceScore(result *DocumentSearchResult, searchTerms []string) float64 {
	score := 10.0 // Base score for document matches

	// OCR confidence bonus
	if result.OcrConfidence > 0 {
		score += float64(result.OcrConfidence) * 3.0 // Up to 3 points for high confidence OCR
	}

	// Count term matches in highlighted text
	lowerHighlighted := strings.ToLower(result.HighlightedText)
	for _, term := range searchTerms {
		termCount := strings.Count(lowerHighlighted, term)
		score += float64(termCount) * 2.0 // 2 points per term occurrence
	}

	// Title match bonus
	lowerTitle := strings.ToLower(result.DocumentTitle)
	for _, term := range searchTerms {
		if strings.Contains(lowerTitle, term) {
			score += 5.0 // Bonus for terms found in title
		}
	}

	// Recency bonus (newer documents get slight boost)
	now := time.Now()
	age := now.Sub(result.CreatedAt)

	// Give a small bonus for newer documents
	// Documents created within the last 30 days get up to 2 bonus points
	// Documents created within the last 90 days get up to 1 bonus point
	// Documents created within the last 365 days get up to 0.5 bonus points
	if age < 30*24*time.Hour {
		// Linear decay from 2 points at 0 days to 0 points at 30 days
		daysSinceCreation := age.Hours() / 24
		recencyBonus := 2.0 * (1.0 - daysSinceCreation/30.0)
		score += recencyBonus
	} else if age < 90*24*time.Hour {
		// Linear decay from 1 point at 30 days to 0 points at 90 days
		daysSinceCreation := age.Hours() / 24
		recencyBonus := 1.0 * (1.0 - (daysSinceCreation-30.0)/60.0)
		score += recencyBonus
	} else if age < 365*24*time.Hour {
		// Linear decay from 0.5 points at 90 days to 0 points at 365 days
		daysSinceCreation := age.Hours() / 24
		recencyBonus := 0.5 * (1.0 - (daysSinceCreation-90.0)/275.0)
		score += recencyBonus
	}
	// Documents older than 1 year get no recency bonus

	return score
}

// createSnippet creates a text snippet with context around a match position
func (s *DefaultDocumentSearchEngine) createSnippet(text string, matchPos int, searchTerm string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	contextLength := (maxLength - len(searchTerm)) / 2

	start := matchPos - contextLength
	if start < 0 {
		start = 0
	}

	end := start + maxLength
	if end > len(text) {
		end = len(text)
		start = end - maxLength
		if start < 0 {
			start = 0
		}
	}

	snippet := text[start:end]

	// Add ellipsis if we're not at the beginning/end
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}

	return snippet
}

// highlightMatches adds highlighting markers around matches
func (s *DefaultDocumentSearchEngine) highlightMatches(text string, matches []int, searchTerm string) string {
	if len(matches) == 0 {
		return text
	}

	// Build result with highlights, working backwards to preserve indices
	result := text
	for i := len(matches) - 1; i >= 0; i-- {
		pos := matches[i]
		if pos >= 0 && pos+len(searchTerm) <= len(result) {
			// Add highlighting markers (using **text** format)
			before := result[:pos]
			match := result[pos : pos+len(searchTerm)]
			after := result[pos+len(searchTerm):]
			result = before + "**" + match + "**" + after
		}
	}

	return result
}

// highlightMatchesForTerms adds highlighting markers for multiple search terms
func (s *DefaultDocumentSearchEngine) highlightMatchesForTerms(text string, matches []int, searchTerms []string) string {
	if len(matches) == 0 || len(searchTerms) == 0 {
		return text
	}

	result := text
	lowerText := strings.ToLower(text)

	// Collect all match positions for all terms
	type matchInfo struct {
		pos    int
		length int
	}

	var allMatches []matchInfo
	for _, term := range searchTerms {
		termMatches := s.findSingleTermMatches(lowerText, term)
		for _, pos := range termMatches {
			allMatches = append(allMatches, matchInfo{pos: pos, length: len(term)})
		}
	}

	// Sort by position (descending) to apply highlights backwards
	sort.Slice(allMatches, func(i, j int) bool {
		return allMatches[i].pos > allMatches[j].pos
	})

	// Apply highlights backwards to preserve positions
	for _, match := range allMatches {
		if match.pos >= 0 && match.pos+match.length <= len(result) {
			before := result[:match.pos]
			matchText := result[match.pos : match.pos+match.length]
			after := result[match.pos+match.length:]
			result = before + "**" + matchText + "**" + after
		}
	}

	return result
}

// sortSearchResults sorts search results by the specified criteria
func (s *DefaultDocumentSearchEngine) sortSearchResults(results []*DocumentSearchResult, searchTerms []string, sortBy string, sortAsc bool) {
	if len(results) <= 1 {
		return
	}

	// Calculate relevance scores for all results (needed for relevance sorting)
	for _, result := range results {
		result.RelevanceScore = s.calculateRelevanceScore(result, searchTerms)
	}

	// Let the sorter handle all sorting logic, including relevance
	s.sorter.Sort(results, sortBy, sortAsc)
}

// validatePaginationParams validates and normalizes pagination parameters
func (s *DefaultDocumentSearchEngine) validatePaginationParams(page, pageSize int) (int, int) {
	// Ensure page is at least 1
	if page < 1 {
		page = 1
	}

	// Set default page size if not provided or invalid
	if pageSize < 1 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100 // Limit max page size to prevent excessive load
	}

	return page, pageSize
}
