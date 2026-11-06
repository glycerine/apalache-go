package numeral

// Source: src/util/inf_int_rational.h.
// Concrete translation of src/util/inf_int_rational.h/.cpp.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrInfIntOverflow = errors.New("mpnum: infinitesimal integer signed overflow")
var ErrInfIntScalar = z3exception.NewDefaultException("multiplication with large rational is not possible")
var ErrInfIntNotRational = errors.New("mpnum: infinitesimal coefficient must be zero")

// OwnedInfIntRational is the source rational/int32 pair plus scalar provenance.
// Its zero value is invalid. Never shallow-copy ownership; use InfIntCopy.
// All operations borrow the service; closing it invalidates its values.
type OwnedInfIntRational struct {
	First   Rational
	second  int32
	ownerID uint64
}

func (s *RationalService) initInfIntRationalConstants() error {
	values := [3]*OwnedInfIntRational{&s.InfIntZeroField, &s.InfIntOneField, &s.InfIntMinusOneField}
	bases := [3]*Rational{&s.ZeroField, &s.OneField, &s.MinusOneField}
	for i, v := range values {
		*v = OwnedInfIntRational{First: NewRational(), ownerID: s.ownerID}
		if e := s.Manager.Copy(&v.First, bases[i]); e != nil {
			return e
		}
	}
	return nil
}
func (s *RationalService) destroyInfIntRationalConstants() {
	for _, v := range [3]*OwnedInfIntRational{&s.InfIntZeroField, &s.InfIntOneField, &s.InfIntMinusOneField} {
		if v.ownerID == s.ownerID && v.ownerID != 0 {
			s.Manager.Del(&v.First)
			*v = OwnedInfIntRational{}
		}
	}
}
func (s *RationalService) checkInfInt(v *OwnedInfIntRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	return s.Manager.check(&v.First)
}
func (s *RationalService) newInfInt() OwnedInfIntRational {
	return OwnedInfIntRational{First: NewRational(), ownerID: s.ownerID}
}
func (s *RationalService) copyInfInt(v *OwnedInfIntRational) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	out := s.newInfInt()
	if e := s.Manager.Copy(&out.First, &v.First); e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	out.second = v.second
	return out, nil
}

