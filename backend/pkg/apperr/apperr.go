// Package apperr defines typed application errors that travel unchanged over
// NATS, HTTP and Centrifugo RPC.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	Invalid           Code = "invalid"
	Unauthorized      Code = "unauthorized"
	Forbidden         Code = "forbidden"
	NotFound          Code = "not_found"
	Conflict          Code = "conflict"
	InsufficientFunds Code = "insufficient_funds"
	Cooldown          Code = "cooldown"
	RateLimited       Code = "rate_limited"
	Unavailable       Code = "unavailable"
	Internal          Code = "internal"
)

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func New(c Code, format string, args ...any) *Error {
	return &Error{Code: c, Message: fmt.Sprintf(format, args...)}
}

// From converts any error into an *Error, hiding internal details.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: Internal, Message: "internal error"}
}

func Is(err error, c Code) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == c
}

// HTTPStatus maps a code to an HTTP status.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case Invalid:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case NotFound:
		return http.StatusNotFound
	case Conflict:
		return http.StatusConflict
	case InsufficientFunds:
		return http.StatusPaymentRequired
	case Cooldown, RateLimited:
		return http.StatusTooManyRequests
	case Unavailable:
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// RPCCode maps to a Centrifugo application error code (must be >= 400).
func (e *Error) RPCCode() uint32 { return uint32(e.HTTPStatus()) + 1000 }
