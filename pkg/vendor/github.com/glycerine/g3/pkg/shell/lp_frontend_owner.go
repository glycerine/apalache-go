package shell

// Source destruction order: solver, reader, timer/scoped rlimit, parameters.

import "errors"

func (f *LPFrontend) Close() (err error) {
	if f == nil || f.closed {
		return nil
	}
	if f.running {
		return ErrShellOptionService
	}
	f.closed = true
	if f.solver != nil {
		err = errors.Join(err, f.solver.Close())
		f.solver = nil
	}
	if f.reader != nil {
		err = errors.Join(err, f.reader.Close())
		f.reader = nil
	}
	err = errors.Join(err, f.stopLimits())
	if f.limit != nil {
		f.limit.Close()
		f.limit = nil
	}
	if f.params != nil {
		err = errors.Join(err, f.params.Close())
		f.params = nil
	}
	return err
}
