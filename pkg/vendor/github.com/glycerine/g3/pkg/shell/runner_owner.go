package shell

// Source: src/shell/main.cpp frontend/input/context destruction boundaries.

import "errors"

func (r *Runner) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	if r.running {
		return ErrShellOptionService
	}
	r.closed = true
	if r.inputClose != nil {
		result = errors.Join(result, r.inputClose.Close())
		r.inputClose = nil
	}
	if r.owned && r.context != nil {
		result = errors.Join(result, r.context.Close())
	}
	r.context = nil
	r.command = nil
	r.input = nil
	r.output = nil
	return result
}
