package numeral

// Source: src/util/rational.h; src/util/rational.cpp.
// Source: src/util/rational.h owner-local operator facade.

import (
	"errors"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrRationalFacadeDomain = errors.New("mpnum: rational facade source assertion domain")

type rationalFacadeBinaryOp uint8

const (
	facadeInvalid rationalFacadeBinaryOp = iota
	rationalFacadeAdd
	rationalFacadeSub
	rationalFacadeMul
	rationalFacadeDiv
	rationalFacadeRem
	rationalFacadeIDiv
	rationalFacadeMachineDiv
	rationalFacadeMod
	rationalFacadeGCD
)

// These helpers require the caller to hold s.mu. All input provenance is
// checked before numeric actions; no foreign raw numeral is admitted.
func (s *RationalService) facadeEntry(v *OwnedRational, op rationalFacadeBinaryOp) error {
	if op >= rationalFacadeAdd && op <= rationalFacadeDiv {
		return s.rationalScalarOwner(v)
	}
	return s.check(v)
}
func (s *RationalService) facadeAssignLocked(a, b *OwnedRational, op rationalFacadeBinaryOp) error {
	if e := s.facadeEntry(a, op); e != nil {
		return e
	}
	if e := s.facadeEntry(b, op); e != nil {
		return e
	}
	return s.facadeRawBinaryLocked(&a.Raw, &b.Raw, &a.Raw, op)
}
func (s *RationalService) facadeRawBinaryLocked(a, b, out *Rational, op rationalFacadeBinaryOp) error {
	switch op {
	case rationalFacadeAdd:
		return s.Manager.Add(a, b, out)
	case rationalFacadeSub:
		return s.Manager.Sub(a, b, out)
	case rationalFacadeMul:
		return s.Manager.Mul(a, b, out)
	case rationalFacadeDiv:
		return s.Manager.Div(a, b, out)
	case rationalFacadeRem:
		return s.Manager.Rem(a, b, out)
	case rationalFacadeIDiv:
		return s.Manager.IDiv(a, b, out)
	case rationalFacadeMachineDiv:
		return s.Manager.MachineIDiv(a, b, out)
	case rationalFacadeMod:
		return s.Manager.Mod(a, b, out)
	case rationalFacadeGCD:
		return s.Manager.GCD(a, b, out)
	default:
		return ErrRationalFacadeDomain
	}
}
func (s *RationalService) facadeValueLocked(a, b *OwnedRational, op rationalFacadeBinaryOp) (_lexOut OwnedRational, _lexErr error) {
	return s.facadeValueLockedWithInvocation(a, b, op, nil)
}
func (s *RationalService) facadeValueLockedWithInvocation(a, b *OwnedRational, op rationalFacadeBinaryOp, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.facadeEntry(a, op); e != nil {
		return OwnedRational{}, e
	}
	if e := s.facadeEntry(b, op); e != nil {
		return OwnedRational{}, e
	}
	return s.facadeValueRawOperandsLockedWithInvocation(&a.Raw, &b.Raw, op, c)
}

// The caller holds the service lock and lexical scope. Raw operands may be
// borrowed static constants; only work and the returned value are owned here.
func (s *RationalService) facadeValueRawOperandsLockedWithInvocation(a, b *Rational, op rationalFacadeBinaryOp, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	work := NewRational()
	workComplete := !(op >= rationalFacadeAdd && op <= rationalFacadeDiv)
	defer s.endLexicalLocal(&work, &workComplete)
	defer func() {
		if cause := c.childSource(); cause != nil {
			if s.Lexical.Cause == nil {
				s.Lexical.Cause = cause
			}
			_lexErr = errors.Join(_lexErr, cause)
		}
	}()
	if op >= rationalFacadeAdd && op <= rationalFacadeDiv {
		// return rational(a) op= b: copy before compound, then copy the returned
		// lvalue into the value result. The second copy is intentionally observable.
		if e := s.Manager.copyWithAllocationContext(&work, a, c.allocationContext()); e != nil {
			return OwnedRational{}, e
		}
		workComplete = true
		var body error
		if c != nil && (op == rationalFacadeAdd || op == rationalFacadeSub) {
			body = s.Manager.addSubWithAllocationContext(&work, b, &work, op == rationalFacadeSub, c.allocationContext())
		} else if c != nil && op == rationalFacadeMul {
			body = s.Manager.mulWithAllocationContext(&work, b, &work, c.allocationContext())
		} else if c != nil && op == rationalFacadeDiv {
			body = s.Manager.divWithAllocationContext(&work, b, &work, c.allocationContext())
		} else {
			body = s.facadeRawBinaryLocked(&work, b, &work, op)
		}
		if body != nil {
			return OwnedRational{}, body
		}
		return s.copyOutputWithInvocation(&work, c)
	}
	var body error
	if c != nil && op == rationalFacadeMod {
		body = s.Manager.modWithAllocationContext(a, b, &work, c.allocationContext())
	} else if c != nil && op == rationalFacadeRem {
		body = s.Manager.integerOpRationalWithAllocationContext(a, b, &work, qIntegerRem, c.allocationContext())
	} else if c != nil && op == rationalFacadeMachineDiv {
		body = s.Manager.integerOpRationalWithAllocationContext(a, b, &work, qIntegerMachineDiv, c.allocationContext())
	} else if c != nil && op == rationalFacadeIDiv {
		body = s.Manager.integerOpRationalWithAllocationContext(a, b, &work, qIntegerDiv, c.allocationContext())
	} else {
		body = s.facadeRawBinaryLocked(a, b, &work, op)
	}
	if body != nil {
		return OwnedRational{}, body
	}
	return s.facadeTakeLocked(&work)
}

// Named local return transfers ownership; it does not introduce a second copy.
func (s *RationalService) facadeTakeLocked(work *Rational) (OwnedRational, error) {
	if e := s.lexicalReady(); e != nil {
		return OwnedRational{}, e
	}
	out := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	if e := s.Manager.Move(&out.Raw, work); e != nil {
		return OwnedRational{}, e
	}
	return out, nil
}
func (s *RationalService) facadeValue(a, b *OwnedRational, op rationalFacadeBinaryOp) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	// Same actual held numerical invocation as PowerOfTwo: only a reached
	// Runtime transport erases reentrant mutex acquisition. Ordinary callers
	// continue to serialize on mu.
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadeValueLocked(a, b, op)
}
func (s *RationalService) facadeAssign(a, b *OwnedRational, op rationalFacadeBinaryOp) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if op == rationalFacadeMul {
		atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
		defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	}
	return s.facadeAssignLocked(a, b, op)
}
func (s *RationalService) Add(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeAdd)
}
func (s *RationalService) Sub(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeSub)
}
func (s *RationalService) Mul(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeMul)
}
func (s *RationalService) Div(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeDiv)
}
func (s *RationalService) Rem(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeRem)
}
func (s *RationalService) IDiv(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeIDiv)
}
func (s *RationalService) MachineDiv(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeMachineDiv)
}
func (s *RationalService) Mod(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeMod)
}
func (s *RationalService) GCD(a, b *OwnedRational) (OwnedRational, error) {
	return s.facadeValue(a, b, rationalFacadeGCD)
}
func (s *RationalService) AddAssign(a, b *OwnedRational) error {
	return s.facadeAssign(a, b, rationalFacadeAdd)
}
func (s *RationalService) SubAssign(a, b *OwnedRational) error {
	return s.facadeAssign(a, b, rationalFacadeSub)
}
func (s *RationalService) MulAssign(a, b *OwnedRational) error {
	return s.facadeAssign(a, b, rationalFacadeMul)
}
func (s *RationalService) DivAssign(a, b *OwnedRational) error {
	return s.facadeAssign(a, b, rationalFacadeDiv)
}
func (s *RationalService) RemAssign(a, b *OwnedRational) error {
	return s.facadeAssign(a, b, rationalFacadeRem)
}
func (s *RationalService) IDivInto(a, b, out *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	for _, v := range []*OwnedRational{a, b, out} {
		if e := s.check(v); e != nil {
			return e
		}
	}
	return s.facadeRawBinaryLocked(&a.Raw, &b.Raw, &out.Raw, rationalFacadeIDiv)
}
func (s *RationalService) ModInto(a, b, out *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	for _, v := range []*OwnedRational{a, b, out} {
		if e := s.check(v); e != nil {
			return e
		}
	}
	return s.facadeRawBinaryLocked(&a.Raw, &b.Raw, &out.Raw, rationalFacadeMod)
}

