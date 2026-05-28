package documents

import (
	"context"
	"sync"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

type DefaultOCRDispatcherFactory struct {
	uowFactory DocumentUnitOfWorkFactory
	ocrConfig  ccc.OCRConfig
	logger     ccc.Logger
}

func NewDefaultOCRDispatcherFactory(
	uowFactory DocumentUnitOfWorkFactory,
	ocrConfig ccc.OCRConfig,
	logger ccc.Logger,
) *DefaultOCRDispatcherFactory {
	if logger == nil {
		logger = ccc.NopLogger
	}
	return &DefaultOCRDispatcherFactory{
		uowFactory: uowFactory,
		ocrConfig:  ocrConfig,
		logger:     logger,
	}
}

func (f *DefaultOCRDispatcherFactory) Create() OCRDispatcher {
	return &DefaultOCRDispatcher{
		uowFactory: f.uowFactory,
		ocrConfig:  f.ocrConfig,
		logger:     f.logger,
	}
}

type DefaultOCRDispatcher struct {
	uowFactory DocumentUnitOfWorkFactory
	ocrConfig  ccc.OCRConfig
	logger     ccc.Logger
	mu         sync.Mutex
	queue      []OCRDispatchRequest
}

func (d *DefaultOCRDispatcher) Enqueue(request OCRDispatchRequest) {
	request.FileData = append([]byte(nil), request.FileData...)

	d.mu.Lock()
	defer d.mu.Unlock()
	d.queue = append(d.queue, request)
}

func (d *DefaultOCRDispatcher) Dispatch() {
	d.mu.Lock()
	queue := append([]OCRDispatchRequest(nil), d.queue...)
	d.queue = nil
	d.mu.Unlock()

	for _, request := range queue {
		request := request
		go d.process(request)
	}
}

func (d *DefaultOCRDispatcher) process(request OCRDispatchRequest) {
	maxAttempts := d.ocrConfig.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	backoff := retryBackoff(d.ocrConfig.RetryInitialBackoffSeconds, 2)
	maxBackoff := retryBackoff(d.ocrConfig.RetryMaxBackoffSeconds, 30)

	var text string
	var confidence float32
	var pageCount int
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		text, confidence, pageCount, lastErr = request.Processor.ExtractText(context.Background(), request.FileData)
		if lastErr == nil {
			break
		}
		d.logger.Warn("Async text extraction attempt failed", "error", lastErr, "fileId", request.DocumentFileId, "attempt", attempt)
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff = minDuration(backoff*2, maxBackoff)
		}
	}

	if lastErr != nil {
		d.persistFailure(request.DocumentFileId, request.StartedAt, lastErr)
		return
	}

	var encryptedText string
	if text != "" {
		var err error
		encryptedText, err = request.DataProtector.Protect(text)
		if err != nil {
			d.persistFailure(request.DocumentFileId, request.StartedAt, err)
			return
		}
	}

	d.persistSuccess(request.DocumentFileId, encryptedText, confidence, pageCount, request.StartedAt)
}

func (d *DefaultOCRDispatcher) persistSuccess(fileId, encryptedText string, confidence float32, pageCount int, startedAt time.Time) {
	completedAt := time.Now()
	metadata := &DocumentFileMetadata{
		DocumentFileId: fileId,
		ExtractedText:  encryptedText,
		OcrConfidence:  confidence,
		OcrStatus:      OcrStatusCompleted,
		OcrStartedAt:   &startedAt,
		OcrCompletedAt: &completedAt,
	}
	d.persistResult(fileId, pageCount, metadata)
}

func (d *DefaultOCRDispatcher) persistFailure(fileId string, startedAt time.Time, extractionErr error) {
	completedAt := time.Now()
	metadata := &DocumentFileMetadata{
		DocumentFileId: fileId,
		OcrStatus:      OcrStatusFailed,
		OcrError:       extractionErr.Error(),
		OcrStartedAt:   &startedAt,
		OcrCompletedAt: &completedAt,
	}
	d.persistResult(fileId, 0, metadata)
}

func (d *DefaultOCRDispatcher) persistResult(fileId string, pageCount int, metadata *DocumentFileMetadata) {
	maxAttempts := d.ocrConfig.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	backoff := retryBackoff(d.ocrConfig.RetryInitialBackoffSeconds, 2)
	maxBackoff := retryBackoff(d.ocrConfig.RetryMaxBackoffSeconds, 30)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := d.uowFactory.Create().Execute(context.Background(), func(uow DocumentUnitOfWork) error {
			if pageCount > 0 {
				file, err := uow.DocumentFileRepo().FindById(context.Background(), fileId)
				if err != nil {
					return err
				}
				if file != nil {
					file.PageCount = pageCount
					file.ModifiedAt = time.Now()
					if err := uow.DocumentFileRepo().Update(context.Background(), file); err != nil {
						return err
					}
				}
			}
			return uow.DocumentFileMetadataRepo().Update(context.Background(), metadata)
		})
		if err == nil {
			return
		}
		d.logger.Warn("Failed to persist async text extraction result", "error", err, "fileId", fileId, "attempt", attempt)
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff = minDuration(backoff*2, maxBackoff)
		}
	}
}

func retryBackoff(configuredSeconds int, defaultSeconds int) time.Duration {
	if configuredSeconds <= 0 {
		configuredSeconds = defaultSeconds
	}
	return time.Duration(configuredSeconds) * time.Second
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
