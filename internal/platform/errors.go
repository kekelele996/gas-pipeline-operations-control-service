package platform

import (
	"errors"
	"fmt"
)

// Sentinel errors used across all domain packages. They are comparable with
// errors.Is so the HTTP layer can map them to status codes without importing
// domain types.
var (
	// ErrNotFound is returned when a referenced entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned for unique-constraint or version conflicts.
	ErrConflict = errors.New("conflict")
	// ErrInvalid is returned for malformed or semantically invalid input.
	ErrInvalid = errors.New("invalid input")
	// ErrState is returned when a state-machine transition is illegal.
	ErrState = errors.New("illegal state transition")
	// ErrUnauthorized is returned when an actor lacks permission for an action.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrExhausted is returned when a capacity / quota is exceeded.
	ErrExhausted = errors.New("capacity exhausted")
)

// Error is a domain-aware error type that wraps a sentinel category and
// carries a human-readable message plus optional context. It implements Is so
// errors.Is(err, ErrNotFound) matches when Category == ErrNotFound.
type Error struct {
	Category error
	Message  string
	Wrapped  error
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Category, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

func (e *Error) Unwrap() error {
	if e.Wrapped != nil {
		return e.Wrapped
	}
	return e.Category
}

func (e *Error) Is(target error) bool {
	return errors.Is(e.Category, target) || errors.Is(e.Wrapped, target)
}

// Newf builds a typed Error with a formatted message under the given category.
func Newf(category error, format string, args ...any) *Error {
	return &Error{Category: category, Message: fmt.Sprintf(format, args...)}
}

// Wrap attaches a message and an underlying error to a category.
func Wrap(category error, msg string, wrapped error) *Error {
	return &Error{Category: category, Message: msg, Wrapped: wrapped}
}

// NotFoundf is a convenience constructor for not-found errors.
func NotFoundf(format string, args ...any) *Error {
	return Newf(ErrNotFound, format, args...)
}

// Conflictf is a convenience constructor for conflict errors.
func Conflictf(format string, args ...any) *Error {
	return Newf(ErrConflict, format, args...)
}

// Invalidf is a convenience constructor for invalid-input errors.
func Invalidf(format string, args ...any) *Error {
	return Newf(ErrInvalid, format, args...)
}

// Statef is a convenience constructor for illegal-state errors.
func Statef(format string, args ...any) *Error {
	return Newf(ErrState, format, args...)
}

// Category returns the sentinel category for err, or nil if err is not a
// *platform.Error or wraps no known sentinel.
func Category(err error) error {
	if err == nil {
		return nil
	}
	var pe *Error
	if errors.As(err, &pe) {
		return pe.Category
	}
	return err
}

// Exhaustedf returns a categorized error for capacity/quota exhaustion.
func Exhaustedf(format string, args ...any) *Error {
	return Newf(ErrExhausted, format, args...)
}