// Scalar overloads construct an actual rational(int), rather than taking the
// raw manager's mixed-integer arithmetic shortcut.
func (s *RationalService) facadeScalar(a *OwnedRational, n int32, op rationalFacadeBinaryOp, left bool) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.check(a); e != nil {
		return OwnedRational{}, e
	}
	v := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	vComplete := false
	defer s.endLexicalLocal(&v.Raw, &vComplete)
	if e := s.Manager.SetInt64(&v.Raw, int64(n)); e != nil {
		return OwnedRational{}, e
	}
	vComplete = true
	if left {
		return s.facadeValueLocked(&v, a, op)
	}
	return s.facadeValueLocked(a, &v, op)
}
func (s *RationalService) AddInt(a *OwnedRational, n int32) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeAdd, false)
}
func (s *RationalService) IntAdd(n int32, a *OwnedRational) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeAdd, true)
}
func (s *RationalService) SubInt(a *OwnedRational, n int32) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeSub, false)
}
func (s *RationalService) IntSub(n int32, a *OwnedRational) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeSub, true)
}
func (s *RationalService) MulInt(a *OwnedRational, n int32) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeMul, false)
}
func (s *RationalService) IntMul(n int32, a *OwnedRational) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeMul, true)
}
func (s *RationalService) DivInt(a *OwnedRational, n int32) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeDiv, false)
}
func (s *RationalService) IntDiv(n int32, a *OwnedRational) (OwnedRational, error) {
	return s.facadeScalar(a, n, rationalFacadeDiv, true)
}
func (s *RationalService) AddAssignInt(a *OwnedRational, n int32) (_lexErr error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.check(a); e != nil {
		return e
	}
	v := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	vComplete := false
	defer s.endLexicalLocal(&v.Raw, &vComplete)
	if e := s.Manager.SetInt64(&v.Raw, int64(n)); e != nil {
		return e
	}
	vComplete = true
	return s.facadeAssignLocked(a, &v, rationalFacadeAdd)
}
func (s *RationalService) AssignInt(a *OwnedRational, n int32) (_lexErr error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.check(a); e != nil {
		return e
	}
	v := NewRational()
	vComplete := false
	defer s.endLexicalLocal(&v, &vComplete)
	if e := s.Manager.SetInt64(&v, int64(n)); e != nil {
		return e
	}
	vComplete = true
	return s.Manager.Copy(&a.Raw, &v)
}
func (s *RationalService) facadePrefixLocked(a *OwnedRational, dec bool) error {
	if e := s.check(a); e != nil {
		return e
	}
	// Source mk_q(1) creates a local inline rational, not the cached constant.
	one := NewRational()
	one.Num = NewInt(1)
	if dec {
		return s.Manager.Sub(&a.Raw, &one, &a.Raw)
	}
	return s.Manager.Add(&a.Raw, &one, &a.Raw)
}
func (s *RationalService) facadePrefix(a *OwnedRational, dec bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.facadePrefixLocked(a, dec)
}
func (s *RationalService) PrefixInc(a *OwnedRational) error { return s.facadePrefix(a, false) }
func (s *RationalService) PrefixDec(a *OwnedRational) error { return s.facadePrefix(a, true) }
func (s *RationalService) facadePostfix(a *OwnedRational, dec bool) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.facadePostfixLockedWithInvocation(a, dec, nil)
}
func (s *RationalService) facadePostfixLockedWithInvocation(a *OwnedRational, dec bool, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	defer func() {
		if cause := c.childSource(); cause != nil {
			if s.Lexical.Cause == nil {
				s.Lexical.Cause = cause
			}
			_lexErr = errors.Join(_lexErr, cause)
		}
	}()
	if e := s.check(a); e != nil {
		return OwnedRational{}, e
	}
	old := NewRational()
	oldComplete := false
	defer s.endLexicalLocal(&old, &oldComplete)
	if e := s.Manager.copyWithAllocationContext(&old, &a.Raw, c.allocationContext()); e != nil {
		return OwnedRational{}, e
	}
	oldComplete = true
	var e error
	if c == nil {
		e = s.facadePrefixLocked(a, dec)
	} else {
		one := NewRational()
		one.Num = NewInt(1)
		e = s.Manager.addSubWithAllocationContext(&a.Raw, &one, &a.Raw, dec, c.allocationContext())
	}
	if e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&old)
}
func (s *RationalService) PostInc(a *OwnedRational) (OwnedRational, error) {
	return s.facadePostfix(a, false)
}
func (s *RationalService) PostDec(a *OwnedRational) (OwnedRational, error) {
	return s.facadePostfix(a, true)
}

