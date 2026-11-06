package api

import (
	"errors"
	"fmt"
)

var ErrZ3ReplayState = errors.New("api: invalid Z3 replay state")
var ErrZ3ReplayReference = errors.New("api: invalid Z3 replay argument reference")

type Z3ReplayError struct {
	Line    uint32
	Message string
}

// Z3ReplayCommandError is the Go counterpart of a z3_exception thrown by a
// registered API command. The source prints it and continues replay; parser,
// reference, and z3_error failures remain fatal.
type Z3ReplayCommandError struct{ Message string }

func (e *Z3ReplayCommandError) Error() string { return e.Message }

func (e *Z3ReplayError) Error() string {
	return fmt.Sprintf("Z3 replay line %d: %s", e.Line, e.Message)
}

func (r *Z3Replayer) failure(message string) error {
	return &Z3ReplayError{Line: r.Line(), Message: message}
}
