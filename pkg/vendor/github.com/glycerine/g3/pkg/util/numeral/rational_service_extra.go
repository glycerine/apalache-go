package numeral

// Source: remaining src/util/rational.h wrappers; concrete owner-local facade.

import (
	"errors"
	"sync/atomic"
)

// RationalDoubleError classifies the source UNREACHABLE double constructor and
// assignment. It does not implement a numerical floating-point conversion.
type RationalDoubleError struct{}

func (*RationalDoubleError) Error() string {
	return "mpnum: source rational double constructor/assignment is unreachable"
}

var ErrRationalDoubleUnsupported = &RationalDoubleError{}

func (s *RationalService) Root(a *OwnedRational, n uint32, out *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	if e := s.check(out); e != nil {
		return false, e
	}
	return s.Manager.Root(&a.Raw, n, &out.Raw)
}
func (s *RationalService) IsPerfectSquare(a, out *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	if e := s.check(out); e != nil {
		return false, e
	}
	return s.Manager.IsPerfectSquare(&a.Raw, &out.Raw)
}
func (s *RationalService) IsIntPerfectSquare(a, out *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return false, e
	}
	if e := s.check(out); e != nil {
		return false, e
	}
	return s.Manager.IsIntPerfectSquare(&a.Raw, &out.Raw)
}

// GCDCoefficients is rational.h's manager-backed gcd(a,b,x,y) value overload,
// distinct from ExtendedGCD, the separate util/ext_gcd.h wrapper algorithm.
// Source output denominators/coefficients publish in manager order, including
// any successful prefixes on error; no extra staging silently erases them.
func (s *RationalService) GCDCoefficients(a, b, x, y *OwnedRational) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.gcdCoefficientsLockedWithInvocation(a, b, x, y, nil)
}
func (s *RationalService) gcdCoefficientsLockedWithInvocation(a, b, x, y *OwnedRational, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	for _, v := range []*OwnedRational{a, b, x, y} {
		if e := s.check(v); e != nil {
			return OwnedRational{}, e
		}
	}
	result := NewRational()
	defer s.destroyLexicalRational(&result)
	defer func() {
		if cause := c.childSource(); cause != nil {
			if s.Lexical.Cause == nil {
				s.Lexical.Cause = cause
			}
			_lexErr = errors.Join(_lexErr, cause)
		}
	}()
	if e := s.Manager.extendedGCDWithAllocationContext(&a.Raw, &b.Raw, &x.Raw, &y.Raw, &result, c.allocationContext()); e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&result)
}
func (s *RationalService) extraBinary(a, b *OwnedRational, op uint8) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if op != 0 {
		atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
		defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	}
	return s.extraBinaryLockedWithInvocation(a, b, op, nil)
}
func (s *RationalService) extraBinaryLockedWithInvocation(a, b *OwnedRational, op uint8, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
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
	result := NewRational()
	defer s.destroyLexicalRational(&result)
	var e error
	if op == 0 {
		e = s.Manager.LCM(&a.Raw, &b.Raw, &result)
	} else {
		e = s.Manager.rationalBitwiseWithAllocationContext(&a.Raw, &b.Raw, &result, op-1, c.allocationContext())
	}
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	if e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&result)
}
func (s *RationalService) LCM(a, b *OwnedRational) (OwnedRational, error) {
	return s.extraBinary(a, b, 0)
}
func (s *RationalService) BitwiseOr(a, b *OwnedRational) (OwnedRational, error) {
	return s.extraBinary(a, b, 1)
}
func (s *RationalService) BitwiseAnd(a, b *OwnedRational) (OwnedRational, error) {
	return s.extraBinary(a, b, 2)
}
func (s *RationalService) BitwiseXor(a, b *OwnedRational) (OwnedRational, error) {
	return s.extraBinary(a, b, 3)
}
func (s *RationalService) BitwiseNot(width uint32, a *OwnedRational) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.bitwiseNotLockedWithInvocation(width, a, nil)
}
func (s *RationalService) bitwiseNotLockedWithInvocation(width uint32, a *OwnedRational, c *RationalServiceInvocation) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.check(a); e != nil {
		return OwnedRational{}, e
	}
	result := NewRational()
	defer s.destroyLexicalRational(&result)
	e := s.Manager.bitwiseNotWithAllocationContext(width, &a.Raw, &result, c.allocationContext())
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	if e != nil {
		return OwnedRational{}, e
	}
	return s.facadeTakeLocked(&result)
}

// Go cannot enforce a const reference lifetime. Self/constant projections return
// independent owning copies, leaving the source token and service constants private.
func (s *RationalService) ToRational(a *OwnedRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&a.Raw)
}
func (s *RationalService) GetRational(a *OwnedRational) (OwnedRational, error) {
	return s.ToRational(a)
}
func (s *RationalService) GetInfinitesimal(a *OwnedRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&s.ZeroField)
}
func (s *RationalService) IsRational(a *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return false, e
	}
	return true, nil
}
func (s *RationalService) NewDouble(v float64) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return OwnedRational{}, ErrRationalDoubleUnsupported
}
func (s *RationalService) AssignDouble(a *OwnedRational, v float64) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return e
	}
	return ErrRationalDoubleUnsupported
}