type rationalFacadeUnaryOp uint8

const (
	facadeNeg rationalFacadeUnaryOp = iota
	facadeAbs
	facadeFloor
	facadeCeil
	facadeNumerator
	facadeDenominator
	facadePower
)

func (s *RationalService) facadeUnary(a *OwnedRational, op rationalFacadeUnaryOp, p uint32) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if op == facadePower {
		atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
		defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	}
	return s.facadeUnaryLockedWithInvocation(a, op, p, nil)
}
func (s *RationalService) facadeUnaryLockedWithInvocation(a *OwnedRational, op rationalFacadeUnaryOp, p uint32, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	defer func() {
		if cause := c.childSource(); cause != nil {
			if s.Lexical.Cause == nil {
				s.Lexical.Cause = cause
			}
			_lexErr = errors.Join(_lexErr, cause)
		}
	}()
	if op == facadeNumerator || op == facadeDenominator || op == facadeNeg || op == facadeAbs {
		if e := s.rationalScalarOwner(a); e != nil {
			return OwnedRational{}, e
		}
	} else {
		if e := s.check(a); e != nil {
			return OwnedRational{}, e
		}
	}
	work := NewRational()
	workComplete := op != facadeNeg && op != facadeAbs
	defer s.endLexicalLocal(&work, &workComplete)
	var e error
	switch op {
	case facadeNeg, facadeAbs:
		e = s.Manager.copyWithAllocationContext(&work, &a.Raw, c.allocationContext())
		if e == nil {
			workComplete = true
			if op == facadeNeg {
				e = s.Manager.negWithSourceContext(&work, c.allocationContext())
			} else {
				e = s.Manager.Abs(&work)
			}
		}
	case facadeFloor:
		e = s.Manager.Floor(&a.Raw, &work)
	case facadeCeil:
		e = s.Manager.Ceil(&a.Raw, &work)
	case facadeNumerator:
		e = s.Manager.NumeratorRational(&a.Raw, &work)
	case facadeDenominator:
		e = s.Manager.DenominatorRational(&a.Raw, &work)
	case facadePower:
		e = s.Manager.powerWithAllocationContext(&a.Raw, p, &work, c.allocationContext())
		// A reached child source-end failure stops the older result's source
		// destructor before the ordinary facade epilogue can attempt it.
		if cause := c.childSource(); cause != nil {
			if s.Lexical.Cause == nil {
				s.Lexical.Cause = cause
			}
			e = errors.Join(e, cause)
		}
	default:
		e = ErrRationalFacadeDomain
	}
	if e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&work)
}
func (s *RationalService) Neg(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeNeg, 0)
}
func (s *RationalService) Abs(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeAbs, 0)
}
func (s *RationalService) Floor(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeFloor, 0)
}
func (s *RationalService) Ceil(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeCeil, 0)
}
func (s *RationalService) Numerator(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeNumerator, 0)
}
func (s *RationalService) Denominator(a *OwnedRational) (OwnedRational, error) {
	return s.facadeUnary(a, facadeDenominator, 0)
}
func (s *RationalService) Power(a *OwnedRational, p uint32) (OwnedRational, error) {
	return s.facadeUnary(a, facadePower, p)
}

