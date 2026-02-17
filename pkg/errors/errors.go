package errors

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode defines specific error codes
type ErrorCode string

const (
	// Request errors (400)
	ErrCodeMaxTokensExceeded ErrorCode = "max_tokens_exceeded"
	ErrCodeRequestTooLarge   ErrorCode = "request_too_large"
	ErrCodeInvalidRequest    ErrorCode = "invalid_request"
	ErrCodeInvalidModel      ErrorCode = "invalid_model"
	ErrCodeInvalidParameters ErrorCode = "invalid_parameters"

	// Auth errors (401)
	ErrCodeInvalidAPIKey ErrorCode = "invalid_api_key"
	ErrCodeKeyExpired    ErrorCode = "key_expired"
	ErrCodeMissingAPIKey ErrorCode = "missing_api_key"

	// Permission errors (403)
	ErrCodeKeyDisabled      ErrorCode = "key_disabled"
	ErrCodePermissionDenied ErrorCode = "permission_denied"

	// Rate limit errors (429)
	ErrCodeRateLimitExceeded ErrorCode = "rate_limit_exceeded"
	ErrCodeQuotaExceeded     ErrorCode = "quota_exceeded"

	// Server errors (500)
	ErrCodeInternalError ErrorCode = "internal_error"
	ErrCodeProviderError ErrorCode = "provider_error"

	// Gateway errors (502, 503)
	ErrCodeBadGateway         ErrorCode = "bad_gateway"
	ErrCodeModelOverloaded    ErrorCode = "model_overloaded"
	ErrCodeModelNotAvailable  ErrorCode = "model_not_available"
	ErrCodeServiceUnavailable ErrorCode = "service_unavailable"

	// Cache errors
	ErrCodeCacheError ErrorCode = "cache_error"

	// Circuit breaker errors
	ErrCodeCircuitOpen ErrorCode = "circuit_open"
)

// ErrorType defines the error type field
type ErrorType string

const (
	ErrTypeInvalidRequestError ErrorType = "invalid_request_error"
	ErrTypeInvalidAPIKey       ErrorType = "invalid_api_key"
	ErrTypePermissionError     ErrorType = "permission_error"
	ErrTypeRateLimitError      ErrorType = "rate_limit_error"
	ErrTypeServerError         ErrorType = "server_error"
	ErrTypeServiceUnavailable  ErrorType = "service_unavailable"
	ErrTypeInternalError       ErrorType = "internal_error"
)

// GatewayError represents a gateway error response
type GatewayError struct {
	Message string    `json:"message"`
	Type    ErrorType `json:"type"`
	Code    ErrorCode `json:"code"`
}

// ErrorResponse represents the OpenAI-compatible error response
type ErrorResponse struct {
	Error GatewayError `json:"error"`
}

// NewError creates a new GatewayError
func NewError(message string, errorType ErrorType, code ErrorCode) GatewayError {
	return GatewayError{
		Message: message,
		Type:    errorType,
		Code:    code,
	}
}

// ToErrorResponse converts to OpenAI-compatible error response
func (e GatewayError) ToErrorResponse() ErrorResponse {
	return ErrorResponse{
		Error: e,
	}
}

// HTTPStatus returns the HTTP status code for the error
func (e GatewayError) HTTPStatus() int {
	switch e.Type {
	case ErrTypeInvalidRequestError, ErrTypeInvalidAPIKey:
		return http.StatusBadRequest
	case ErrTypePermissionError:
		return http.StatusForbidden
	case ErrTypeRateLimitError:
		return http.StatusTooManyRequests
	case ErrTypeServerError:
		return http.StatusInternalServerError
	case ErrTypeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Common error constructors

// InvalidAPIKey returns an invalid API key error
func InvalidAPIKey(message string) GatewayError {
	return NewError(message, ErrTypeInvalidAPIKey, ErrCodeInvalidAPIKey)
}

// KeyExpired returns a key expired error
func KeyExpired(message string) GatewayError {
	return NewError(message, ErrTypeInvalidAPIKey, ErrCodeKeyExpired)
}

// KeyDisabled returns a key disabled error
func KeyDisabled(message string) GatewayError {
	return NewError(message, ErrTypePermissionError, ErrCodeKeyDisabled)
}

// RateLimitExceeded returns a rate limit exceeded error
func RateLimitExceeded(message string) GatewayError {
	return NewError(message, ErrTypeRateLimitError, ErrCodeRateLimitExceeded)
}

// QuotaExceeded returns a quota exceeded error
func QuotaExceeded(message string) GatewayError {
	return NewError(message, ErrTypeRateLimitError, ErrCodeQuotaExceeded)
}

// InvalidRequest returns an invalid request error
func InvalidRequest(message string) GatewayError {
	return NewError(message, ErrTypeInvalidRequestError, ErrCodeInvalidRequest)
}

// MaxTokensExceeded returns a max tokens exceeded error
func MaxTokensExceeded(message string) GatewayError {
	return NewError(message, ErrTypeInvalidRequestError, ErrCodeMaxTokensExceeded)
}

// RequestTooLarge returns a request too large error
func RequestTooLarge(message string) GatewayError {
	return NewError(message, ErrTypeInvalidRequestError, ErrCodeRequestTooLarge)
}

// InvalidModel returns an invalid model error
func InvalidModel(message string) GatewayError {
	return NewError(message, ErrTypeInvalidRequestError, ErrCodeInvalidModel)
}

// InternalError returns an internal server error
func InternalError(message string) GatewayError {
	return NewError(message, ErrTypeServerError, ErrCodeInternalError)
}

// ProviderError returns a provider error
func ProviderError(message string) GatewayError {
	return NewError(message, ErrTypeServerError, ErrCodeProviderError)
}

// BadGateway returns a bad gateway error
func BadGateway(message string) GatewayError {
	return NewError(message, ErrTypeServerError, ErrCodeBadGateway)
}

// ModelOverloaded returns a model overloaded error
func ModelOverloaded(message string) GatewayError {
	return NewError(message, ErrTypeServiceUnavailable, ErrCodeModelOverloaded)
}

// ModelNotAvailable returns a model not available error
func ModelNotAvailable(message string) GatewayError {
	return NewError(message, ErrTypeServiceUnavailable, ErrCodeModelNotAvailable)
}

// ServiceUnavailable returns a service unavailable error
func ServiceUnavailable(message string) GatewayError {
	return NewError(message, ErrTypeServiceUnavailable, ErrCodeServiceUnavailable)
}

// CircuitOpen returns a circuit breaker open error
func CircuitOpen(message string) GatewayError {
	return NewError(message, ErrTypeServiceUnavailable, ErrCodeCircuitOpen)
}

// CacheError returns a cache error
func CacheError(message string) GatewayError {
	return NewError(message, ErrTypeServerError, ErrCodeCacheError)
}

// JSON returns the error as a JSON response
func (e GatewayError) JSON(c *gin.Context) {
	c.JSON(e.HTTPStatus(), e.ToErrorResponse())
}
