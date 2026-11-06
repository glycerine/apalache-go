package stopwatch

// Source: src/util/stopwatch.h.
// Translation of src/util/stopwatch.h's POSIX control flow.

import "errors"

var ErrStopwatchOwner = errors.New("g3: invalid stopwatch owner")

// Stopwatch is cold report metadata. Source reset clears only accumulated time;
// querying a running watch performs stop then start. Clock reads can fail in Go
// instead of using an uninitialized POSIX timespec after a system-call failure.
type Stopwatch struct {
	elapsed, start uint64
	running        bool
	clock          func() (uint64, error) // nil selects the actual platform clock.
}

type State struct {
	Elapsed uint64
	Start   uint64
	Running bool
}

func New(clock func() (uint64, error)) Stopwatch { return Stopwatch{clock: clock} }

func (w *Stopwatch) State() State {
	return State{Elapsed: w.elapsed, Start: w.start, Running: w.running}
}

func (w *Stopwatch) read() (uint64, error) {
	if w.clock != nil {
		return w.clock()
	}
	return platformNanoseconds()
}
func (w *Stopwatch) Start() error {
	if w.running {
		return nil
	}
	n, e := w.read()
	if e != nil {
		return e
	}
	w.start = n
	w.running = true
	return nil
}
func (w *Stopwatch) Stop() error {
	if !w.running {
		return nil
	}
	n, e := w.read()
	if e != nil {
		return e
	}
	// Preserve Linux's unsigned arithmetic and nanosecond borrow guard.
	w.elapsed += (n/1e9 - w.start/1e9) * 1e9
	if w.elapsed != 0 || n%1e9 >= w.start%1e9 {
		w.elapsed += n%1e9 - w.start%1e9
	}
	w.running = false
	return nil
}
func (w *Stopwatch) Reset()               { w.elapsed = 0 }
func (w *Stopwatch) Add(other *Stopwatch) { w.elapsed += other.elapsed }
func (w *Stopwatch) Seconds() (float64, error) {
	if w.running {
		if e := w.Stop(); e != nil {
			return 0, e
		}
		if e := w.Start(); e != nil {
			return 0, e
		}
	}
	return float64(w.elapsed) / 1e9, nil
}
func (w *Stopwatch) GetSeconds() (float64, error)        { return w.Seconds() }
func (w *Stopwatch) CurrentSeconds() (float64, error)    { return w.Seconds() }
func (w *Stopwatch) GetCurrentSeconds() (float64, error) { return w.Seconds() }
func (w *Stopwatch) Running() bool                       { return w.running }
func (w *Stopwatch) ElapsedNanoseconds() uint64          { return w.elapsed }

// ScopedWatch translates scoped_watch. The returned value borrows watch and
// should be closed with defer; Close is harmless when repeated.
type ScopedWatch struct {
	watch  *Stopwatch
	active bool
}

func NewScopedWatch(watch *Stopwatch, reset bool) (ScopedWatch, error) {
	if watch == nil {
		return ScopedWatch{}, ErrStopwatchOwner
	}
	if reset {
		watch.Reset()
	}
	if err := watch.Start(); err != nil {
		return ScopedWatch{}, err
	}
	return ScopedWatch{watch: watch, active: true}, nil
}

func (watch *ScopedWatch) Close() error {
	if watch == nil {
		return ErrStopwatchOwner
	}
	if !watch.active {
		return nil
	}
	if err := watch.watch.Stop(); err != nil {
		return err
	}
	watch.active = false
	watch.watch = nil
	return nil
}
