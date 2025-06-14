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
}

type AddFileRequest struct {
	FileName    string
	ContentType string
	FileData    []byte
}

type GetDocumentsRequest struct {
	SearchTerm string
	TagIds     []string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	SortBy     string // "title", "created_at", "modified_at"
	SortAsc    bool
}

type DocumentFilters struct {
	TagIds   []string
	DateFrom *time.Time
	DateTo   *time.Time
	SortBy   string
	SortAsc  bool
}

type SearchFilters struct {
	TagIds   []string
	DateFrom *time.Time
	DateTo   *time.Time
}

type PaginatedDocumentResponse struct {
	Documents  []*DocumentDto
	TotalCount int
	Page       int
	PageSize   int
	TotalPages int
}

type DocumentSearchResult struct {
	DocumentId      string
	DocumentTitle   string // Decrypted
	FileName        string // Decrypted
	HighlightedText string // Decrypted snippet
	FileId          string
	OcrConfidence   float32
	CreatedAt       time.Time
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
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

type DocumentFileDto struct {
	Id          string
	DocumentId  string
	FileName    string // Decrypted
	ContentType string
	FileSize    int64
	PageCount   int
	OcrText     string // Decrypted, if available
	Confidence  float32
	FileData    []byte // Decrypted, only when explicitly requested
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

type TagDto struct {
	Id         string
	Name       string
	Color      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}
