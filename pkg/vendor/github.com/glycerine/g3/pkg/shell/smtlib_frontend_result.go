package shell

// Source: src/shell/smtlib_frontend.cpp parser/statistics completion path.

// SMTLIBFrontendResult records the two independently observable completion
// phases. Parse errors do not suppress requested final statistics.
type SMTLIBFrontendResult struct {
	Completed       bool
	DurationSeconds float64
	ParseError      error
	StatisticsError error
}

func (f *SMTLIBFrontend) Result() (SMTLIBFrontendResult, error) {
	if f == nil || f.closed {
		return SMTLIBFrontendResult{}, ErrShellOptionService
	}
	return f.result, nil
}
