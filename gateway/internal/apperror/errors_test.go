package apperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppError_ImplementsError(t *testing.T) {
	var err error = &AppError{
		Code:       "TEST",
		Message:    "test error",
		HTTPStatus: http.StatusBadRequest,
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner problem")
	appErr := &AppError{
		Code:       "WRAPPED",
		Message:    "wrapped error",
		HTTPStatus: http.StatusInternalServerError,
		Err:        inner,
	}

	if !errors.Is(appErr, inner) {
		t.Error("errors.Is should find the wrapped inner error")
	}
}

func TestAppError_ErrorsAs(t *testing.T) {
	appErr := &AppError{
		Code:       "TEST_CODE",
		Message:    "test message",
		HTTPStatus: http.StatusNotFound,
	}

	wrapped := fmt.Errorf("context: %w", appErr)

	var target *AppError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should unwrap to *AppError")
	}
	if target.Code != "TEST_CODE" {
		t.Errorf("expected code TEST_CODE, got %q", target.Code)
	}
	if target.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", target.HTTPStatus)
	}
}

func TestSentinelErrors_Exist(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrInvalidTransition", ErrInvalidTransition},
		{"ErrOrderNotFound", ErrOrderNotFound},
		{"ErrInstrumentNotFound", ErrInstrumentNotFound},
		{"ErrPositionNotFound", ErrPositionNotFound},
		{"ErrInvalidQuantity", ErrInvalidQuantity},
		{"ErrInvalidPrice", ErrInvalidPrice},
		{"ErrDuplicateClientOrderID", ErrDuplicateClientOrderID},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Errorf("sentinel %s should not be nil", s.name)
			}
			var appErr *AppError
			if !errors.As(s.err, &appErr) {
				t.Errorf("sentinel %s should be *AppError", s.name)
			}
		})
	}
}

func TestSentinelErrors_HTTPStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{ErrInvalidTransition, http.StatusConflict},
		{ErrOrderNotFound, http.StatusNotFound},
		{ErrInstrumentNotFound, http.StatusNotFound},
		{ErrPositionNotFound, http.StatusNotFound},
		{ErrInvalidQuantity, http.StatusBadRequest},
		{ErrInvalidPrice, http.StatusBadRequest},
		{ErrDuplicateClientOrderID, http.StatusConflict},
	}

	for _, tt := range tests {
		var appErr *AppError
		if errors.As(tt.err, &appErr) {
			if appErr.HTTPStatus != tt.status {
				t.Errorf("%s: expected HTTP %d, got %d", appErr.Code, tt.status, appErr.HTTPStatus)
			}
		}
	}
}

func TestWriteError_AppError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := &AppError{
		Code:       "ORDER_NOT_FOUND",
		Message:    "order 123 not found",
		HTTPStatus: http.StatusNotFound,
	}

	WriteError(rr, appErr)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error.Code != "ORDER_NOT_FOUND" {
		t.Errorf("expected code ORDER_NOT_FOUND, got %q", body.Error.Code)
	}
	if body.Error.Message != "order 123 not found" {
		t.Errorf("expected message 'order 123 not found', got %q", body.Error.Message)
	}
}

func TestWriteError_GenericError(t *testing.T) {
	rr := httptest.NewRecorder()
	err := fmt.Errorf("something unexpected happened")

	WriteError(rr, err)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %q", body.Error.Code)
	}
}

func TestWriteError_WrappedAppError(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := fmt.Errorf("additional context: %w", ErrOrderNotFound)

	WriteError(rr, wrapped)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for wrapped AppError, got %d", rr.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Error.Code != "ORDER_NOT_FOUND" {
		t.Errorf("expected code ORDER_NOT_FOUND, got %q", body.Error.Code)
	}
}

func TestErrorsIs_WithWrappedSentinel(t *testing.T) {
	wrapped := fmt.Errorf("repo layer: %w", ErrOrderNotFound)
	if !errors.Is(wrapped, ErrOrderNotFound) {
		t.Error("errors.Is should match wrapped sentinel")
	}
}
