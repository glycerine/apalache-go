package duality

// profileScope restores the current timer on every search return, including
// cancellation and inference errors.
type profileScope struct {
	profiler *Profiler
	name     string
}

// WithProfiler lends a profiler to operations on this context for the duration
// of fn, restoring any previous profiler even if fn fails or panics.
func (c *Context) WithProfiler(profiler *Profiler, fn func() error) error {
	if err := c.open(); err != nil {
		return err
	}
	if fn == nil {
		return ErrWrapperOwner
	}
	previous := c.activeProfiler
	c.activeProfiler = profiler
	defer func() { c.activeProfiler = previous }()
	return fn()
}

func startProfile(context *Context, name string) (*profileScope, error) {
	if context == nil || context.activeProfiler == nil {
		return nil, nil
	}
	if err := context.activeProfiler.TimerStart(name); err != nil {
		return nil, err
	}
	return &profileScope{profiler: context.activeProfiler, name: name}, nil
}
func (s *profileScope) Close() error {
	if s == nil || s.profiler == nil {
		return nil
	}
	p := s.profiler
	s.profiler = nil
	return p.TimerStop(s.name)
}
