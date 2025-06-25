package documents

import "time"

// Request/Response DTOs for service layer
type CreateDocumentRequest struct {
	Title       string
	Description string
	TagIds      []string
	Files       []AddFileRequest
}

type UpdateDocumentRequest struct {
	Title       string
	Description string
	TagIds      []string
}

type AddFileRequest struct {
	FileName    string
	ContentType string
	FileData    []byte
}

type GetDocumentsRequest struct {
	Filters  DocumentFilters
	Page     int
	PageSize int
	SortBy   string // "title", "created_at", "modified_at"
	SortAsc  bool
}

type DocumentFilters struct {
	TagIds   []string
	DateFrom *time.Time
	DateTo   *time.Time
}

type DocumentSearchRequest struct {
	SearchTerm string
	Filters    DocumentFilters
	DeepSearch bool   // If true, search within OCR-extracted text content
	Page       int    // Page number (1-based)
	PageSize   int    // Number of results per page
	SortBy     string // "relevance", "created_at", "modified_at", "title"
	SortAsc    bool   // If true, sort ascending; if false, sort descending
}

type PaginatedDocumentResponse struct {
	Documents  []*DocumentDto
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
}

type PaginatedDocumentSearchResponse struct {
	Results    []*DocumentSearchResult
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
}

type DocumentSearchResult struct {
	DocumentId      string
	DocumentTitle   string // Decrypted
	HighlightedText string // Decrypted snippet
	OcrConfidence   float32
	RelevanceScore  float64 // Calculated relevance score for sorting
	CreatedAt       time.Time
	ModifiedAt      time.Time
	MatchTypes      []string            // Types of matches found: "title", "description", "filename", "content"
	Tags            []*TagDto           // Associated tags
	Preview         *DocumentPreviewDto // Preview of the oldest file in the document
}

type CreateTagRequest struct {
	Name  string
	Color string
}

type UpdateTagRequest struct {
	Name  string
	Color string
}

// DTOs for API responses (with decrypted data)
type DocumentDto struct {
	Id          string
	Title       string // Decrypted
	Description string // Decrypted
	FileCount   int
	Tags        []*TagDto
	Preview     *DocumentPreviewDto // Preview of the oldest file in the document
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

type DocumentFileDto struct {
	Id            string
	DocumentId    string
	FileName      string // Decrypted
	ContentType   string
	FileSize      int64
	PageCount     int
	ExtractedText string // Decrypted, if available
	Confidence    float32
	FileData      []byte // Decrypted, only when explicitly requested
	CreatedAt     time.Time
	ModifiedAt    time.Time
}

// DocumentFilePreviewDto represents a document file with preview data but without full file content
// This is used for listing files in the UI where we want preview images but not the entire file data
type DocumentFilePreviewDto struct {
	Id            string
	DocumentId    string
	FileName      string // Decrypted
	ContentType   string
	FileSize      int64
	PageCount     int
	ExtractedText string // Decrypted, if available
	Confidence    float32
	Preview       *DocumentPreviewDto // Preview/thumbnail data, if available
	CreatedAt     time.Time
	ModifiedAt    time.Time
}

type TagDto struct {
	Id         string
	Name       string
	Color      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// DocumentPreviewDto represents preview/thumbnail data for a document
type DocumentPreviewDto struct {
	DocumentFileId string
	PreviewData    []byte // Decrypted preview image data
	PreviewType    string // e.g., "image/jpeg", "image/png"
	Width          int    // Preview image width
	Height         int    // Preview image height
}

// CreateFileRequest encapsulates all parameters needed for creating a document file
type CreateFileRequest struct {
	UserId      string
	DocumentId  string
	FileName    string
	ContentType string
	FileData    []byte
}

// CreateDocumentResponse represents the response from creating a document
type CreateDocumentResponse struct {
	DocumentId string
}
