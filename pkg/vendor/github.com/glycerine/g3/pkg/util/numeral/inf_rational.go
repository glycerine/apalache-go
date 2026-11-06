package numeral

// Source: src/util/inf_rational.h; src/util/inf_rational.cpp.
// Source: src/util/inf_rational.h/.cpp; owner-local rational manager adaptation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrInfRationalDomain = errors.New("mpnum: infinitesimal rational domain")

// OwnedInfRational owns both components. Use InfCopy/Clone/Move/Swap, never a
// shallow copy of a live value. Closing its service invalidates the value.
type OwnedInfRational struct {
	First, Second Rational
	ownerID       uint64
}

func (s *RationalService) initInfRationalConstants() error {
	for _, v := range []*OwnedInfRational{&s.InfZeroField, &s.InfOneField, &s.InfMinusOneField} {
		*v = OwnedInfRational{First: NewRational(), Second: NewRational(), ownerID: s.ownerID}
	}
	if e := s.Manager.Copy(&s.InfZeroField.First, &s.ZeroField); e != nil {
		return e
	}
	if e := s.Manager.Copy(&s.InfOneField.First, &s.OneField); e != nil {
		return e
	}
	return s.Manager.Copy(&s.InfMinusOneField.First, &s.MinusOneField)
}
func (s *RationalService) destroyInfRationalConstants() {
	for _, v := range []*OwnedInfRational{&s.InfZeroField, &s.InfOneField, &s.InfMinusOneField} {
		s.infDestroy(v)
	}
}
func (s *RationalService) infDestroy(v *OwnedInfRational) error {
	e := s.destroyLexicalRational(&v.Second)
	if next := s.destroyLexicalRational(&v.First); e == nil {
		e = next
	}
	*v = OwnedInfRational{}
	return e
}

// A failed SECOND copy constructor never owns a completed SECOND rational.
// Destroy the completed FIRST before explicitly aborting partial SECOND storage.
func (s *RationalService) abortInfClone(v *OwnedInfRational, completed uint8) {
	if completed == 1 {
		s.destroyLexicalRational(&v.First)
	} else {
		s.abortLexicalRational(&v.First)
	}
	s.abortLexicalRational(&v.Second)
	*v = OwnedInfRational{}
}
func (s *RationalService) infCheck(v *OwnedInfRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.check(&v.First); e != nil {
		return e
	}
	return s.Manager.check(&v.Second)
}
func (s *RationalService) infCheckCleanup(v *OwnedInfRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.checkCleanup(&v.First); e != nil {
		return e
	}
	return s.Manager.checkCleanup(&v.Second)
}
func (s *RationalService) infEmpty() OwnedInfRational {
	return OwnedInfRational{First: NewRational(), Second: NewRational(), ownerID: s.ownerID}
}
func (s *RationalService) infCopy(dst, src *OwnedInfRational) error {
	if e := s.Manager.Copy(&dst.First, &src.First); e != nil {
		return e
	}
	return s.Manager.Copy(&dst.Second, &src.Second)
}
func (s *RationalService) infClone(src *OwnedInfRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	v := s.infEmpty()
	if e := s.Manager.Copy(&v.First, &src.First); e != nil {
		s.abortInfClone(&v, 0)
		return OwnedInfRational{}, e
	}
	if e := s.Manager.Copy(&v.Second, &src.Second); e != nil {
		s.abortInfClone(&v, 1)
		return OwnedInfRational{}, e
	}
	return v, nil
}
func (s *RationalService) NewInfValue() (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	return s.infEmpty(), nil
}

// InfFromInt32 follows inf_rational(int): construct FIRST from the integer,
// then complete a default zero EPS member without copying a temporary value.
func (s *RationalService) InfFromInt32(n int32) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	v := s.infEmpty()
	if e := s.Manager.SetInt64(&v.First, int64(n)); e != nil {
		s.abortLexicalRational(&v.First)
		return OwnedInfRational{}, e
	}
	return v, nil
}

