package polynomial

// Source: src/math/polynomial/upolynomial.cpp:97-106.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyFactorsDisplayScope struct {
	owner                       *UPolynomialFactors
	invocation                  uint64
	active, executing, cleaning bool
	outputDepth                 uint32
	polynomial                  upolyDisplayInvocationReceipt
	previous                    *upolyFactorsDisplayScope
	bindings                    [3]unumeral.NumericCallerBinding
	bound                       uint8
}

type upolyFactorsDisplayReceipt struct {
	owner      *UPolynomialFactors
	frame      *upolyFactorsDisplayScope
	invocation uint64
}

func (q *upolyFactorsDisplayScope) NumericExecuting() bool { return q.executing || q.cleaning }
func (q *upolyFactorsDisplayScope) pending() bool {
	return q.owner != nil && q.active && q.owner.manager.displayReceiptPending(&q.polynomial)
}
func (q *upolyFactorsDisplayScope) callback() bool {
	z := q.owner.manager.config.Integer
	return q.outputDepth != 0 || z.RuntimeOwner().FailureTransportActive() || z.BorrowTraceTags().OutputCallbackActive()
}
func (q *upolyFactorsDisplayScope) bind() error {
	m := q.owner.manager
	q.previous = m.factorsDisplay
	m.factorsDisplay = q
	if e := m.config.Integer.BindCaller(&q.bindings[0], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	if e := m.config.Integer.Natural.BindCaller(&q.bindings[1], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	if e := m.modular.BindCaller(&q.bindings[2], q, 0); e != nil {
		q.unbind()
		return e
	}
	q.bound++
	return nil
}
func (q *upolyFactorsDisplayScope) unbind() {
	for q.bound != 0 {
		q.bound--
		if e := q.bindings[q.bound].Unbind(); e != nil {
			panic(e)
		}
	}
	m := q.owner.manager
	if m.factorsDisplay != q {
		panic(ErrUPolynomialOwner)
	}
	m.factorsDisplay = q.previous
	q.previous = nil
}
func (r *upolyFactorsDisplayReceipt) matches() bool {
	return r != nil && r.owner != nil && r.owner.self == r.owner && r.frame == &r.owner.displayScope && r.invocation != 0 && r.invocation == r.frame.invocation
}
func (r *upolyFactorsDisplayReceipt) pending() bool { return r.matches() && r.frame.pending() }
func (r *upolyFactorsDisplayReceipt) sourceBlocker() error {
	if !r.pending() {
		return nil
	}
	if e := r.owner.manager.displayReceiptSourceCause(&r.frame.polynomial); e != nil {
		return e
	}
	return ErrUPolynomialCleanupPending
}
func (r *upolyFactorsDisplayReceipt) cleanup() error {
	if !r.matches() || !r.frame.active {
		return nil
	}
	return r.owner.CleanupDisplay()
}

func (fs *UPolynomialFactors) DisplayInto(out unumeral.RationalSourceStream) error {
	return fs.displayIntoWithReceipt(out, nil)
}
func (fs *UPolynomialFactors) displayIntoWithReceipt(out unumeral.RationalSourceStream, receipt *upolyFactorsDisplayReceipt) error {
	if e := fs.check(); e != nil {
		return e
	}
	q := &fs.displayScope
	if q.active || q.invocation == ^uint64(0) {
		return ErrUPolynomialCleanupPending
	}
	if out == nil {
		return arena.ErrConfig
	}
	*q = upolyFactorsDisplayScope{owner: fs, invocation: q.invocation + 1, active: true, executing: true}
	if e := q.bind(); e != nil {
		q.active, q.executing = false, false
		return e
	}
	if receipt != nil {
		*receipt = upolyFactorsDisplayReceipt{fs, q, q.invocation}
	}
	defer func() {
		q.unbind()
		q.executing = false
		if q.pending() {
			fs.manager.pendingFactorsDisplay = q
		} else {
			q.active = false
		}
	}()
	wrapped := upolyFactorsDisplayStream{q, out}
	// The source to_string completes before the outer string insertion.
	text, e := fs.manager.modular.String(&fs.constant)
	if e != nil {
		return e
	}
	if e = wrapped.AppendCString(text); e != nil {
		return e
	}
	for i := uint32(0); i < fs.count; i++ {
		if e = wrapped.AppendCString(" * ("); e != nil {
			return e
		}
		if uint64(i) >= uint64(len(fs.factors)) {
			return arena.ErrBounds
		}
		if e = fs.manager.displayIntoWithReceipt(fs.manager.stateView(&fs.factors[i]), wrapped, "x", false, &q.polynomial); e != nil {
			return e
		}
		if e = wrapped.AppendCString(")^"); e != nil {
			return e
		}
		degree, e := fs.multiplicity(i)
		if e != nil {
			return e
		}
		if e = displayUnsigned(wrapped, degree); e != nil {
			return e
		}
	}
	return nil
}
func (fs *UPolynomialFactors) CleanupDisplay() error {
	if fs == nil || fs.self != fs || fs.closed {
		return ErrUPolynomialOwner
	}
	if fs.manager.factorTrace != nil && fs.manager.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	q := &fs.displayScope
	if q.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if !q.active {
		return nil
	}
	q.cleaning = true
	if e := q.bind(); e != nil {
		q.cleaning = false
		return e
	}
	defer func() { q.unbind(); q.cleaning = false }()
	if e := fs.manager.cleanupDisplayReceipt(&q.polynomial); e != nil {
		return e
	}
	q.active = false
	if fs.manager.pendingFactorsDisplay == q {
		fs.manager.pendingFactorsDisplay = nil
	}
	return nil
}

type upolyFactorsDisplayStream struct {
	scope *upolyFactorsDisplayScope
	out   unumeral.RationalSourceStream
}

func (s upolyFactorsDisplayStream) AppendCString(v string) error {
	s.scope.outputDepth++
	defer func() { s.scope.outputDepth-- }()
	return s.out.AppendCString(v)
}
func (s upolyFactorsDisplayStream) AppendInt32(v int32) error {
	s.scope.outputDepth++
	defer func() { s.scope.outputDepth-- }()
	return s.out.AppendInt32(v)
}
func (s upolyFactorsDisplayStream) AppendNumeralText(v string) error {
	s.scope.outputDepth++
	defer func() { s.scope.outputDepth-- }()
	return s.out.AppendNumeralText(v)
}
