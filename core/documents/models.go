package documents

import "time"

type Document struct {
	Id          string
	UserId      string
	Title       string // Encrypted title
	Description string // Encrypted description
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

type DocumentFile struct {
	Id          string
	DocumentId  string
	FileName    string // Encrypted file name
	ContentType string
	FileSize    int64
	PageCount   int
	FileData    []byte // Encrypted file data/content
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

type DocumentFileMetadata struct {
	DocumentFileId string
	ExtractedText  string // Encrypted extracted text
	OcrConfidence  float32
}

type Tag struct {
	Id         string
	UserId     string
	Name       string
	Color      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type DocumentTag struct {
	DocumentId string
	TagId      string
}

// Note represents a free text note attached to a document
type Note struct {
	Id         string
	DocumentId string
	UserId     string
	Content    string // Encrypted note content
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// DocumentFilePreview represents preview/thumbnail data for a document file
type DocumentFilePreview struct {
	DocumentFileId string
	PreviewData    []byte // Encrypted preview image data (e.g., thumbnail JPEG)
	PreviewType    string // e.g., "image/jpeg", "image/png"
	Width          int    // Preview image width
	Height         int    // Preview image height
}

// ExtendedDocumentFileMetadata combines DocumentFileMetadata with lightweight DocumentFile fields
// This is used for efficient searching without loading the full file data
type ExtendedDocumentFileMetadata struct {
	DocumentFileId string
	DocumentId     string
	FileName       string // Encrypted file name
	ContentType    string
	FileSize       int64
	PageCount      int
	CreatedAt      time.Time
	ModifiedAt     time.Time
	ExtractedText  string // Encrypted extracted text
	OcrConfidence  float32
}

// DocumentFileDetails combines DocumentFile with its optional metadata
type DocumentFileDetails struct {
	File     *DocumentFile
	Metadata *DocumentFileMetadata // Can be nil if no metadata exists
	Preview  *DocumentFilePreview  // Can be nil if no preview exists
}

// PreviewGenerationResult encapsulates the result of preview generation
type PreviewGenerationResult struct {
	PreviewData []byte // The generated preview image data (unencrypted)
	PreviewType string // The MIME type of the preview (e.g., "image/jpeg", "image/png")
	Width       int    // Preview image width in pixels
	Height      int    // Preview image height in pixels
}

// DocumentDetails combines Document with its associated tags and file count
type DocumentDetails struct {
	Document  *Document
	Tags      []*Tag
	FileCount int
}