// InfFromRational constructs m_first from the source value, then resets the
// default-constructed infinitesimal member, as in inf_rational(rational const&).
func (s *RationalService) InfFromRational(r *OwnedRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfRational{}, e
	}
	v := s.infEmpty()
	if e := s.Manager.Copy(&v.First, &r.Raw); e != nil {
		s.abortLexicalRational(&v.First)
		return OwnedInfRational{}, e
	}
	if e := s.Manager.Reset(&v.Second); e != nil {
		s.destroyLexicalRational(&v.Second)
		s.destroyLexicalRational(&v.First)
		return OwnedInfRational{}, e
	}
	return v, nil
}

// Both two-argument constructors complete FIRST before copying EPS. A failed
// EPS copy destroys the completed FIRST and aborts only the incomplete EPS.
func (s *RationalService) infFromPairLocked(first, second *Rational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	v := s.infEmpty()
	if e := s.Manager.Copy(&v.First, first); e != nil {
		s.abortLexicalRational(&v.First)
		return OwnedInfRational{}, e
	}
	if e := s.Manager.Copy(&v.Second, second); e != nil {
		s.destroyLexicalRational(&v.First)
		s.abortLexicalRational(&v.Second)
		return OwnedInfRational{}, e
	}
	return v, nil
}

// InfFromPair is inf_rational(rational const&, rational const&).
func (s *RationalService) InfFromPair(first, second *OwnedRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(first); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.rationalScalarOwner(second); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infFromPairLocked(&first.Raw, &second.Raw)
}

