package errors

import (
	stderrors "errors"
	"fmt"
)

// Kind classifies an error for CLI handling and reporting.
type Kind string

const (
	KindUserInput  Kind = "user_input"
	KindValidation Kind = "validation"
	KindNotFound   Kind = "not_found"
	KindInternal   Kind = "internal"
)

// AppError is a typed error with an operation name and optional wrapped cause.
type AppError struct {
	Kind Kind
	Op   string
	Msg  string
	Err  error
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Op, e.Msg)
	}

	if e.Msg == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}

	return fmt.Sprintf("%s: %s: %v", e.Op, e.Msg, e.Err)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// New builds an AppError value.
func New(kind Kind, op string, msg string, err error) *AppError {
	return &AppError{
		Kind: kind,
		Op:   op,
		Msg:  msg,
		Err:  err,
	}
}

// IsUserError reports whether an error was caused by user input or validation.
func IsUserError(err error) bool {
	var appErr *AppError
	if !stderrors.As(err, &appErr) {
		return false
	}

	switch appErr.Kind {
	case KindUserInput, KindValidation, KindNotFound:
		return true
	default:
		return false
	}
}
