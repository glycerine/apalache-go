package tactic

// Source: src/tactic/tactic_exception.h:25-34.

import (
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

const (
	TacticCanceledMessage          = messages.Canceled
	TacticMaxMemoryMessage         = messages.MaxMemory
	TacticMaxScopesMessage         = messages.MaxScopes
	TacticMaxStepsMessage          = messages.MaxSteps
	TacticMaxFramesMessage         = messages.MaxFrames
	TacticNoProofGenerationMessage = messages.NoProofGeneration
)

// TacticException is the source tactic_exception message copied from a caught
// z3_exception. It owns its immutable message and has no error code.
type TacticException struct {
	message string
	cause   error
}

func NewTacticException(message string) TacticException { return TacticException{message: message} }

// WrapTacticException keeps the source-visible message while preserving the
// Go cause for callers that must finish interrupted resource cleanup.
func WrapTacticException(cause error) TacticException {
	if cause == nil {
		return TacticException{}
	}
	return TacticException{message: cause.Error(), cause: cause}
}

func (e TacticException) Error() string                    { return e.message }
func (e TacticException) Message() string                  { return e.message }
func (e TacticException) Unwrap() error                    { return e.cause }
func (e TacticException) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e TacticException) HasErrorCode() bool               { return false }
