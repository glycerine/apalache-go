package polynomial

// Source: src/math/polynomial/upolynomial.cpp:40-55,174-189.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (fs *UPolynomialFactors) NumericExecuting() bool {
	return fs.sourceRunning || fs.sourceCleanupRunning
}
func (fs *UPolynomialFactors) NumericCallbackActive() bool {
	z := fs.manager.config.Integer
	return z.RuntimeOwner().FailureTransportActive() || z.BorrowTraceTags().OutputCallbackActive()
}
func (fs *UPolynomialFactors) bindSourceDestructor() error {
	m := fs.manager
	fs.sourcePrevious = m.sourceFactors
	m.sourceFactors = fs
	if e := m.config.Integer.BindCaller(&fs.sourceBindings[0], fs, 0); e != nil {
		fs.unbindSourceDestructor()
		return e
	}
	fs.sourceBound++
	if e := m.config.Integer.Natural.BindCaller(&fs.sourceBindings[1], fs, 0); e != nil {
		fs.unbindSourceDestructor()
		return e
	}
	fs.sourceBound++
	if e := m.modular.BindCaller(&fs.sourceBindings[2], fs, 0); e != nil {
		fs.unbindSourceDestructor()
		return e
	}
	fs.sourceBound++
	return nil
}
func (fs *UPolynomialFactors) unbindSourceDestructor() {
	for fs.sourceBound > 0 {
		fs.sourceBound--
		if e := fs.sourceBindings[fs.sourceBound].Unbind(); e != nil {
			panic(e)
		}
	}
	m := fs.manager
	if m.sourceFactors != fs {
		panic(ErrUPolynomialOwner)
	}
	m.sourceFactors = fs.sourcePrevious
	fs.sourcePrevious = nil
}

// SourceFinalize executes factors::~factors. It never acknowledges an already
// consumed NUM release or retires the Go owner/header. Close owns that cleanup.
func (fs *UPolynomialFactors) SourceFinalize() error {
	if e := fs.checkCleanupExecution(); e != nil {
		return e
	}
	if fs.sourceStarted {
		return fs.sourceCause
	}
	if fs.closing {
		return arena.ErrClosed
	}
	if e := fs.manager.check(); e != nil {
		return e
	}
	fs.sourceStarted, fs.sourceRunning, fs.closing = true, true, true
	defer func() { fs.sourceRunning = false }()
	if e := fs.bindSourceDestructor(); e != nil {
		fs.sourceCause = e
		return e
	}
	defer fs.unbindSourceDestructor()
	m := fs.manager
	fail := func(e error) error { fs.sourceCause = e; return e }
	for fs.sourcePhase < 6 {
		switch fs.sourcePhase {
		case 0:
			for fs.sourceFactor < fs.count {
				s := &fs.factors[fs.sourceFactor]
				if !fs.sourceResetAdmitted {
					// reset -> set_size(0): even the true old_sz>=0 assertion consults
					// its actual process owner before any source NUM destruction.
					if _, e := refinementSourceAssertionsEnabled(); e != nil {
						return fail(e)
					}
					fs.sourceResetAdmitted = true
				}
				for fs.sourceIndex < s.size {
					values := m.values(s)
					if uint64(fs.sourceIndex) >= uint64(len(values)) {
						return fail(arena.ErrBounds)
					}
					if e := unumeral.SourceIntegerDel(m.config.Integer, &values[fs.sourceIndex]); e != nil {
						return fail(e)
					}
					fs.sourceIndex++
				}
				s.size = 0
				fs.sourceIndex = 0
				fs.sourceResetAdmitted = false
				fs.sourceFactor++
			}
		case 1:
			for i := uint32(0); i < fs.count; i++ {
				m.freeBacking(&fs.factors[i])
			}
			fs.count = 0
			fs.factors = fs.factors[:0]
			fs.degrees = fs.degrees[:0]
		case 2:
			if e := m.modular.SetInt32(&fs.constant, 1); e != nil {
				return fail(e)
			}
			fs.totalFactors, fs.totalDegree = 0, 0
		case 3:
			if e := unumeral.SourceIntegerDel(m.config.Integer, &fs.constant); e != nil {
				return fail(e)
			}
		case 4:
			fs.degrees = nil
		case 5:
			fs.factors = nil
		}
		fs.sourcePhase++
	}
	fs.sourceFinalized = true
	return nil
}

// closeAfterSource owns only this factors object's actual vector/constant
// fields. It remains available after the borrowed UP modular member has ended.
func (fs *UPolynomialFactors) closeAfterSource() error {
	if e := fs.checkCleanupExecution(); e != nil {
		return e
	}
	m := fs.manager
	if m.closed || m.config.Integer.Closed() {
		return arena.ErrClosed
	}
	fs.sourceCleanupRunning = true
	defer func() { fs.sourceCleanupRunning = false }()
	z := m.config.Integer
	if e := z.BindCaller(&fs.sourceBindings[0], fs, 0); e != nil {
		return e
	}
	defer func() {
		if e := fs.sourceBindings[0].Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := z.Natural.BindCaller(&fs.sourceBindings[1], fs, 0); e != nil {
		return e
	}
	defer func() {
		if e := fs.sourceBindings[1].Unbind(); e != nil {
			panic(e)
		}
	}()
	previous := m.sourceFactors
	m.sourceFactors = fs
	defer func() { m.sourceFactors = previous }()
	for i := range fs.factors {
		s := &fs.factors[i]
		for j := range m.values(s) {
			if e := m.config.Integer.Del(&m.values(s)[j]); e != nil {
				return e
			}
		}
		m.freeBacking(s)
	}
	if e := m.config.Integer.Del(&fs.constant); e != nil {
		return e
	}
	fs.factors = nil
	fs.degrees = nil
	fs.count = 0
	fs.totalFactors, fs.totalDegree = 0, 0
	m.config.Integer.Budget().Release(fs.header)
	fs.header = 0
	m.liveFactors--
	fs.closed = true
	return nil
}