func (s *RationalService) lockInfInt(v *OwnedInfIntRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	if e := s.checkInfInt(v); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
func (s *RationalService) lockInfInt2(a, b *OwnedInfIntRational) error {
	if e := s.lockInfInt(a); e != nil {
		return e
	}
	if e := s.checkInfInt(b); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
func (s *RationalService) lockInfIntRat(a *OwnedInfIntRational, b *OwnedRational) error {
	if e := s.lockInfInt(a); e != nil {
		return e
	}
	if e := s.check(b); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
func (s *RationalService) InfIntNewValue() (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.newInfInt(), nil
}
func (s *RationalService) InfIntFromInt(n int32) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	out := s.newInfInt()
	if e := s.Manager.SetInt64(&out.First, int64(n)); e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfIntFromFraction(n, d int32) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	out := s.newInfInt()
	e := s.Manager.SetFractionSigned(&out.First, n, d)
	if e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfIntFromRational(r *OwnedRational, epsilon int32) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfIntRational{}, e
	}
	out := s.newInfInt()
	if e := s.Manager.Copy(&out.First, &r.Raw); e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	out.second = epsilon
	return out, nil
}
func (s *RationalService) InfIntFromSignedInf(r *OwnedRational, positive bool) (OwnedInfIntRational, error) {
	epsilon := int32(-1)
	if positive {
		epsilon = 1
	}
	return s.InfIntFromRational(r, epsilon)
}
func (s *RationalService) InfIntZero() (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyInfInt(&s.InfIntZeroField)
}
func (s *RationalService) InfIntOne() (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyInfInt(&s.InfIntOneField)
}
func (s *RationalService) InfIntMinusOne() (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.copyInfInt(&s.InfIntMinusOneField)
}
func (s *RationalService) InfIntDel(v *OwnedInfIntRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := callerDeleteRational(s.Manager.Z, &v.First); e != nil {
		return e
	}
	*v = OwnedInfIntRational{}
	return nil
}
func (s *RationalService) InfIntCopy(dst, src *OwnedInfIntRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(dst); e != nil {
		return e
	}
	if e := s.infIntScalarOwner(src); e != nil {
		return e
	}
	if e := s.Manager.Copy(&dst.First, &src.First); e != nil {
		return e
	}
	dst.second = src.second
	return nil
}
func (s *RationalService) InfIntSetParts(dst *OwnedInfIntRational, src *OwnedRational, epsilon int32) error {
	if e := s.lock(); e != nil {
		return e
	}
	if e := s.infIntScalarOwner(dst); e != nil {
		s.mu.Unlock()
		return e
	}
	if e := s.rationalScalarOwner(src); e != nil {
		s.mu.Unlock()
		return e
	}
	defer s.mu.Unlock()
	if e := s.Manager.Copy(&dst.First, &src.Raw); e != nil {
		return e
	}
	dst.second = epsilon
	return nil
}
func (s *RationalService) InfIntSetRational(dst *OwnedInfIntRational, src *OwnedRational) error {
	return s.InfIntSetParts(dst, src, 0)
}
func (s *RationalService) InfIntSetInt(dst *OwnedInfIntRational, n int32) error {
	if e := s.lock(); e != nil {
		return e
	}
	if e := s.infIntScalarOwner(dst); e != nil {
		s.mu.Unlock()
		return e
	}
	defer s.mu.Unlock()
	if e := s.Manager.SetInt64(&dst.First, int64(n)); e != nil {
		return e
	}
	dst.second = 0
	return nil
}
func (s *RationalService) InfIntReset(v *OwnedInfIntRational) error {
	if e := s.lockInfIntSource(v); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.Manager.Reset(&v.First); e != nil {
		return e
	}
	v.second = 0
	return nil
}
func (s *RationalService) InfIntSwap(a, b *OwnedInfIntRational) error {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.Manager.Swap(&a.First, &b.First); e != nil {
		return e
	}
	a.second, b.second = b.second, a.second
	return nil
}
func infIntPublishSecond(v *OwnedInfIntRational, n int64) error {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return ErrInfIntOverflow
	}
	v.second = int32(n)
	return nil
}
func (s *RationalService) addSubInfInt(a, b *OwnedInfIntRational, sub bool) error {
	if sub {
		if e := s.Manager.Sub(&a.First, &b.First, &a.First); e != nil {
			return e
		}
		return infIntPublishSecond(a, int64(a.second)-int64(b.second))
	}
	if e := s.Manager.Add(&a.First, &b.First, &a.First); e != nil {
		return e
	}
	return infIntPublishSecond(a, int64(a.second)+int64(b.second))
}
func (s *RationalService) InfIntAddAssign(a, b *OwnedInfIntRational) error {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.addSubInfInt(a, b, false)
}
func (s *RationalService) InfIntSubAssign(a, b *OwnedInfIntRational) error {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.addSubInfInt(a, b, true)
}
func (s *RationalService) mulInfInt(a *OwnedInfIntRational, r *Rational) error {
	valid, e := s.Manager.IsInt32(r)
	if e != nil {
		return e
	}
	if !valid {
		return ErrInfIntScalar
	}
	if e = s.Manager.Mul(&a.First, r, &a.First); e != nil {
		return e
	}
	// Source gets coefficient AFTER rational multiplication. Keep this ordering,
	// including scalar alias with first in private/internal use.
	coefficient, e := s.Manager.Int32(r)
	if e != nil {
		return e
	}
	return infIntPublishSecond(a, int64(a.second)*int64(coefficient))
}
func (s *RationalService) InfIntMulAssign(a *OwnedInfIntRational, r *OwnedRational) error {
	if e := s.lockInfIntSourceRat(a, r); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.mulInfInt(a, &r.Raw)
}
func (s *RationalService) InfIntAddRationalAssign(a *OwnedInfIntRational, r *OwnedRational) error {
	if e := s.lockInfIntSourceRat(a, r); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.Manager.Add(&a.First, &r.Raw, &a.First)
}
func (s *RationalService) InfIntSubRationalAssign(a *OwnedInfIntRational, r *OwnedRational) error {
	if e := s.lockInfIntSourceRat(a, r); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.Manager.Sub(&a.First, &r.Raw, &a.First)
}

