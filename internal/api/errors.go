package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError represents an error response from the API.
type APIError struct {
	Code       string `json:"error"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAPIError creates a new API error with the given code, message, and status.
func NewAPIError(code, message string, statusCode int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Predefined API errors
var (
	ErrNotFound = &APIError{
		Code:       "not_found",
		Message:    "Resource not found",
		StatusCode: http.StatusNotFound,
	}

	ErrBadRequest = &APIError{
		Code:       "bad_request",
		Message:    "Invalid request",
		StatusCode: http.StatusBadRequest,
	}

	ErrMissingParameter = &APIError{
		Code:       "missing_parameter",
		Message:    "Required parameter is missing",
		StatusCode: http.StatusBadRequest,
	}

	ErrInvalidParameter = &APIError{
		Code:       "invalid_parameter",
		Message:    "Parameter value is invalid",
		StatusCode: http.StatusBadRequest,
	}

	ErrInternal = &APIError{
		Code:       "internal_error",
		Message:    "An unexpected error occurred",
		StatusCode: http.StatusInternalServerError,
	}

	ErrRateLimited = &APIError{
		Code:       "rate_limit_exceeded",
		Message:    "Too many requests. Please slow down.",
		StatusCode: http.StatusTooManyRequests,
	}

	ErrServiceUnavailable = &APIError{
		Code:       "service_unavailable",
		Message:    "Service temporarily unavailable",
		StatusCode: http.StatusServiceUnavailable,
	}
)

// RespondWithError writes an error response to the client.
func RespondWithError(c *gin.Context, err *APIError) {
	requestID, _ := c.Get("request_id")
	reqIDStr, _ := requestID.(string)

	c.JSON(err.StatusCode, ErrorResponse{
		Error:     err.Code,
		Message:   err.Message,
		RequestID: reqIDStr,
	})
}

// RespondWithValidationError writes a validation error for a specific field.
func RespondWithValidationError(c *gin.Context, field, message string) {
	err := NewAPIError(
		"invalid_parameter",
		fmt.Sprintf("Invalid value for '%s': %s", field, message),
		http.StatusBadRequest,
	)
	RespondWithError(c, err)
}

// RespondWithMissingParam writes a missing parameter error.
func RespondWithMissingParam(c *gin.Context, param string) {
	err := NewAPIError(
		"missing_parameter",
		fmt.Sprintf("Required parameter '%s' is missing", param),
		http.StatusBadRequest,
	)
	RespondWithError(c, err)
}

// RespondWithNotFound writes a not found error for a specific resource.
func RespondWithNotFound(c *gin.Context, resource, identifier string) {
	err := NewAPIError(
		"not_found",
		fmt.Sprintf("%s '%s' not found", resource, identifier),
		http.StatusNotFound,
	)
	RespondWithError(c, err)
}
