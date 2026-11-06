package api

// Source: src/api/api_datalog.cpp:357-400.

import (
	"io"
)

// LoadSMT2 parses fixedpoint declarations, rules, background assertions, and
// fixedpoint scopes into this object. Query and check commands are parsed but
// not executed, matching the native fixedpoint-from-stream loading phase.
func (f *APIFixedpoint) LoadSMT2(input io.Reader) error {
	queries, err := f.LoadSMT2WithQueries(input)
	if err != nil {
		return err
	}
	return queries.Close()
}