// Source ++/-- resolves the rational facade Add/Sub with mk_q(1), not the
// manager's distinct mixed-int Inc/Dec route (which has different fault actions).
func (s *RationalService) stepInfInt(v *Rational, dec bool) error {
	one := NewRational()
	one.Num = NewInt(1)
	if dec {
		return s.Manager.Sub(v, &one, v)
	}
	return s.Manager.Add(v, &one, v)
}
func (s *RationalService) InfIntInc(v *OwnedInfIntRational) error {
	if e := s.lockInfInt(v); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.stepInfInt(&v.First, false)
}
func (s *RationalService) InfIntDec(v *OwnedInfIntRational) error {
	if e := s.lockInfInt(v); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.stepInfInt(&v.First, true)
}
func (s *RationalService) postInfInt(v *OwnedInfIntRational, dec bool) (OwnedInfIntRational, error) {
	old, e := s.copyInfInt(v)
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	e = s.stepInfInt(&v.First, dec)
	if e != nil {
		s.destroyLexicalRational(&old.First)
		return OwnedInfIntRational{}, e
	}
	return old, nil
}
func (s *RationalService) InfIntPostInc(v *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lockInfInt(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.postInfInt(v, false)
}
func (s *RationalService) InfIntPostDec(v *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lockInfInt(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.postInfInt(v, true)
}
func (s *RationalService) negInfInt(v *OwnedInfIntRational) error {
	if e := s.Manager.Neg(&v.First); e != nil {
		return e
	}
	return infIntPublishSecond(v, -int64(v.second))
}
func (s *RationalService) InfIntNeg(v *OwnedInfIntRational) error {
	if e := s.lockInfIntSource(v); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return s.negInfInt(v)
}
func (s *RationalService) resultInfInt(a, b *OwnedInfIntRational, sub bool) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	// Source free binary returns a compound-operator lvalue: copy operand,
	// mutate temporary, copy return, destroy temporary (no elision of that copy).
	tmp, e := s.copyInfInt(a)
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.destroyLexicalRational(&tmp.First)
	if e = s.addSubInfInt(&tmp, b, sub); e != nil {
		return OwnedInfIntRational{}, e
	}
	return s.copyInfInt(&tmp)
}
func (s *RationalService) InfIntAdd(a, b *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.resultInfInt(a, b, false)
}
func (s *RationalService) InfIntSub(a, b *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	return s.resultInfInt(a, b, true)
}
func (s *RationalService) InfIntMul(r *OwnedRational, a *OwnedInfIntRational) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lockInfIntSourceRat(a, r); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	tmp, e := s.copyInfInt(a)
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.destroyLexicalRational(&tmp.First)
	if e = s.mulInfInt(&tmp, &r.Raw); e != nil {
		return OwnedInfIntRational{}, e
	}
	return s.copyInfInt(&tmp)
}
func (s *RationalService) InfIntNegCopy(v *OwnedInfIntRational) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lockInfIntSource(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	out, e := s.copyInfInt(v)
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	if e = s.negInfInt(&out); e != nil {
		s.destroyLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfIntAbs(v *OwnedInfIntRational) (_lexOut OwnedInfIntRational, _lexErr error) {
	if e := s.lockInfIntSource(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	out, e := s.copyInfInt(v)
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	if out.First.Num.Val < 0 || (out.First.Num.Val == 0 && out.second < 0) {
		if e = s.negInfInt(&out); e != nil {
			s.destroyLexicalRational(&out.First)
			return OwnedInfIntRational{}, e
		}
	}
	return out, nil
}
func (s *RationalService) InfIntHash(v *OwnedInfIntRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return 0, e
	}
	h, e := s.Manager.Hash(&v.First)
	if e != nil {
		return 0, e
	}
	return h ^ (uint32(v.second) + 1), nil
}
func (s *RationalService) InfIntString(v *OwnedInfIntRational) (string, error) {
	if e := s.lockInfIntSource(v); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	var n uint32
	var e error
	if v.second != 0 {
		n, e = s.Manager.Z.Natural.appendDisplay(0, "(")
		if e != nil {
			return "", e
		}
	}
	n, e = s.infEpsInfPrepareRational(&v.First, n)
	if e != nil {
		return "", e
	}
	if v.second != 0 {
		// Source formats the first component before the signed coefficient.
		if v.second == math.MinInt32 {
			return "", ErrInfIntOverflow
		}
		coefficient := v.second
		sign := " +e*"
		if coefficient < 0 {
			sign = " -e*"
			coefficient = -coefficient
		}
		n, e = s.Manager.Z.Natural.appendDisplay(n, sign)
		if e != nil {
			return "", e
		}
		value := NewInt(coefficient)
		n, e = s.Manager.Z.prepareInteger(&value, n)
		if e != nil {
			return "", e
		}
		n, e = s.Manager.Z.Natural.appendDisplay(n, ")")
		if e != nil {
			return "", e
		}
	}
	return s.Manager.Z.Natural.finalString(n)
}
func (s *RationalService) InfIntFirst(v *OwnedInfIntRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.First)
}
func (s *RationalService) InfIntRational(v *OwnedInfIntRational) (OwnedRational, error) {
	return s.InfIntFirst(v)
}
func (s *RationalService) InfIntInfinitesimal(v *OwnedInfIntRational) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.infIntScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	out := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	if e := s.Manager.SetInt64(&out.Raw, int64(v.second)); e != nil {
		s.abortLexicalRational(&out.Raw)
		return OwnedRational{}, e
	}
	return out, nil
}
func (s *RationalService) partInfInt(v *OwnedInfIntRational, den bool) (_lexOut OwnedInfIntRational, _lexErr error) {
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return OwnedInfIntRational{}, e
		}
		if enabled && v.second != 0 {
			line := int32(145)
			if den {
				line = 150
			}
			if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_int_rational.h", line, "r.m_second == 0"); e != nil {
				return OwnedInfIntRational{}, e
			}
		}
	}
	if e := s.beginLexical(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.endLexicalInfInt(&_lexOut, &_lexErr)
	part := NewRational()
	defer s.destroyLexicalRational(&part)
	var e error
	if den {
		e = s.Manager.DenominatorRational(&v.First, &part)
	} else {
		e = s.Manager.NumeratorRational(&v.First, &part)
	}
	if e != nil {
		return OwnedInfIntRational{}, e
	}
	out := s.newInfInt()
	if e = s.Manager.Copy(&out.First, &part); e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfIntRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfIntNumerator(v *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	return s.partInfInt(v, false)
}
func (s *RationalService) InfIntDenominator(v *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	return s.partInfInt(v, true)
}
func (s *RationalService) roundInfInt(v *OwnedInfIntRational, ceil bool) (_lexOut OwnedRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	out := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	var e error
	if isIntegerRational(&v.First) {
		if (ceil && v.second <= 0) || (!ceil && v.second >= 0) {
			e = s.Manager.Copy(&out.Raw, &v.First)
		} else {
			// Source rational free +/- also uses clone→compound→return copy.
			tmp := NewRational()
			tmpComplete := false
			defer s.endLexicalLocal(&tmp, &tmpComplete)
			e = s.Manager.Copy(&tmp, &v.First)
			if e == nil {
				tmpComplete = true
				if ceil {
					e = s.Manager.Add(&tmp, &s.OneField, &tmp)
				} else {
					e = s.Manager.Sub(&tmp, &s.OneField, &tmp)
				}
			}
			if e == nil {
				e = s.Manager.Copy(&out.Raw, &tmp)
			}
		}
	} else if ceil {
		e = s.Manager.Ceil(&v.First, &out.Raw)
	} else {
		e = s.Manager.Floor(&v.First, &out.Raw)
	}
	if e != nil {
		if isIntegerRational(&v.First) {
			s.abortLexicalRational(&out.Raw)
		} else {
			s.destroyLexicalRational(&out.Raw)
		}
		return OwnedRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfIntFloor(v *OwnedInfIntRational) (OwnedRational, error) {
	if e := s.lockInfInt(v); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.roundInfInt(v, false)
}
func (s *RationalService) InfIntCeil(v *OwnedInfIntRational) (OwnedRational, error) {
	if e := s.lockInfInt(v); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	return s.roundInfInt(v, true)
}

func (s *RationalService) lessInfInt(a *Rational, ae int32, b *Rational, be int32) (bool, error) {
	cmp, e := s.Manager.Compare(a, b)
	if e != nil {
		return false, e
	}
	if cmp < 0 {
		return true, nil
	}
	equal, e := s.Manager.Equal(a, b)
	if e != nil || !equal {
		return false, e
	}
	return ae < be, nil
}
func (s *RationalService) equalInfInt(a *Rational, ae int32, b *Rational, be int32) (bool, error) {
	equal, e := s.Manager.Equal(a, b)
	if e != nil || !equal {
		return false, e
	}
	return ae == be, nil
}

// Scalar projections validate provenance, then follow source component reads.
func (s *RationalService) infIntScalarOwner(v *OwnedInfIntRational) error {
	if v == nil {
		return ErrRationalServiceOwner
	}
	return s.scalarProjectionOwner(v.ownerID)
}
func (s *RationalService) infIntScalarFits(v *OwnedInfIntRational, unsigned bool) (bool, error) {
	var ok bool
	var e error
	if unsigned {
		ok, e = s.Manager.IsUint64(&v.First)
	} else {
		ok, e = s.Manager.IsInt64(&v.First)
	}
	return ok && v.second == 0, e
}
func (s *RationalService) infIntSignedProjectionLocked(v *OwnedInfIntRational) (int64, error) {
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infIntScalarFits(v, false)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_int_rational.h", 113, "is_int64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	return s.Manager.Int64(&v.First)
}
func (s *RationalService) infIntUnsignedProjectionLocked(v *OwnedInfIntRational) (uint64, error) {
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infIntScalarFits(v, true)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_int_rational.h", 118, "is_uint64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	return s.Manager.Uint64(&v.First)
}
func (s *RationalService) InfIntInt64(v *OwnedInfIntRational) (int64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return 0, e
	}
	return s.infIntSignedProjectionLocked(v)
}
func (s *RationalService) InfIntUint64(v *OwnedInfIntRational) (uint64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return 0, e
	}
	return s.infIntUnsignedProjectionLocked(v)
}
func (s *RationalService) InfIntIsZero(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val == 0 && v.second == 0, nil
}
func (s *RationalService) InfIntIsOne(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return isOneInteger(&v.First.Num) && isIntegerRational(&v.First) && v.second == 0, nil
}
func (s *RationalService) InfIntIsMinusOne(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.IsSmall() && v.First.Num.Val == -1 && isIntegerRational(&v.First) && v.second == 0, nil
}
func (s *RationalService) InfIntIsNeg(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val < 0 || (v.First.Num.Val == 0 && v.second < 0), nil
}
func (s *RationalService) InfIntIsPos(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val > 0 || (v.First.Num.Val == 0 && v.second > 0), nil
}
func (s *RationalService) InfIntIsNonNeg(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val > 0 || (v.First.Num.Val == 0 && v.second >= 0), nil
}
func (s *RationalService) InfIntIsNonPos(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val < 0 || (v.First.Num.Val == 0 && v.second <= 0), nil
}
func (s *RationalService) InfIntIsRational(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return v.second == 0, nil
}
func (s *RationalService) InfIntIsInt(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsInt(&v.First)
	return ok && v.second == 0, e
}
func (s *RationalService) InfIntIsInt64(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return s.infIntScalarFits(v, false)
}
func (s *RationalService) InfIntIsUint64(v *OwnedInfIntRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return false, e
	}
	return s.infIntScalarFits(v, true)
}
func (s *RationalService) InfIntEqual(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.equalInfInt(&a.First, a.second, &b.First, b.second)
}
func (s *RationalService) InfIntNotEqual(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.equalInfInt(&a.First, a.second, &b.First, b.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntLess(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&a.First, a.second, &b.First, b.second)
}
func (s *RationalService) InfIntGreater(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&b.First, b.second, &a.First, a.second)
}
func (s *RationalService) InfIntLessEqual(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&b.First, b.second, &a.First, a.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntGreaterEqual(a, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSource2(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&a.First, a.second, &b.First, b.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntEqualRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.equalInfInt(&a.First, a.second, &b.Raw, 0)
}
func (s *RationalService) InfIntNotEqualRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.equalInfInt(&a.First, a.second, &b.Raw, 0)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntLessRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&a.First, a.second, &b.Raw, 0)
}
func (s *RationalService) InfIntGreaterRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&b.Raw, 0, &a.First, a.second)
}
func (s *RationalService) InfIntLessEqualRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&b.Raw, 0, &a.First, a.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntGreaterEqualRational(a *OwnedInfIntRational, b *OwnedRational) (bool, error) {
	if e := s.lockInfIntSourceRat(a, b); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&a.First, a.second, &b.Raw, 0)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntRationalEqual(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.equalInfInt(&a.Raw, 0, &b.First, b.second)
}
func (s *RationalService) InfIntRationalNotEqual(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.equalInfInt(&a.Raw, 0, &b.First, b.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntRationalLess(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&a.Raw, 0, &b.First, b.second)
}
func (s *RationalService) InfIntRationalGreater(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	return s.lessInfInt(&b.First, b.second, &a.Raw, 0)
}
func (s *RationalService) InfIntRationalLessEqual(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&b.First, b.second, &a.Raw, 0)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (s *RationalService) InfIntRationalGreaterEqual(a *OwnedRational, b *OwnedInfIntRational) (bool, error) {
	if e := s.lockInfIntSourceRat(b, a); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	v, e := s.lessInfInt(&a.Raw, 0, &b.First, b.second)
	if e != nil {
		return false, e
	}
	return !v, nil
}

func (s *RationalService) InfIntClone(v *OwnedInfIntRational) (OwnedInfIntRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfIntRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infIntScalarOwner(v); e != nil {
		return OwnedInfIntRational{}, e
	}
	return s.copyInfInt(v)
}

// Source-bound unary/comparison entry checks only provenance/readiness; reached
// numeral operations perform their own actual handle checks.
func (s *RationalService) lockInfIntSource(v *OwnedInfIntRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	if e := s.infIntScalarOwner(v); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
func (s *RationalService) lockInfIntSource2(a, b *OwnedInfIntRational) error {
	if e := s.lockInfIntSource(a); e != nil {
		return e
	}
	if e := s.infIntScalarOwner(b); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
func (s *RationalService) lockInfIntSourceRat(a *OwnedInfIntRational, b *OwnedRational) error {
	if e := s.lockInfIntSource(a); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		s.mu.Unlock()
		return e
	}
	return nil
}
