package shell

// Source: src/shell/smtlib_frontend.cpp context/file lifetime.

import (
	"errors"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// Context returns the live command context. It is useful to embedders that
// borrow solver state after Run; ownership remains unchanged.
func (f *SMTLIBFrontend) Context() (*cmd_context.Context, error) {
	if f == nil || f.closed || f.context == nil {
		return nil, ErrShellOptionService
	}
	return f.context, nil
}

// Close releases an input file opened by the frontend and a context created by
// the frontend. Caller-provided readers and contexts are borrowed.
func (f *SMTLIBFrontend) Close() (result error) {
	if f == nil || f.closed {
		return nil
	}
	f.closed = true
	if f.inputClose != nil {
		result = errors.Join(result, f.inputClose.Close())
		f.inputClose = nil
	}
	if f.owned && f.context != nil {
		result = errors.Join(result, f.context.Close())
	}
	f.context = nil
	f.input = nil
	f.output = nil
	return result
}
