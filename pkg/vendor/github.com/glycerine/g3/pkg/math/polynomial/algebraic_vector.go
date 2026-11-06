package polynomial

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Native owning algebraic slice. Get returns a borrowed scalar snapshot; it must
// never be deleted or retained beyond its owning element's lifetime.
type AlgebraicVector struct {
	self            *AlgebraicVector
	core            *AlgebraicRootCore
	values          []Algebraic
	header          uint64
	closed, closing bool
	resetPending    bool
}

func (v *AlgebraicVector) isolationOutputExecuting() bool {
	return v != nil && v.self == v && v.core != nil && v.core.self == v.core && v.core.univariateIsolationRunning && v.core.univariateIsolationOutput == v
}

func (m *AlgebraicRootCore) NewVector() (*AlgebraicVector, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	if m.vectors == math.MaxUint32 {
		return nil, arena.ErrSlotLimit
	}
	h := uint64(reflect.TypeOf(AlgebraicVector{}).Size())
	if e := m.config.Rationals.Budget().Reserve(h); e != nil {
		return nil, e
	}
	v := &AlgebraicVector{core: m, header: h}
	v.self = v
	m.vectors++
	return v, nil
}
func (v *AlgebraicVector) check() error {
	if v == nil || v.self != v {
		return ErrAlgebraicOwner
	}
	if v.closed || v.closing {
		return arena.ErrClosed
	}
	return v.core.check()
}
func (v *AlgebraicVector) Len() uint32 {
	if v == nil || v.self != v || v.closed {
		return 0
	}
	return uint32(len(v.values))
}
func (v *AlgebraicVector) Get(i uint32) (Algebraic, error) {
	if e := v.check(); e != nil {
		return Algebraic{}, e
	}
	if i >= uint32(len(v.values)) {
		return Algebraic{}, arena.ErrBounds
	}
	a := v.values[i]
	_, e := v.core.location(&a)
	return a, e
}

// Borrow returns the actual owning element for retained callers passing an
// anum& to operations that may refine or replace it. Get returns a scalar
// snapshot and cannot publish a replaced cell back into this vector. The
// borrowed pointer is invalidated by vector growth, reset or close.
func (v *AlgebraicVector) Borrow(i uint32) (*Algebraic, error) {
	if e := v.check(); e != nil {
		return nil, e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return nil, arena.ErrBounds
	}
	a := &v.values[i]
	_, e := v.core.location(a)
	return a, e
}
func (v *AlgebraicVector) pushTake(a *Algebraic) error {
	if e := v.check(); e != nil {
		return e
	}
	if uint64(len(v.values)) >= math.MaxUint32 || len(v.values) == int(^uint(0)>>1) {
		return arena.ErrBounds
	}
	old := v.values
	v.values = append(v.values, *a)
	if len(old) != 0 && &old[0] != &v.values[0] {
		clear(old)
	}
	*a = Algebraic{}
	return nil
}
func (v *AlgebraicVector) PushCopy(a *Algebraic) error {
	if v.isolationOutputExecuting() {
		return ErrAlgebraicPending
	}
	if e := v.check(); e != nil {
		return e
	}
	if v.core.pending.slot != 0 {
		return ErrAlgebraicPending
	}
	var next Algebraic
	if e := v.core.Copy(&next, a); e != nil {
		if next.slot != 0 {
			v.core.pending = next
			v.core.locations[next.slot-1].published = false
			v.core.callers--
		}
		return e
	}
	if e := v.pushTake(&next); e != nil {
		v.core.pending = next
		if next.slot != 0 {
			v.core.locations[next.slot-1].published = false
			v.core.callers--
		}
		return e
	}
	return nil
}
func (v *AlgebraicVector) Reset() error {
	if v.isolationOutputExecuting() {
		return ErrAlgebraicPending
	}
	if e := v.check(); e != nil {
		return e
	}
	for i := uint32(0); i < uint32(len(v.values)); i++ {
		if e := v.core.Del(&v.values[i]); e != nil {
			v.resetPending = true
			return e
		}
	}
	v.values = v.values[:0]
	v.resetPending = false
	return nil
}
func (v *AlgebraicVector) Close() error {
	if v == nil || v.self != v {
		return ErrAlgebraicOwner
	}
	if v.isolationOutputExecuting() || v.core != nil && (v.core.isolationSigns.traceRunning || v.core.rootTrace.running || v.core.rootTracePending() || v.core.rootDisplay.active || v.core.coefficientNormalizationPending()) {
		return ErrAlgebraicPending
	}
	return v.closeValues(false)
}

// The retained core owns its root-selection scratch vector. Its owning Close
// uses this continuation, while public Close executes literal source deletion.
func (v *AlgebraicVector) closeValues(owning bool) error {
	if v == nil || v.self != v {
		return ErrAlgebraicOwner
	}
	if v.closed {
		return nil
	}
	v.closing = true
	for i := uint32(0); i < uint32(len(v.values)); i++ {
		var e error
		if owning {
			var l *algebraicLocation
			l, e = v.core.locationCleanup(&v.values[i])
			if e == nil && l != nil {
				e = v.core.destroyCell(&v.values[i], l, true)
			}
		} else if v.core.closing {
			e = v.core.delClosing(&v.values[i])
		} else {
			e = v.core.Del(&v.values[i])
		}
		if e != nil {
			return e
		}
	}
	b := v.core.config.Rationals.Budget()
	b.Release(v.header)
	v.values = nil
	v.header = 0
	v.core.vectors--
	v.closed = true
	return nil
}

// These operations use the actual owning elements: source predicates and
// comparisons may replace a root cell with a basic rational cell.
func (v *AlgebraicVector) Compare(i, j uint32) (int, error) {
	if e := v.check(); e != nil {
		return 0, e
	}
	if i >= uint32(len(v.values)) || j >= uint32(len(v.values)) {
		return 0, arena.ErrBounds
	}
	return v.core.Compare(&v.values[i], &v.values[j])
}
func (v *AlgebraicVector) IsInt(i uint32) (bool, error) {
	if e := v.check(); e != nil {
		return false, e
	}
	if i >= uint32(len(v.values)) {
		return false, arena.ErrBounds
	}
	return v.core.IsInt(&v.values[i])
}
func (v *AlgebraicVector) IsRational(i uint32) (bool, error) {
	if e := v.check(); e != nil {
		return false, e
	}
	if i >= uint32(len(v.values)) {
		return false, arena.ErrBounds
	}
	return v.core.IsRational(&v.values[i])
}

// CleanupRetained acknowledges values left by a stopped Reset or Close. Finish
// the core's retained numerical children first. Healthy vectors are unchanged;
// Close still owns the final vector header release.
func (v *AlgebraicVector) CleanupRetained() error {
	if v == nil || v.self != v {
		return ErrAlgebraicOwner
	}
	if v.closed {
		return nil
	}
	if v.isolationOutputExecuting() {
		return ErrAlgebraicPending
	}
	if !v.closing && !v.resetPending {
		return nil
	}
	if e := v.core.check(); e != nil {
		return e
	}
	for i := range v.values {
		if e := v.core.CleanupValue(&v.values[i]); e != nil {
			return e
		}
	}
	v.values = v.values[:0]
	v.resetPending = false
	return nil
}