// Expt preserves source signed-to-unsigned forwarding, never reciprocal power.
func (s *RationalService) Expt(a *OwnedRational, p int32) (OwnedRational, error) {
	return s.Power(a, uint32(p))
}
func (s *RationalService) NegAssign(a *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	return s.Manager.Neg(&a.Raw)
}
func (s *RationalService) Reset(a *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	return s.Manager.Reset(&a.Raw)
}

// Comparison routes retain operand reversal, structural equality, and source
// sign shortcuts even for noncanonical raw denominators.
func (s *RationalService) facadeCompareLocked(a, b *OwnedRational, op uint8) (bool, error) {
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	if op == 0 {
		v, e := s.Manager.Equal(&a.Raw, &b.Raw)
		if e != nil {
			return false, e
		}
		return !v, nil
	}
	x, y := a, b
	if op == 2 || op == 3 {
		x, y = b, a
	}
	c, e := s.Manager.Compare(&x.Raw, &y.Raw)
	if e != nil {
		return false, e
	}
	if op == 3 || op == 4 {
		return c >= 0, nil
	}
	return c < 0, nil
}
func (s *RationalService) facadeCompare(a, b *OwnedRational, op uint8) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.facadeCompareLocked(a, b, op)
}
func (s *RationalService) NotEqual(a, b *OwnedRational) (bool, error) {
	return s.facadeCompare(a, b, 0)
}
func (s *RationalService) Less(a, b *OwnedRational) (bool, error)    { return s.facadeCompare(a, b, 1) }
func (s *RationalService) Greater(a, b *OwnedRational) (bool, error) { return s.facadeCompare(a, b, 2) }
func (s *RationalService) LessEqual(a, b *OwnedRational) (bool, error) {
	return s.facadeCompare(a, b, 3)
}
func (s *RationalService) GreaterEqual(a, b *OwnedRational) (bool, error) {
	return s.facadeCompare(a, b, 4)
}
func (s *RationalService) facadeCompareInt(a *OwnedRational, n int32, op uint8) (_lexValue bool, _lexErr error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return false, e
	}
	defer func() {
		if _lexErr != nil {
			_lexValue = false
		}
	}()
	defer s.endLexicalError(&_lexErr)
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	v := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	vComplete := false
	defer s.endLexicalLocal(&v.Raw, &vComplete)
	if e := s.Manager.SetInt64(&v.Raw, int64(n)); e != nil {
		return false, e
	}
	vComplete = true
	if op == 5 {
		return s.Manager.Equal(&a.Raw, &v.Raw)
	}
	return s.facadeCompareLocked(a, &v, op)
}
func (s *RationalService) NotEqualInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 0)
}
func (s *RationalService) LessInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 1)
}
func (s *RationalService) GreaterInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 2)
}
func (s *RationalService) EqualInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 5)
}
func (s *RationalService) IsPos(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsPos(&a.Raw)
}
func (s *RationalService) IsNeg(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsNeg(&a.Raw)
}
func (s *RationalService) IsZero(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsZero(&a.Raw)
}
func (s *RationalService) IsOne(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsOne(&a.Raw)
}
func (s *RationalService) IsMinusOne(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsMinusOne(&a.Raw)
}
func (s *RationalService) IsNonNeg(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsNonNeg(&a.Raw)
}
func (s *RationalService) IsNonPos(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsNonPos(&a.Raw)
}

