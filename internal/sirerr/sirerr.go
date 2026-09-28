package sirerr

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error class for CLI exit mapping.
type Code string

const (
	CodeInvalid     Code = "invalid"
	CodeNotFound    Code = "not_found"
	CodeUnavailable Code = "unavailable"
	CodeFailed      Code = "failed"
	CodeAI          Code = "ai"
	CodeAuth        Code = "auth"
	CodeNetwork     Code = "network"
)

// Error is a typed domain error with op and optional fields.
type Error struct {
	Code    Code
	Op      string
	Message string
	Fields  map[string]string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Wrap returns a typed error wrapping cause.
func Wrap(cause error, code Code, op, message string) *Error {
	return &Error{Code: code, Op: op, Message: message, Err: cause}
}

// New returns a typed error without a cause.
func New(code Code, op, message string) *Error {
	return &Error{Code: code, Op: op, Message: message}
}

// With attaches a field for diagnostics.
func (e *Error) With(key, value string) *Error {
	if e == nil {
		return nil
	}
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	e.Fields[key] = value
	return e
}

// AsCode extracts Code from err if present.
func AsCode(err error) (Code, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e.Code, true
	}
	return "", false
}

// ExitCode maps domain errors to process exit codes (aligned with emailops-cli).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	code, ok := AsCode(err)
	if !ok {
		return 1
	}
	switch code {
	case CodeInvalid:
		return 2
	case CodeNotFound:
		return 3
	case CodeAuth:
		return 4
	case CodeNetwork:
		return 5
	case CodeAI:
		return 6
	case CodeUnavailable:
		return 5
	default:
		return 1
	}
}
