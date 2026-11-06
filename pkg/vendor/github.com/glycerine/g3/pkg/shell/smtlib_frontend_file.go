package shell

// Source: src/shell/smtlib_frontend.cpp:39-44 file frontend selection.

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
)

type SMTLIBFileFrontendConfig struct {
	Context           *cmd_context.Context
	Output            io.Writer
	Diagnostics       io.Writer
	DisplayStatistics bool
	Timer             *TimeoutService
	Clock             func() (uint64, error)
}

// SMTLIBFileOpenError carries the filename needed for the native shell's
// `(error "failed to open file '…'")` diagnostic and ERR_OPEN_FILE status.
type SMTLIBFileOpenError struct {
	Name  string
	Cause error
}

func (e *SMTLIBFileOpenError) Error() string {
	return fmt.Sprintf("failed to open file '%s': %v", e.Name, e.Cause)
}
func (e *SMTLIBFileOpenError) Unwrap() error { return e.Cause }

// OpenSMTLIBFrontend opens name and transfers the file lifetime to the
// returned frontend. Failed construction closes the file before returning.
func OpenSMTLIBFrontend(name string, c SMTLIBFileFrontendConfig) (f *SMTLIBFrontend, result error) {
	if name == "" || c.Output == nil {
		return nil, ErrShellOptionService
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, &SMTLIBFileOpenError{Name: name, Cause: err}
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, file.Close())
		}
	}()
	f, err = NewSMTLIBFrontend(SMTLIBFrontendConfig{
		Context:           c.Context,
		Input:             file,
		Output:            c.Output,
		Diagnostics:       c.Diagnostics,
		DisplayStatistics: c.DisplayStatistics,
		Timer:             c.Timer,
		Clock:             c.Clock,
	})
	if err != nil {
		return nil, err
	}
	f.inputClose = file
	return f, nil
}
