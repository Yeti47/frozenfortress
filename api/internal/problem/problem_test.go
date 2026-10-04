package problem

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/danielgtaylor/huma/v2"
)

type recordingLogger struct{ errors []string }

func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Error(msg string, _ ...any) {
	l.errors = append(l.errors, msg)
}

func model(t *testing.T, err huma.StatusError) *huma.ErrorModel {
	t.Helper()
	m, ok := err.(*huma.ErrorModel)
	if !ok {
		t.Fatalf("expected *huma.ErrorModel, got %T", err)
	}
	return m
}

func TestMap_Nil(t *testing.T) {
	if Map(nil, nil) != nil {
		t.Fatal("expected nil for nil error")
	}
}

func TestMap_ApiErrorTypes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		detail     string
		wantLogged bool
	}{
		{"not found", ccc.NewResourceNotFoundError("1", "Secret"), 404, "Secret not found", false},
		{"already exists", ccc.NewResourceAlreadyExistsError("1", "Tag"), 409, "Tag already exists", false},
		{"validation failed", ccc.NewValidationFailedError("bad"), 400, "Validation failed", false},
		{"unauthorized", ccc.NewUnauthorizedError("no"), 401, "Unauthorized access", false},
		{"forbidden", ccc.NewForbiddenError("no"), 403, "Access forbidden", false},
		{"database", ccc.NewDatabaseError("insert", errors.New("boom")), 500, "A database error occurred", true},
		{"operation failed", ccc.NewOperationFailedError("op", "why"), 500, "Operation failed", true},
		{"internal", ccc.NewInternalError("x", errors.New("boom")), 500, "An internal error occurred", true},
		{"wrapped", fmt.Errorf("wrap: %w", ccc.NewResourceNotFoundError("1", "Tag")), 404, "Tag not found", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &recordingLogger{}
			m := model(t, Map(logger, tt.err))
			if m.GetStatus() != tt.status || m.Detail != tt.detail {
				t.Fatalf("got %d %q, want %d %q", m.GetStatus(), m.Detail, tt.status, tt.detail)
			}
			if len(m.Errors) != 0 {
				t.Fatalf("unexpected errors[]: %v", m.Errors)
			}
			if (len(logger.errors) > 0) != tt.wantLogged {
				t.Fatalf("logged=%v, want %v", logger.errors, tt.wantLogged)
			}
		})
	}
}

func TestMap_InvalidInputExposesField(t *testing.T) {
	for _, err := range []error{
		ccc.NewInvalidInputError("name", "too long"),
		ccc.NewInvalidInputErrorWithMessage("name", "too long", "Name is too long"),
	} {
		m := model(t, Map(nil, err))
		if m.GetStatus() != http.StatusBadRequest {
			t.Fatalf("status = %d", m.GetStatus())
		}
		if len(m.Errors) != 1 || m.Errors[0].Location != "name" {
			t.Fatalf("errors[] = %+v", m.Errors)
		}
		if m.Errors[0].Message != m.Detail {
			t.Fatalf("message %q != detail %q", m.Errors[0].Message, m.Detail)
		}
	}
}

func TestMap_UnknownErrorIsGenericAndLogged(t *testing.T) {
	logger := &recordingLogger{}
	m := model(t, Map(logger, errors.New("secret internals")))
	if m.GetStatus() != 500 || m.Detail != GenericMessage {
		t.Fatalf("got %d %q", m.GetStatus(), m.Detail)
	}
	if len(logger.errors) != 1 {
		t.Fatalf("expected the real error to be logged, got %v", logger.errors)
	}
}

func TestMap_OutOfRangeStatusFallsBackTo500(t *testing.T) {
	m := model(t, Map(nil, &ccc.ApiError{StatusCode: 200, UserMessage: "odd"}))
	if m.GetStatus() != 500 {
		t.Fatalf("status = %d", m.GetStatus())
	}
}
