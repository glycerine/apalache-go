package polynomial

// Source: src/util/obj_ref.h; src/util/ref_vector.h; src/math/polynomial/polynomial.h:108-109.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type polynomialReference struct {
	cleanupCause                                error
	value, incoming                             Polynomial
	decremented, incomingDecremented, assigning bool
}

func polynomialReferenceDrop(m *MultivariatePolynomialCore, p *Polynomial, started *bool) error {
	if *p == (Polynomial{}) {
		*started = false
		return nil
	}
	if *started {
		if e := m.CleanupValue(p); e != nil {
			return e
		}
		*started = false
		return nil
	}
	var outcome polynomialDecrementOutcome
	e := m.decRefCoreOutcome(*p, &outcome)
	if e != nil {
		// This caller's completed last DEC grants the owning continuation,
		// including a stop in TRACE before callbacks/stage publication. A
		// refused call cannot adopt an older deletion's descriptor state.
		if outcome.consumed && outcome.last {
			*started = true
		}
		return e
	}
	*p = Polynomial{}
	return nil
}
func (r *polynomialReference) assign(m *MultivariatePolynomialCore, p Polynomial) error {
	return r.assignWithFactoryReceipt(m, p, nil)
}
func (r *polynomialReference) assignWithFactoryReceipt(m *MultivariatePolynomialCore, p Polynomial, receipt *polynomialFactoryInvocationReceipt) error {
	if r.assigning {
		return ErrPolynomialPending
	}
	if m.matchesFactoryInvocation(receipt) && m.factoryInvocation.result != p {
		return ErrPolynomialOwner
	}
	if p != (Polynomial{}) {
		if e := m.incRefCore(p); e != nil {
			return e
		}
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 && f.freshResult == p {
		f.freshResult = Polynomial{}
	}
	r.incoming = p
	r.assigning = true
	// Actual INC has completed and this real caller reference now owns incoming.
	if e := m.transferFactoryResult(receipt, p); e != nil {
		return e
	}
	if e := polynomialReferenceDrop(m, &r.value, &r.decremented); e != nil {
		r.cleanupCause = e
		return e
	}
	r.value = r.incoming
	r.incoming = Polynomial{}
	r.assigning = false
	return nil
}
func (r *polynomialReference) closeValue(m *MultivariatePolynomialCore, owning bool) error {
	if r.decremented && !owning {
		return r.cleanupCause
	}
	if e := polynomialReferenceDrop(m, &r.value, &r.decremented); e != nil {
		r.cleanupCause = e
		return e
	}
	r.cleanupCause = nil
	return nil
}
func (r *polynomialReference) abort(m *MultivariatePolynomialCore) error {
	if !r.assigning {
		return nil
	}
	if e := polynomialReferenceDrop(m, &r.incoming, &r.incomingDecremented); e != nil {
		return e
	}
	r.assigning = false
	return nil
}
func (r *polynomialReference) close(m *MultivariatePolynomialCore) error {
	if e := r.closeValue(m, true); e != nil {
		return e
	}
	return r.abort(m)
}

// PolynomialRef is one actual charged cold owning object. Shallow copies are invalid.
type PolynomialRef struct {
	quasiFrame      *polynomialQuasiResultantScope
	quasiInvocation uint64
	self            *PolynomialRef
	owner           *MultivariatePolynomialCore
	root            polynomialReference
	header          uint64
	closed, closing bool
}

func NewPolynomialRef(m *MultivariatePolynomialCore) (*PolynomialRef, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialRef{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	r := &PolynomialRef{owner: m, header: h}
	r.self = r
	return r, nil
}
func (r *PolynomialRef) open() error {
	if r == nil || r.self != r {
		return ErrPolynomialOwner
	}
	if r.closed || r.closing {
		return arena.ErrClosed
	}
	return r.owner.check()
}
func (r *PolynomialRef) Assign(p Polynomial) error {
	if e := r.open(); e != nil {
		return e
	}
	return r.root.assign(r.owner, p)
}
func (r *PolynomialRef) Get() (Polynomial, error) {
	if e := r.open(); e != nil {
		return Polynomial{}, e
	}
	return r.root.value, nil
}
func (r *PolynomialRef) Reset() error { return r.Assign(Polynomial{}) }

// SourceFinalize runs the lexical reference destructor once. It does not
// acknowledge an already stopped DEC or abort an unpublished assignment INC;
// Close is the separate owning continuation for those accepted operations.
// complete reports whether no unpublished assignment ownership remains.
func (r *PolynomialRef) SourceFinalize() (complete bool, err error) {
	if r.quasiResultantPending() {
		return false, ErrPolynomialPending
	}
	if r == nil || r.self != r {
		return false, ErrPolynomialOwner
	}
	if r.closed {
		return true, nil
	}
	r.closing = true
	if err := r.root.closeValue(r.owner, false); err != nil {
		return false, err
	}
	if r.root.assigning || r.root.incoming != (Polynomial{}) {
		return false, nil
	}
	r.owner.integer.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return true, nil
}

func (r *PolynomialRef) Close() error {
	if r.quasiResultantPending() {
		return ErrPolynomialPending
	}
	if r == nil || r.self != r {
		return ErrPolynomialOwner
	}
	if r.closed {
		return nil
	}
	r.closing = true
	if e := r.root.close(r.owner); e != nil {
		return e
	}
	r.owner.integer.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}

type polynomialReferenceVector struct {
	setInvocation                           uint64
	setIndex                                uint32
	setActive, setExecuting, setOldConsumed bool
	setOldRead                              bool
	resetExecuting                          bool
	setBodyCause, setCleanupCause           error
	values                                  []Polynomial
	cleanupCause                            error
	incoming                                Polynomial
	incomingDec                             bool
	drainDec                                bool
	drain                                   uint32
	draining                                bool
	shrinking                               bool
	shrinkTo                                uint32
}

func (v *polynomialReferenceVector) get(m *MultivariatePolynomialCore, i uint32) (Polynomial, error) {
	if uint64(i) >= uint64(len(v.values)) {
		if polynomialSourceDebug {
			return Polynomial{}, ErrPolynomialAssertion
		}
		return Polynomial{}, arena.ErrBounds
	}
	return v.values[i], nil
}
func (v *polynomialReferenceVector) push(m *MultivariatePolynomialCore, p Polynomial) error {
	if v.setActive || v.draining || v.incoming != (Polynomial{}) {
		return ErrPolynomialPending
	}
	if p != (Polynomial{}) {
		if e := m.incRefCore(p); e != nil {
			return e
		}
	}
	v.values = append(v.values, p)
	return nil
}

func (v *polynomialReferenceVector) reset(m *MultivariatePolynomialCore) error {
	return v.resetMode(m, false)
}
func (v *polynomialReferenceVector) resetMode(m *MultivariatePolynomialCore, owning bool) (err error) {
	if v.resetExecuting {
		return ErrPolynomialPending
	}
	if v.setActive {
		return ErrPolynomialPending
	}
	if v.shrinking {
		if !owning {
			return ErrPolynomialPending
		}
		if e := v.finishShrink(m); e != nil {
			return e
		}
	}
	if !owning && v.cleanupCause != nil {
		return v.cleanupCause
	}
	v.resetExecuting = true
	defer func() { v.resetExecuting = false }()
	defer func() {
		if err != nil {
			v.cleanupCause = err
		} else {
			v.cleanupCause = nil
		}
	}()
	v.draining = true
	n := uint32(len(v.values))
	for v.drain < n {
		p, e := v.get(m, v.drain)
		if e != nil {
			return e
		}
		if e = polynomialReferenceDrop(m, &p, &v.drainDec); e != nil {
			return e
		}
		v.drain++
	}
	v.values = v.values[:0]
	v.drain = 0
	v.draining = false
	return nil
}
func (v *polynomialReferenceVector) close(m *MultivariatePolynomialCore) error {
	if e := v.abortSet(m); e != nil {
		return e
	}
	if e := v.resetMode(m, true); e != nil {
		return e
	}
	v.values = nil
	return nil
}

// PolynomialRefVector owns one reference per native slice entry.
type PolynomialRefVector struct {
	self            *PolynomialRefVector
	owner           *MultivariatePolynomialCore
	state           polynomialReferenceVector
	header          uint64
	closed, closing bool
}

func NewPolynomialRefVector(m *MultivariatePolynomialCore) (*PolynomialRefVector, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialRefVector{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	v := &PolynomialRefVector{owner: m, header: h}
	v.self = v
	return v, nil
}
func (v *PolynomialRefVector) open() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing {
		return arena.ErrClosed
	}
	return v.owner.check()
}
func (v *PolynomialRefVector) Push(p Polynomial) error {
	if e := v.open(); e != nil {
		return e
	}
	return v.state.push(v.owner, p)
}
func (v *PolynomialRefVector) Get(i uint32) (Polynomial, error) {
	if e := v.open(); e != nil {
		return Polynomial{}, e
	}
	return v.state.get(v.owner, i)
}
func (v *PolynomialRefVector) Size() (uint32, error) {
	if e := v.open(); e != nil {
		return 0, e
	}
	return uint32(len(v.state.values)), nil
}
func (v *PolynomialRefVector) Reset() error {
	if e := v.open(); e != nil {
		return e
	}
	return v.state.reset(v.owner)
}

// SourceFinalize runs the actual forward lexical reset, without adopting a
// previously stopped DEC, SET or SHRINK. Close resumes owning cleanup.
func (v *PolynomialRefVector) SourceFinalize() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return nil
	}
	if v.state.setExecuting || v.state.resetExecuting {
		return ErrPolynomialPending
	}
	v.closing = true
	if err := v.state.resetMode(v.owner, false); err != nil {
		return err
	}
	v.owner.integer.Budget().Release(v.header)
	v.header = 0
	v.closed = true
	return nil
}

func (v *PolynomialRefVector) Close() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return nil
	}
	if v.state.setExecuting || v.state.resetExecuting {
		return ErrPolynomialPending
	}
	v.closing = true
	if e := v.state.close(v.owner); e != nil {
		return e
	}
	v.owner.integer.Budget().Release(v.header)
	v.header = 0
	v.closed = true
	return nil
}
