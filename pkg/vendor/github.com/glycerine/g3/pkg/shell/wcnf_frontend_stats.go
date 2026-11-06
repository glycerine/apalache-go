package shell

import "time"

func (f *WCNFFrontend) Elapsed() (time.Duration, error) {
	if f == nil || !f.finished {
		return 0, ErrWCNFFrontend
	}
	return f.elapsed, nil
}