// InfFromOffset is inf_rational(rational const&, bool). The epsilon source is
// the service's static one or minus-one rational.
func (s *RationalService) InfFromOffset(first *OwnedRational, positive bool) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(first); e != nil {
		return OwnedInfRational{}, e
	}
	second := &s.MinusOneField
	if positive {
		second = &s.OneField
	}
	return s.infFromPairLocked(&first.Raw, second)
}
func (s *RationalService) InfClone(src *OwnedInfRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(src); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infClone(src)
}
func (s *RationalService) InfCopy(dst, src *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(dst); e != nil {
		return e
	}
	if e := s.infScalarOwner(src); e != nil {
		return e
	}
	return s.infCopy(dst, src)
}
func (s *RationalService) InfDel(v *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := callerDeleteRational(s.Manager.Z, &v.Second); e != nil {
		return e
	}
	if e := callerDeleteRational(s.Manager.Z, &v.First); e != nil {
		return e
	}
	*v = OwnedInfRational{}
	return nil
}
func (s *RationalService) InfReset(v *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.Manager.Reset(&v.First); e != nil {
		return e
	}
	return s.Manager.Reset(&v.Second)
}
func (s *RationalService) InfMove(dst, src *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infCheck(dst); e != nil {
		return e
	}
	if e := s.infCheck(src); e != nil {
		return e
	}
	if e := s.Manager.Move(&dst.First, &src.First); e != nil {
		return e
	}
	return s.Manager.Move(&dst.Second, &src.Second)
}
func (s *RationalService) InfSwap(a, b *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return e
	}
	if e := s.infScalarOwner(b); e != nil {
		return e
	}
	if e := s.Manager.Swap(&a.First, &b.First); e != nil {
		return e
	}
	return s.Manager.Swap(&a.Second, &b.Second)
}
func (s *RationalService) InfSetRational(v *OwnedInfRational, a *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if e := s.Manager.Copy(&v.First, &a.Raw); e != nil {
		return e
	}
	return s.Manager.Reset(&v.Second)
}
func (s *RationalService) InfSetPair(v *OwnedInfRational, a, b *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return e
	}
	if e := s.Manager.Copy(&v.First, &a.Raw); e != nil {
		return e
	}
	return s.Manager.Copy(&v.Second, &b.Raw)
}
func (s *RationalService) InfSetOffset(v *OwnedInfRational, a *OwnedRational, pos bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if e := s.Manager.Copy(&v.First, &a.Raw); e != nil {
		return e
	}
	c := &s.MinusOneField
	if pos {
		c = &s.OneField
	}
	return s.Manager.Copy(&v.Second, c)
}
func (s *RationalService) infConstant(v *OwnedInfRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	return s.infClone(v)
}
func (s *RationalService) InfZero() (OwnedInfRational, error) {
	if s == nil {
		return OwnedInfRational{}, ErrRationalServiceCopy
	}
	return s.infConstant(&s.InfZeroField)
}
func (s *RationalService) InfOne() (OwnedInfRational, error) {
	if s == nil {
		return OwnedInfRational{}, ErrRationalServiceCopy
	}
	return s.infConstant(&s.InfOneField)
}
func (s *RationalService) InfMinusOne() (OwnedInfRational, error) {
	if s == nil {
		return OwnedInfRational{}, ErrRationalServiceCopy
	}
	return s.infConstant(&s.InfMinusOneField)
}
func (s *RationalService) InfGetRational(v *OwnedInfRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.First)
}
func (s *RationalService) InfGetFirst(v *OwnedInfRational) (OwnedRational, error) {
	return s.InfGetRational(v)
}
func (s *RationalService) InfGetInfinitesimal(v *OwnedInfRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.Second)
}
func (s *RationalService) InfHash(v *OwnedInfRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return 0, e
	}
	a, e := s.Manager.Hash(&v.First)
	if e != nil {
		return 0, e
	}
	b, e := s.Manager.Hash(&v.Second)
	if e != nil {
		return 0, e
	}
	return a ^ (b + 1), nil
}
func (s *RationalService) InfString(v *OwnedInfRational) (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return "", e
	}
	n, e := s.infEpsInfPrepareNumeral(v, 0)
	if e != nil {
		return "", e
	}
	return s.Manager.Z.Natural.finalString(n)
}
func (s *RationalService) infSign(v *OwnedInfRational) int32 {
	if v.First.Num.Val != 0 {
		return v.First.Num.Val
	}
	return v.Second.Num.Val
}
func (s *RationalService) infCompare(a, b *OwnedInfRational) (int, error) {
	n, e := s.Manager.Compare(&a.First, &b.First)
	if e != nil || n < 0 {
		return n, e
	}
	eq, e := s.Manager.Equal(&a.First, &b.First)
	if e != nil {
		return 0, e
	}
	if eq {
		return s.Manager.Compare(&a.Second, &b.Second)
	}
	return 1, nil
}
func (s *RationalService) InfCompare(a, b *OwnedInfRational) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infCheck(a); e != nil {
		return 0, e
	}
	if e := s.infCheck(b); e != nil {
		return 0, e
	}
	return s.infCompare(a, b)
}
func (s *RationalService) InfCompareRational(a *OwnedInfRational, b *OwnedRational) (int, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infCheck(a); e != nil {
		return 0, e
	}
	if e := s.check(b); e != nil {
		return 0, e
	}
	n, e := s.Manager.Compare(&a.First, &b.Raw)
	if e != nil || n < 0 {
		return n, e
	}
	eq, e := s.Manager.Equal(&a.First, &b.Raw)
	if e != nil {
		return 0, e
	}
	if !eq {
		return 1, nil
	}
	if a.Second.Num.Val < 0 {
		return -1, nil
	}
	if a.Second.Num.Val > 0 {
		return 1, nil
	}
	return 0, nil
}
func (s *RationalService) InfEqual(a, b *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.Manager.Equal(&a.First, &b.First)
	if e != nil || !ok {
		return ok, e
	}
	return s.Manager.Equal(&a.Second, &b.Second)
}
func (s *RationalService) InfEqualRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.Manager.Equal(&a.First, &b.Raw)
	return ok && a.Second.Num.Val == 0, e
}
func (s *RationalService) infCompound(a, b *OwnedInfRational, sub bool) error {
	if sub {
		if e := s.Manager.Sub(&a.First, &b.First, &a.First); e != nil {
			return e
		}
		return s.Manager.Sub(&a.Second, &b.Second, &a.Second)
	}
	if e := s.Manager.Add(&a.First, &b.First, &a.First); e != nil {
		return e
	}
	return s.Manager.Add(&a.Second, &b.Second, &a.Second)
}
func (s *RationalService) infScalar(v *OwnedInfRational, r *Rational, op byte) error {
	if e := s.infRatCompound(&v.First, r, op); e != nil {
		return e
	}
	if op == '+' || op == '-' {
		return nil
	}
	return s.infRatCompound(&v.Second, r, op)
}
func (s *RationalService) infRatCompound(a, b *Rational, op byte) error {
	switch op {
	case '+':
		return s.Manager.Add(a, b, a)
	case '-':
		return s.Manager.Sub(a, b, a)
	case '*':
		return s.Manager.Mul(a, b, a)
	default:
		return s.Manager.Div(a, b, a)
	}
}

