package runbook

import "fmt"

type ErrorType string

const (
	ErrorTypeParse      ErrorType = "parse"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeTemplate   ErrorType = "template"
	ErrorTypeExecution  ErrorType = "execution"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeNotFound   ErrorType = "not_found"
)

type Error struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}