// Projection provenance guards do not inspect unread numeral components.
func (s *RationalService) scalarProjectionOwner(ownerID uint64) error {
	if ownerID == 0 || ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	return s.Manager.checkOwnerReady()
}
func (s *RationalService) rationalScalarOwner(a *OwnedRational) error {
	if a == nil {
		return ErrRationalServiceOwner
	}
	return s.scalarProjectionOwner(a.ownerID)
}
func (s *RationalService) IsInt(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsInt(&a.Raw)
}
func (s *RationalService) IsUint64(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsUint64(&a.Raw)
}
func (s *RationalService) IsInt64(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsInt64(&a.Raw)
}
func (s *RationalService) IsUnsigned(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsUnsigned(&a.Raw)
}
func (s *RationalService) IsInt32(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return s.Manager.IsInt32(&a.Raw)
}
func (s *RationalService) IsEven(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	return s.Manager.IsEven(&a.Raw)
}
func (s *RationalService) Uint64(a *OwnedRational) (uint64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Uint64(&a.Raw)
}
func (s *RationalService) Int64(a *OwnedRational) (int64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Int64(&a.Raw)
}
func (s *RationalService) Unsigned(a *OwnedRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Unsigned(&a.Raw)
}
func (s *RationalService) Int32(a *OwnedRational) (int32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Int32(&a.Raw)
}
func (s *RationalService) Hash(a *OwnedRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Hash(&a.Raw)
}
func (s *RationalService) Bitsize(a *OwnedRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.Bitsize(&a.Raw)
}
func (s *RationalService) StorageSize(a *OwnedRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.StorageSize(&a.Raw)
}
func (s *RationalService) Float64(a *OwnedRational) (float64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, e
	}
	return s.Manager.Float64(&a.Raw)
}
func (s *RationalService) IsSmall(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	return a.Raw.IsSmall(), nil
}
func (s *RationalService) IsBig(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	return !a.Raw.IsSmall(), nil
}
func (s *RationalService) IsPowerOfTwo(a *OwnedRational) (bool, uint32, error) {
	if e := s.lock(); e != nil {
		return false, 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, 0, e
	}
	return s.Manager.IsPowerOfTwo(&a.Raw)
}
func (s *RationalService) facadeAddMul(a, c, k *OwnedRational, subtract bool) (_lexErr error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	for _, v := range []*OwnedRational{a, c, k} {
		if e := s.check(v); e != nil {
			return e
		}
	}
	one, e := s.Manager.IsOne(&c.Raw)
	if e != nil {
		return e
	}
	minus, e := s.Manager.IsMinusOne(&c.Raw)
	if e != nil {
		return e
	}
	op := rationalFacadeAdd
	if subtract {
		op = rationalFacadeSub
	}
	if one {
		return s.facadeAssignLocked(a, k, op)
	}
	if minus {
		if subtract {
			op = rationalFacadeAdd
		} else {
			op = rationalFacadeSub
		}
		return s.facadeAssignLocked(a, k, op)
	}
	temp := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	tempComplete := false
	defer s.endLexicalLocal(&temp.Raw, &tempComplete)
	if e := s.Manager.Copy(&temp.Raw, &k.Raw); e != nil {
		return e
	}
	tempComplete = true
	if e := s.facadeAssignLocked(&temp, c, rationalFacadeMul); e != nil {
		return e
	}
	return s.facadeAssignLocked(a, &temp, op)
}
func (s *RationalService) AddMulAssign(a, c, k *OwnedRational) error {
	return s.facadeAddMul(a, c, k, false)
}
func (s *RationalService) SubMulAssign(a, c, k *OwnedRational) error {
	return s.facadeAddMul(a, c, k, true)
}
func (s *RationalService) ModHat(a, b *OwnedRational) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.check(a); e != nil {
		return OwnedRational{}, e
	}
	if e := s.check(b); e != nil {
		return OwnedRational{}, e
	}
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return OwnedRational{}, e
		}
		if enabled {
			pos, e := s.Manager.IsPos(&b.Raw)
			if e != nil {
				return OwnedRational{}, e
			}
			if !pos {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 209, "b.is_pos()"); e != nil {
					return OwnedRational{}, e
				}
			}
		}
	}
	r, e := s.facadeValueLocked(a, b, rationalFacadeMod)
	if e != nil {
		return OwnedRational{}, e
	}
	defer s.destroyLexicalRational(&r.Raw)
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return OwnedRational{}, e
		}
		if enabled {
			nonneg, e := s.Manager.IsNonNeg(&r.Raw)
			if e != nil {
				return OwnedRational{}, e
			}
			if !nonneg {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 211, "r.is_nonneg()"); e != nil {
					return OwnedRational{}, e
				}
			}
		}
	}
	r2 := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	r2Complete := false
	defer s.endLexicalLocal(&r2.Raw, &r2Complete)
	if e := s.Manager.Copy(&r2.Raw, &r.Raw); e != nil {
		return OwnedRational{}, e
	}
	r2Complete = true
	two := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	twoComplete := false
	defer s.endLexicalLocal(&two.Raw, &twoComplete)
	if e := s.Manager.SetInt64(&two.Raw, 2); e != nil {
		return OwnedRational{}, e
	}
	twoComplete = true
	if e := s.facadeAssignLocked(&r2, &two, rationalFacadeMul); e != nil {
		return OwnedRational{}, e
	}
	// Source rational(2) is destroyed at this full-expression boundary, before
	// the comparison and optional subtraction. Keep a valid empty value for
	// deferred cleanup on every later error path.
	if e := s.destroyLexicalRational(&two.Raw); e != nil {
		return OwnedRational{}, e
	}
	two.Raw = NewRational()
	lt, e := s.facadeCompareLocked(b, &r2, 1)
	if e != nil {
		return OwnedRational{}, e
	}
	if lt {
		if e := s.facadeAssignLocked(&r, b, rationalFacadeSub); e != nil {
			return OwnedRational{}, e
		}
	}
	return s.facadeTakeLocked(&r.Raw)
}
func (s *RationalService) NumBits(a *OwnedRational) (_lexValue uint32, _lexErr error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return 0, e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.check(a); e != nil {
		return 0, e
	}
	two := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	twoComplete := false
	defer s.endLexicalLocal(&two.Raw, &twoComplete)
	if e := s.Manager.SetInt64(&two.Raw, 2); e != nil {
		return 0, e
	}
	twoComplete = true
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			integer, e := s.Manager.IsInt(&a.Raw)
			if e != nil {
				return 0, e
			}
			if !integer {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 402, "is_int()"); e != nil {
					return 0, e
				}
			}
		}
		enabled, e = diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			neg, e := s.Manager.IsNeg(&a.Raw)
			if e != nil {
				return 0, e
			}
			if neg {
				if e := diagnostic.DebugAssertionFailure(d, "src/util/rational.h", 403, "!is_neg()"); e != nil {
					return 0, e
				}
			}
		}
	}
	n := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	nComplete := false
	defer s.endLexicalLocal(&n.Raw, &nComplete)
	if e := s.Manager.Copy(&n.Raw, &a.Raw); e != nil {
		return 0, e
	}
	nComplete = true
	count := uint32(1)
	for {
		quotient, e := s.facadeValueLocked(&n, &two, rationalFacadeIDiv)
		if e != nil {
			return 0, e
		}
		e = s.Manager.Copy(&n.Raw, &quotient.Raw)
		if cleanup := s.destroyLexicalRational(&quotient.Raw); e == nil {
			e = cleanup
		}
		if e != nil {
			return 0, e
		}
		pos, e := s.Manager.IsPos(&n.Raw)
		if e != nil {
			return 0, e
		}
		if !pos {
			return count, nil
		}
		count++
	}
}

func (s *RationalService) LessEqualInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 3)
}
func (s *RationalService) GreaterEqualInt(a *OwnedRational, n int32) (bool, error) {
	return s.facadeCompareInt(a, n, 4)
}
