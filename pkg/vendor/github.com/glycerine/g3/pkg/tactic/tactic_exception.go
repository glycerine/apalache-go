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
type TacticException struct{ message string }

func NewTacticException(message string) TacticException { return TacticException{message: message} }

func (e TacticException) Error() string                    { return e.message }
func (e TacticException) Message() string                  { return e.message }
func (e TacticException) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e TacticException) HasErrorCode() bool               { return false }
