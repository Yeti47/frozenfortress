package documents

import (
	"sort"
	"strings"
	"time"
)

// SortableDocument defines the interface for documents that can be sorted
type SortableDocument interface {
	GetTitle() string
	GetCreatedAt() time.Time
	GetModifiedAt() time.Time
}

// DocumentSorter interface for sorting documents
type DocumentSorter[T SortableDocument] interface {
	// Sort by string criteria (handles normalization internally)
	Sort(items []T, sortBy string, ascending bool)
}

// DefaultDocumentSorter provides stateless sorting functionality
type DefaultDocumentSorter[T SortableDocument] struct{}

// NewDefaultDocumentSorter creates a new stateless sorter
func NewDefaultDocumentSorter[T SortableDocument]() DocumentSorter[T] {
	return &DefaultDocumentSorter[T]{}
}

// Sort sorts a slice of documents by the specified criteria
func (s *DefaultDocumentSorter[T]) Sort(items []T, sortBy string, ascending bool) {
	if len(items) <= 1 {
		return
	}

	// Normalize the sort criteria internally
	normalizedCriteria := s.normalizeSortCriteria(sortBy)

	sort.Slice(items, func(i, j int) bool {
		switch normalizedCriteria {
		case "title":
			return s.compareByTitle(items[i], items[j], ascending)
		case "created_at":
			return s.compareByCreatedAt(items[i], items[j], ascending)
		case "modified_at":
			return s.compareByModifiedAt(items[i], items[j], ascending)
		default:
			// Default to modified_at descending
			return s.compareByModifiedAt(items[i], items[j], false)
		}
	})
}

// normalizeSortCriteria converts string to normalized criteria (internal method)
func (s *DefaultDocumentSorter[T]) normalizeSortCriteria(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "title":
		return "title"
	case "created_at", "createdat", "created":
		return "created_at"
	case "modified_at", "modifiedat", "modified":
		return "modified_at"
	default:
		return "modified_at" // Default
	}
}

// Built-in comparison methods
func (s *DefaultDocumentSorter[T]) compareByTitle(a, b T, ascending bool) bool {
	titleA := strings.ToLower(a.GetTitle())
	titleB := strings.ToLower(b.GetTitle())

	if ascending {
		return titleA < titleB
	}
	return titleA > titleB
}

func (s *DefaultDocumentSorter[T]) compareByCreatedAt(a, b T, ascending bool) bool {
	if ascending {
		return a.GetCreatedAt().Before(b.GetCreatedAt())
	}
	return a.GetCreatedAt().After(b.GetCreatedAt())
}

func (s *DefaultDocumentSorter[T]) compareByModifiedAt(a, b T, ascending bool) bool {
	if ascending {
		return a.GetModifiedAt().Before(b.GetModifiedAt())
	}
	return a.GetModifiedAt().After(b.GetModifiedAt())
}

func (d *DocumentDetails) GetTitle() string {
	return d.Document.Title
}

func (d *DocumentDetails) GetCreatedAt() time.Time {
	return d.Document.CreatedAt
}

func (d *DocumentDetails) GetModifiedAt() time.Time {
	return d.Document.ModifiedAt
}

// SearchDocumentSorter extends sorting functionality for search results with relevance support
type SearchDocumentSorter struct {
	baseSorter DocumentSorter[*DocumentSearchResult]
}

// NewSearchDocumentSorter creates a new search document sorter with relevance support
func NewSearchDocumentSorter() DocumentSorter[*DocumentSearchResult] {
	return &SearchDocumentSorter{
		baseSorter: NewDefaultDocumentSorter[*DocumentSearchResult](),
	}
}

// Sort sorts search results by the specified criteria, with relevance support
func (s *SearchDocumentSorter) Sort(items []*DocumentSearchResult, sortBy string, ascending bool) {
	if len(items) <= 1 {
		return
	}

	// Handle relevance sorting specifically
	normalizedCriteria := s.normalizeSortCriteria(sortBy)
	if normalizedCriteria == "relevance" {
		sort.Slice(items, func(i, j int) bool {
			return s.compareByRelevance(items[i], items[j], ascending)
		})
		return
	}

	// Delegate to base sorter for standard criteria
	s.baseSorter.Sort(items, sortBy, ascending)
}

// normalizeSortCriteria converts string to normalized criteria, including relevance
func (s *SearchDocumentSorter) normalizeSortCriteria(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "relevance", "score":
		return "relevance"
	case "title":
		return "title"
	case "created_at", "createdat", "created":
		return "created_at"
	case "modified_at", "modifiedat", "modified":
		return "modified_at"
	default:
		return "relevance" // Default for search results
	}
}

// compareByRelevance compares search results by their relevance score with tiebreakers
func (s *SearchDocumentSorter) compareByRelevance(a, b *DocumentSearchResult, ascending bool) bool {
	if a.RelevanceScore != b.RelevanceScore {
		if ascending {
			return a.RelevanceScore < b.RelevanceScore
		}
		return a.RelevanceScore > b.RelevanceScore
	}

	// Tiebreaker 1: OCR confidence
	if a.OcrConfidence > 0 && b.OcrConfidence > 0 && a.OcrConfidence != b.OcrConfidence {
		if ascending {
			return a.OcrConfidence < b.OcrConfidence
		}
		return a.OcrConfidence > b.OcrConfidence
	}

	// Tiebreaker 2: Creation date (newer first for relevance sorting)
	return a.CreatedAt.After(b.CreatedAt)
}

func (r *DocumentSearchResult) GetTitle() string {
	return r.DocumentTitle
}

func (r *DocumentSearchResult) GetCreatedAt() time.Time {
	return r.CreatedAt
}

func (r *DocumentSearchResult) GetModifiedAt() time.Time {
	return r.ModifiedAt
}
