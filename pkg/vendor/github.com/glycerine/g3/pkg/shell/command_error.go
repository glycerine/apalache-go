package shell

// Source: src/shell/main.cpp:49-53 and invalid-option branch.

import (
	"errors"
	"io"
)

// CommandLineError preserves the source diagnostic while remaining
// classifiable by the parser's sentinel error.
type CommandLineError struct {
	Message string
	Cause   error
}

func (e *CommandLineError) Error() string { return e.Message }
func (e *CommandLineError) Unwrap() error { return e.Cause }

func sourceCommandLineError(cause error, message string) error {
	return &CommandLineError{Message: message, Cause: cause}
}

var ErrShellInputConflict = errors.New("shell: input file and standard input are both selected")
var ErrShellInputMissing = errors.New("shell: neither input file nor standard input is selected")

func WriteCommandError(out io.Writer, message string) error {
	if out == nil {
		return ErrShellOptionService
	}
	return writeShellText(out, "Error: "+message+"\nFor usage information: z3 -h\n")
}
