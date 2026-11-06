package shell

// Source: src/shell/smtlib_frontend.cpp top-level frontend invocation.

import (
	"errors"
	"io"
)

// RunSMTLIBStream is the composition helper for stdin and other borrowed
// streams. It owns the standalone context created by NewSMTLIBFrontend.
func RunSMTLIBStream(input io.Reader, output io.Writer, displayStatistics bool) (result error) {
	frontend, err := NewSMTLIBFrontend(SMTLIBFrontendConfig{
		Input:             input,
		Output:            output,
		DisplayStatistics: displayStatistics,
	})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, frontend.Close()) }()
	return frontend.Run()
}
