// Package apperror defines typed application errors with HTTP status mapping.
package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// AppError is a structured application error with an error code, message, and HTTP status.
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Sentinel errors for the order-management domain.
var (
	ErrInvalidTransition    = &AppError{Code: "INVALID_TRANSITION", Message: "invalid order state transition", HTTPStatus: http.StatusConflict}
	ErrOrderNotFound        = &AppError{Code: "ORDER_NOT_FOUND", Message: "order not found", HTTPStatus: http.StatusNotFound}
	ErrInstrumentNotFound   = &AppError{Code: "INSTRUMENT_NOT_FOUND", Message: "instrument not found", HTTPStatus: http.StatusNotFound}
	ErrPositionNotFound     = &AppError{Code: "POSITION_NOT_FOUND", Message: "position not found", HTTPStatus: http.StatusNotFound}
	ErrInvalidQuantity      = &AppError{Code: "INVALID_QUANTITY", Message: "quantity must be positive", HTTPStatus: http.StatusBadRequest}
	ErrInvalidPrice         = &AppError{Code: "INVALID_PRICE", Message: "invalid price", HTTPStatus: http.StatusBadRequest}
	ErrDuplicateClientOrderID = &AppError{Code: "DUPLICATE_CLIENT_ORDER_ID", Message: "duplicate client order ID", HTTPStatus: http.StatusConflict}
)

// errorResponse is the JSON envelope for error responses.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a JSON error response. If err is (or wraps) an *AppError,
// the appropriate HTTP status and error code are used. Otherwise, a generic
// 500 Internal Server Error is returned.
func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = &AppError{
			Code:       "INTERNAL_ERROR",
			Message:    "internal server error",
			HTTPStatus: http.StatusInternalServerError,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)

	resp := errorResponse{
		Error: errorBody{
			Code:    appErr.Code,
			Message: appErr.Message,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