// The checked ScopedInt owner is externally confined for the entire call.
// Its implicit low-level manager ownership contract still applies; the service
// never accepts a naked foreign Int or returns an escaped manager/raw borrow.
func (s *RationalService) SetFromScopedInt(dst *OwnedRational, source *ScopedInt) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(dst); e != nil {
		return e
	}
	if _, e := source.Get(); e != nil {
		return e
	}
	if e := s.Manager.Z.CopyFrom(&dst.Raw.Num, source.manager, &source.num); e != nil {
		return e
	}
	return s.Manager.resetDenominator(&dst.Raw)
}
func (s *RationalService) NewFromScopedInt(source *ScopedInt) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if _, e := source.Get(); e != nil {
		return OwnedRational{}, e
	}
	result := NewRational()
	resultComplete := false
	defer s.endLexicalLocal(&result, &resultComplete)
	if e := s.Manager.Z.CopyFrom(&result.Num, source.manager, &source.num); e != nil {
		return OwnedRational{}, e
	}
	if e := s.Manager.resetDenominator(&result); e != nil {
		return OwnedRational{}, e
	}
	resultComplete = true
	return s.facadeTakeLocked(&result)
}
func (s *RationalService) componentToScopedInt(a *OwnedRational, dst *ScopedInt, denominator bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if _, e := dst.Get(); e != nil {
		return e
	}
	src := &a.Raw.Num
	if denominator {
		src = &a.Raw.Den
	}
	return dst.manager.CopyFrom(&dst.num, s.Manager.Z, src)
}
func (s *RationalService) NumeratorToScopedInt(a *OwnedRational, dst *ScopedInt) error {
	return s.componentToScopedInt(a, dst, false)
}
func (s *RationalService) DenominatorToScopedInt(a *OwnedRational, dst *ScopedInt) error {
	return s.componentToScopedInt(a, dst, true)
}

// Formatting stays in the existing owned workspaces. Escaped final Go strings
// belong to the caller; caller-buffer forms publish only after complete staging.
func (s *RationalService) Display(a *OwnedRational) (string, error) { return s.String(a) }
func (s *RationalService) FormatInto(a *OwnedRational, dst []byte) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.FormatInto(&a.Raw, dst)
}
func (s *RationalService) DisplayInto(a *OwnedRational, dst []byte) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.FormatPrettyInto(&a.Raw, dst)
}
func (s *RationalService) DecimalString(a *OwnedRational, precision uint32, truncate bool) (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return "", e
	}
	return s.Manager.DecimalString(&a.Raw, precision, truncate)
}
func (s *RationalService) FormatDecimalInto(a *OwnedRational, precision uint32, truncate bool, dst []byte) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.FormatDecimalInto(&a.Raw, precision, truncate, dst)
}
func (s *RationalService) SMT2String(a *OwnedRational, decimal bool) (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return "", e
	}
	return s.Manager.SMT2String(&a.Raw, decimal)
}
func (s *RationalService) FormatSMT2Into(a *OwnedRational, decimal bool, dst []byte) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return 0, e
	}
	return s.Manager.FormatSMT2Into(&a.Raw, decimal, dst)
}

// These are the concrete checked-owner adaptation of rational(mpq const&).
// The source scoped manager is externally confined, exactly as for ScopedInt.
// CopyFrom preserves noncanonical representation; no parser or normalization
// substitutes for the source record-copy constructor.
func (s *RationalService) NewFromScopedRational(source *ScopedRational) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if _, e := source.Get(); e != nil {
		return OwnedRational{}, e
	}
	result := NewRational()
	resultComplete := false
	defer s.endLexicalLocal(&result, &resultComplete)
	if e := s.Manager.CopyFrom(&result, source.manager, &source.num); e != nil {
		return OwnedRational{}, e
	}
	resultComplete = true
	return s.facadeTakeLocked(&result)
}
func (s *RationalService) SetFromScopedRational(dst *OwnedRational, source *ScopedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(dst); e != nil {
		return e
	}
	if _, e := source.Get(); e != nil {
		return e
	}
	return s.Manager.CopyFrom(&dst.Raw, source.manager, &source.num)
}

// ToScopedRational replaces the source to_mpq const borrow with an explicit
// independent owning copy into a checked concrete destination. Its manager
// must remain externally confined for this call; source fields never escape.
func (s *RationalService) ToScopedRational(a *OwnedRational, dst *ScopedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.check(a); e != nil {
		return e
	}
	if _, e := dst.Get(); e != nil {
		return e
	}
	return dst.manager.CopyFrom(&dst.num, s.Manager, &a.Raw)
}
