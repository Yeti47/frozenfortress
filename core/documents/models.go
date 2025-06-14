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
