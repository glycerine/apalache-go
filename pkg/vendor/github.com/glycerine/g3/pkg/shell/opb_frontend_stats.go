package shell

import "time"

func (f *OPBFrontend) Elapsed() (time.Duration, error) {
	if f == nil || !f.finished {
		return 0, ErrOPBFrontend
	}
	return f.elapsed, nil
}
