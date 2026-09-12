package runbook

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	e := &Error{Type: ErrorTypeValidation, Message: "missing input: PUBLIC_IP"}
	assert.Equal(t, "validation error: missing input: PUBLIC_IP", e.Error())
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("boom")
	e := &Error{Type: ErrorTypeExecution, Message: "spawn failed", Cause: cause}

	assert.Same(t, cause, errors.Unwrap(e))
	assert.True(t, errors.Is(e, cause))
}
