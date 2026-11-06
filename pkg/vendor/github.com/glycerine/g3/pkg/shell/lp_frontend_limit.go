package shell

// Source: front_end_resource_limit, scoped_rlimit and scoped_timer.

import "github.com/glycerine/g3/pkg/util/control"

type lpFrontendLimit struct{ limit *control.Reslimit }

func (l lpFrontendLimit) CancelFlag() (bool, error) { return !l.limit.Inc(), nil }

func (f *LPFrontend) startLimits() error {
	rlimit, err := f.globalUint("rlimit")
	if err != nil {
		return err
	}
	if err = f.limit.Push(rlimit); err != nil {
		return err
	}
	timeout, err := f.globalUint("timeout")
	if err != nil {
		f.limit.Pop()
		return err
	}
	f.timer, err = control.NewScopedTimer(timeout, control.EventHandlerFunc(func(control.EventCaller) { f.limit.Cancel() }))
	if err != nil {
		f.limit.Pop()
	}
	return err
}

func (f *LPFrontend) stopLimits() (err error) {
	if f.timer != nil {
		err = f.timer.Close()
		f.timer = nil
	}
	if f.limit != nil && len(f.limit.Limits) != 0 {
		f.limit.Pop()
	}
	return err
}
