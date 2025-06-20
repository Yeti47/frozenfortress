package documents

import (
	"fmt"
	"strings"
)

// DefaultDocumentFileProcessorFactory implements DocumentFileProcessorFactory
type DefaultDocumentFileProcessorFactory struct {
	processors []DocumentFileProcessor
}

// NewDefaultDocumentFileProcessorFactory creates a new factory with injected processors
func NewDefaultDocumentFileProcessorFactory(processors ...DocumentFileProcessor) *DefaultDocumentFileProcessorFactory {
	return &DefaultDocumentFileProcessorFactory{
		processors: processors,
	}
}

// GetProcessor returns an appropriate DocumentFileProcessor for the given content type
func (f *DefaultDocumentFileProcessorFactory) GetProcessor(contentType string) (DocumentFileProcessor, error) {
	contentType = strings.ToLower(contentType)

	// Find the first processor that supports this content type
	for _, processor := range f.processors {
		if processor.SupportsContentType(contentType) {
			return processor, nil
		}
	}

	return nil, fmt.Errorf("no processor available for content type: %s", contentType)
}