// Free compound expressions return an lvalue: source copies it a second time.
func (s *RationalService) infRatExpr(a, b, out *Rational, op byte) (_lexErr error) {
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	work := NewRational()
	workComplete := false
	defer s.endLexicalLocal(&work, &workComplete)
	if e := s.Manager.Copy(&work, a); e != nil {
		return e
	}
	workComplete = true
	if e := s.infRatCompound(&work, b, op); e != nil {
		return e
	}
	return s.Manager.Copy(out, &work)
}
func (s *RationalService) infBinary(a, b *OwnedInfRational, sub bool) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	v, e := s.infClone(a)
	if e != nil {
		return OwnedInfRational{}, e
	}
	defer s.infDestroy(&v)
	if e = s.infCompound(&v, b, sub); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infClone(&v)
}
func (s *RationalService) infScalarResult(v *OwnedInfRational, r *Rational, op byte) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	out, e := s.infClone(v)
	if e != nil {
		return OwnedInfRational{}, e
	}
	if e = s.infScalar(&out, r, op); e != nil {
		s.infDestroy(&out)
		return OwnedInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfAbs(v *OwnedInfRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	out, e := s.infClone(v)
	if e != nil {
		return OwnedInfRational{}, e
	}
	if s.infSign(&out) < 0 {
		if e = s.infNeg(&out); e != nil {
			s.infDestroy(&out)
			return OwnedInfRational{}, e
		}
	}
	return out, nil
}
func (s *RationalService) infNeg(v *OwnedInfRational) error {
	if e := s.infScalarOwner(v); e != nil {
		return e
	}

	if e := s.Manager.Neg(&v.First); e != nil {
		return e
	}
	return s.Manager.Neg(&v.Second)
}
func (s *RationalService) InfNeg(v *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	return s.infNeg(v)
}
func (s *RationalService) infNumDen(v *OwnedInfRational, den bool) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return OwnedInfRational{}, e
		}
		if enabled && v.Second.Num.Val != 0 {
			line := int32(158)
			if den {
				line = 163
			}
			if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_rational.h", line, "r.m_second.is_zero()"); e != nil {
				return OwnedInfRational{}, e
			}
		}
	}
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	tmp := NewRational()
	defer s.destroyLexicalRational(&tmp)
	var e error
	if den {
		e = s.Manager.DenominatorRational(&v.First, &tmp)
	} else {
		e = s.Manager.NumeratorRational(&v.First, &tmp)
	}
	if e != nil {
		return OwnedInfRational{}, e
	}
	out := s.infEmpty()
	if e = s.Manager.Copy(&out.First, &tmp); e != nil {
		s.abortLexicalRational(&out.First)
		return OwnedInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfNumerator(v *OwnedInfRational) (OwnedInfRational, error) {
	return s.infNumDen(v, false)
}
func (s *RationalService) InfDenominator(v *OwnedInfRational) (OwnedInfRational, error) {
	return s.infNumDen(v, true)
}
func (s *RationalService) infRound(v *OwnedInfRational, ceil bool) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	out := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	var e error
	if isIntegerRational(&v.First) {
		adjust := v.Second.Num.Val < 0
		if ceil {
			adjust = v.Second.Num.Val > 0
		}
		if adjust {
			op := byte('-')
			if ceil {
				op = '+'
			}
			e = s.infRatExpr(&v.First, &s.OneField, &out.Raw, op)
		} else {
			e = s.Manager.Copy(&out.Raw, &v.First)
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
func (s *RationalService) InfFloor(v *OwnedInfRational) (OwnedRational, error) {
	return s.infRound(v, false)
}
func (s *RationalService) InfCeil(v *OwnedInfRational) (OwnedRational, error) {
	return s.infRound(v, true)
}

// rational::addmul/submul clone k then multiply before accumulating; these are
// intentionally not replaced by manager fused arithmetic.
func (s *RationalService) infRatFused(v, c, k *Rational, sub bool) (_lexErr error) {
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	op := byte('+')
	if sub {
		op = '-'
	}
	if isOneInteger(&c.Num) && isIntegerRational(c) {
		return s.infRatCompound(v, k, op)
	}
	if c.Num.IsSmall() && c.Num.Val == -1 && isIntegerRational(c) {
		if sub {
			op = '+'
		} else {
			op = '-'
		}
		return s.infRatCompound(v, k, op)
	}
	tmp := NewRational()
	tmpComplete := false
	defer s.endLexicalLocal(&tmp, &tmpComplete)
	if e := s.Manager.Copy(&tmp, k); e != nil {
		return e
	}
	tmpComplete = true
	if e := s.Manager.Mul(&tmp, c, &tmp); e != nil {
		return e
	}
	return s.infRatCompound(v, &tmp, op)
}
func (s *RationalService) infFused(v *OwnedInfRational, c *OwnedRational, k *OwnedInfRational, sub bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.check(c); e != nil {
		return e
	}
	if e := s.infScalarOwner(k); e != nil {
		return e
	}
	if e := s.infRatFused(&v.First, &c.Raw, &k.First, sub); e != nil {
		return e
	}
	return s.infRatFused(&v.Second, &c.Raw, &k.Second, sub)
}
func (s *RationalService) InfAddMulAssign(v *OwnedInfRational, c *OwnedRational, k *OwnedInfRational) error {
	return s.infFused(v, c, k, false)
}
func (s *RationalService) InfSubMulAssign(v *OwnedInfRational, c *OwnedRational, k *OwnedInfRational) error {
	return s.infFused(v, c, k, true)
}

// Scalar projections validate provenance, then follow source component reads.
func (s *RationalService) infScalarOwner(v *OwnedInfRational) error {
	if v == nil {
		return ErrRationalServiceOwner
	}
	return s.scalarProjectionOwner(v.ownerID)
}
func (s *RationalService) infScalarFits(v *OwnedInfRational, unsigned bool) (bool, error) {
	var ok bool
	var e error
	if unsigned {
		ok, e = s.Manager.IsUint64(&v.First)
	} else {
		ok, e = s.Manager.IsInt64(&v.First)
	}
	return ok && v.Second.Num.Val == 0, e
}
func (s *RationalService) infSignedProjectionLocked(v *OwnedInfRational) (int64, error) {
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infScalarFits(v, false)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_rational.h", 126, "is_int64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	return s.Manager.Int64(&v.First)
}
func (s *RationalService) infUnsignedProjectionLocked(v *OwnedInfRational) (uint64, error) {
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infScalarFits(v, true)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_rational.h", 131, "is_uint64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	return s.Manager.Uint64(&v.First)
}
func (s *RationalService) InfIsInt(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsInt(&v.First)
	return ok && v.Second.Num.Val == 0, e
}
func (s *RationalService) InfIsRational(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return v.Second.Num.Val == 0, nil
}
func (s *RationalService) InfIsZero(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return v.First.Num.Val == 0 && v.Second.Num.Val == 0, nil
}
func (s *RationalService) InfIsOne(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsOne(&v.First)
	return ok && v.Second.Num.Val == 0, e
}
func (s *RationalService) InfIsMinusOne(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsMinusOne(&v.First)
	return ok && v.Second.Num.Val == 0, e
}
func (s *RationalService) InfIsPos(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infSign(v) > 0, nil
}
func (s *RationalService) InfIsNeg(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infSign(v) < 0, nil
}
func (s *RationalService) InfIsNonPos(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infSign(v) <= 0, nil
}
func (s *RationalService) InfIsNonNeg(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infSign(v) >= 0, nil
}
func (s *RationalService) InfIsInt64(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infScalarFits(v, false)
}
func (s *RationalService) InfIsUint64(v *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return false, e
	}
	return s.infScalarFits(v, true)
}
func (s *RationalService) InfInt64(v *OwnedInfRational) (int64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return 0, e
	}
	return s.infSignedProjectionLocked(v)
}
func (s *RationalService) InfUint64(v *OwnedInfRational) (uint64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return 0, e
	}
	return s.infUnsignedProjectionLocked(v)
}
func (s *RationalService) InfAddAssign(a, b *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return e
	}
	if e := s.infScalarOwner(b); e != nil {
		return e
	}
	return s.infCompound(a, b, false)
}
func (s *RationalService) InfAdd(a, b *OwnedInfRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infBinary(a, b, false)
}
func (s *RationalService) InfSubAssign(a, b *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return e
	}
	if e := s.infScalarOwner(b); e != nil {
		return e
	}
	return s.infCompound(a, b, true)
}
func (s *RationalService) InfSub(a, b *OwnedInfRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infBinary(a, b, true)
}
func (s *RationalService) InfAddRationalAssign(v *OwnedInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infScalar(v, &r.Raw, '+')
}
func (s *RationalService) InfSubRationalAssign(v *OwnedInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infScalar(v, &r.Raw, '-')
}
func (s *RationalService) InfMulRationalAssign(v *OwnedInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infScalar(v, &r.Raw, '*')
}
func (s *RationalService) InfMulRational(v *OwnedInfRational, r *OwnedRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infScalarResult(v, &r.Raw, '*')
}
func (s *RationalService) InfDivRationalAssign(v *OwnedInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infScalar(v, &r.Raw, '/')
}
func (s *RationalService) InfDivRational(v *OwnedInfRational, r *OwnedRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infScalarResult(v, &r.Raw, '/')
}
func (s *RationalService) InfInc(v *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	return s.infRatStep(&v.First, false)
}
func (s *RationalService) InfPostInc(v *OwnedInfRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	if e := s.infCheck(v); e != nil {
		return OwnedInfRational{}, e
	}
	out, e := s.infClone(v)
	if e != nil {
		return OwnedInfRational{}, e
	}
	if e = s.infRatStep(&v.First, false); e != nil {
		s.infDestroy(&out)
		return OwnedInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfDec(v *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	return s.infRatStep(&v.First, true)
}
func (s *RationalService) InfPostDec(v *OwnedInfRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	if e := s.infCheck(v); e != nil {
		return OwnedInfRational{}, e
	}
	out, e := s.infClone(v)
	if e != nil {
		return OwnedInfRational{}, e
	}
	if e = s.infRatStep(&v.First, true); e != nil {
		s.infDestroy(&out)
		return OwnedInfRational{}, e
	}
	return out, nil
}

func (s *RationalService) infLess(a, b *OwnedInfRational) (bool, error) {
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}

	n, e := s.Manager.Compare(&a.First, &b.First)
	if e != nil || n < 0 {
		return n < 0, e
	}
	eq, e := s.Manager.Equal(&a.First, &b.First)
	if e != nil || !eq {
		return false, e
	}
	n, e = s.Manager.Compare(&a.Second, &b.Second)
	return n < 0, e
}
func (s *RationalService) InfLess(a, b *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLess(a, b)
	return ok, e
}
func (s *RationalService) InfGreater(a, b *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLess(b, a)
	return ok, e
}
func (s *RationalService) InfLessEqual(a, b *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLess(b, a)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfGreaterEqual(a, b *OwnedInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLess(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}

func (s *RationalService) InfSetInt32(v *OwnedInfRational, n int32) (_lexErr error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	tmp := NewRational()
	tmpComplete := false
	defer s.endLexicalLocal(&tmp, &tmpComplete)
	if e := s.Manager.SetInt64(&tmp, int64(n)); e != nil {
		return e
	}
	tmpComplete = true
	if e := s.Manager.Copy(&v.First, &tmp); e != nil {
		return e
	}
	return s.Manager.Reset(&v.Second)
}
func (s *RationalService) InfSetFractionSigned(v *OwnedInfRational, n, d int32) (_lexErr error) {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return e
	}
	work := NewRational()
	workComplete := false
	defer s.endLexicalLocal(&work, &workComplete)
	if e := s.Manager.SetFractionSigned(&work, n, d); e != nil {
		return e
	}
	workComplete = true
	if e := s.Manager.Copy(&v.First, &work); e != nil {
		return e
	}
	return s.Manager.Reset(&v.Second)
}
func (s *RationalService) InfUnaryMinus(v *OwnedInfRational) (_lexOut OwnedInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.endLexicalInf(&_lexOut, &_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	out, e := s.infClone(v)
	if e != nil {
		return OwnedInfRational{}, e
	}
	if e = s.infNeg(&out); e != nil {
		s.infDestroy(&out)
		return OwnedInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfNotEqual(a, b *OwnedInfRational) (bool, error) {
	ok, e := s.InfEqual(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfNotEqualRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	ok, e := s.InfEqualRational(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) infLessRational(a *OwnedInfRational, b *OwnedRational, reverse bool) (bool, error) {
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}

	left, right := &a.First, &b.Raw
	if reverse {
		left, right = right, left
	}
	n, e := s.Manager.Compare(left, right)
	if e != nil || n < 0 {
		return n < 0, e
	}
	eq, e := s.Manager.Equal(left, right)
	if e != nil || !eq {
		return false, e
	}
	if reverse {
		return a.Second.Num.Val > 0, nil
	}
	return a.Second.Num.Val < 0, nil
}
func (s *RationalService) InfLessRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLessRational(a, b, false)
	return ok, e
}
func (s *RationalService) InfGreaterRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLessRational(a, b, true)
	return ok, e
}
func (s *RationalService) InfLessEqualRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLessRational(a, b, true)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfGreaterEqualRational(a *OwnedInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infLessRational(a, b, false)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfRationalLess(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfGreaterRational(b, a)
}
func (s *RationalService) InfRationalGreater(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfLessRational(b, a)
}
func (s *RationalService) InfRationalLessEqual(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfGreaterEqualRational(b, a)
}
func (s *RationalService) InfRationalGreaterEqual(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfLessEqualRational(b, a)
}
func (s *RationalService) InfRationalEqual(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfEqualRational(b, a)
}
func (s *RationalService) InfRationalNotEqual(a *OwnedRational, b *OwnedInfRational) (bool, error) {
	return s.InfNotEqualRational(b, a)
}

// rational::++/-- call rational Add/Sub with mk_q(1), not mixed AddInt.
func (s *RationalService) infRatStep(v *Rational, dec bool) error {
	one := NewRational()
	one.Num = NewInt(1)
	if dec {
		return s.Manager.Sub(v, &one, v)
	}
	return s.Manager.Add(v, &one, v)
}
