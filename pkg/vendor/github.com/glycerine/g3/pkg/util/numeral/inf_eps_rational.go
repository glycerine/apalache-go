package numeral

// Source: src/util/inf_eps_rational.h.
// Translation of src/util/inf_eps_rational.h<inf_rational>. Selected raw-domain
// projections/sign/hash retain source reads; diagnostic cleanup-refusal ownership
// and other raw-domain arithmetic adapters remain separate obligations.

import "github.com/glycerine/g3/pkg/util/diagnostic"

// OwnedInfEpsInfRational represents infinity*i + (first + epsilon*second).
// Infinity is a rational coefficient, not a saturating flag. Every component
// is owned; never shallow-copy a live value. Its service owns numeric storage.
type OwnedInfEpsInfRational struct {
	infinity Rational
	numeral  OwnedInfRational
	ownerID  uint64
}

func (s *RationalService) infEpsInfEmpty() OwnedInfEpsInfRational {
	return OwnedInfEpsInfRational{infinity: NewRational(), numeral: s.infEmpty(), ownerID: s.ownerID}
}
func (s *RationalService) infEpsInfCheck(v *OwnedInfEpsInfRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.check(&v.infinity); e != nil {
		return e
	}
	return s.infCheck(&v.numeral)
}
func (s *RationalService) infEpsInfCheckCleanup(v *OwnedInfEpsInfRational) error {
	if v == nil || v.ownerID == 0 || v.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	if e := s.Manager.checkCleanup(&v.infinity); e != nil {
		return e
	}
	return s.infCheckCleanup(&v.numeral)
}
func (s *RationalService) infEpsInfDestroy(v *OwnedInfEpsInfRational) error {
	e := s.infDestroy(&v.numeral)
	if next := s.destroyLexicalRational(&v.infinity); e == nil {
		e = next
	}
	*v = OwnedInfEpsInfRational{}
	return e
}
func (s *RationalService) infEpsInfCopy(dst, src *OwnedInfEpsInfRational) error {
	if e := s.Manager.Copy(&dst.infinity, &src.infinity); e != nil {
		return e
	}
	if e := s.infScalarOwner(&dst.numeral); e != nil {
		return e
	}
	if e := s.infScalarOwner(&src.numeral); e != nil {
		return e
	}
	return s.infCopy(&dst.numeral, &src.numeral)
}
func (s *RationalService) infEpsInfClone(src *OwnedInfEpsInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	v := s.infEpsInfEmpty()
	if e := s.Manager.Copy(&v.infinity, &src.infinity); e != nil {
		s.abortLexicalRational(&v.infinity)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.infScalarOwner(&src.numeral); e != nil {
		s.destroyLexicalRational(&v.infinity)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.Manager.Copy(&v.numeral.First, &src.numeral.First); e != nil {
		// Nested construction unwinds no completed FIRST. The completed infinity
		// rational is the outer constructor's source-owned unwind obligation.
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.First)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.Manager.Copy(&v.numeral.Second, &src.numeral.Second); e != nil {
		s.destroyLexicalRational(&v.numeral.First)
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.Second)
		return OwnedInfEpsInfRational{}, e
	}
	return v, nil
}

// The pair constructor builds m_infty before the nested Numeral. A failed
// nested copy destroys only members whose constructors finished.
func (s *RationalService) infEpsInfFromPairLocked(i *Rational, n *OwnedInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	v := s.infEpsInfEmpty()
	if e := s.Manager.Copy(&v.infinity, i); e != nil {
		s.abortLexicalRational(&v.infinity)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.Manager.Copy(&v.numeral.First, &n.First); e != nil {
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.First)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.Manager.Copy(&v.numeral.Second, &n.Second); e != nil {
		s.destroyLexicalRational(&v.numeral.First)
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.Second)
		return OwnedInfEpsInfRational{}, e
	}
	return v, nil
}

// InfEpsInfFromPair is the retained (rational, Numeral) value constructor.
func (s *RationalService) InfEpsInfFromPair(i *OwnedRational, n *OwnedInfRational) (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.rationalScalarOwner(i); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.infScalarOwner(n); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	return s.infEpsInfFromPairLocked(&i.Raw, n)
}

// The Numeral constructor first completes the default infinity member, then
// copies the nested FIRST and EPS rationals in their declaration order.
func (s *RationalService) infEpsInfFromNumeralLocked(n *OwnedInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	v := s.infEpsInfEmpty()
	if e := s.Manager.Copy(&v.numeral.First, &n.First); e != nil {
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.First)
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.Manager.Copy(&v.numeral.Second, &n.Second); e != nil {
		s.destroyLexicalRational(&v.numeral.First)
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.Second)
		return OwnedInfEpsInfRational{}, e
	}
	return v, nil
}

// InfEpsInfFromNumeral is the retained Numeral value constructor.
func (s *RationalService) InfEpsInfFromNumeral(n *OwnedInfRational) (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infScalarOwner(n); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	return s.infEpsInfFromNumeralLocked(n)
}

// InfEpsInfFromInt32 follows inf_eps_rational(int): default-construct the
// infinity coefficient, then construct nested FIRST from n and default EPS.
func (s *RationalService) InfEpsInfFromInt32(n int32) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	v := s.infEpsInfEmpty()
	if e := s.Manager.SetInt64(&v.numeral.First, int64(n)); e != nil {
		s.destroyLexicalRational(&v.infinity)
		s.abortLexicalRational(&v.numeral.First)
		return OwnedInfEpsInfRational{}, e
	}
	return v, nil
}

func (s *RationalService) InfEpsInfNewValue() (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	return s.infEpsInfEmpty(), nil
}
func (s *RationalService) InfEpsInfClone(src *OwnedInfEpsInfRational) (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(src); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	return s.infEpsInfClone(src)
}
func (s *RationalService) InfEpsInfCopy(dst, src *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(dst); e != nil {
		return e
	}
	if e := s.infEpsScalarOwner(src); e != nil {
		return e
	}
	return s.infEpsInfCopy(dst, src)
}
func (s *RationalService) InfEpsInfDel(v *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return e
	}
	if e := callerDeleteRational(s.Manager.Z, &v.numeral.Second); e != nil {
		return e
	}
	if e := callerDeleteRational(s.Manager.Z, &v.numeral.First); e != nil {
		return e
	}
	if e := callerDeleteRational(s.Manager.Z, &v.infinity); e != nil {
		return e
	}
	*v = OwnedInfEpsInfRational{}
	return nil
}
func (s *RationalService) InfEpsInfReset(v *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.Manager.Reset(&v.infinity); e != nil {
		return e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return e
	}
	if e := s.Manager.Reset(&v.numeral.First); e != nil {
		return e
	}
	return s.Manager.Reset(&v.numeral.Second)
}
func (s *RationalService) InfEpsInfSwap(a, b *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return e
	}
	if e := s.Manager.Swap(&a.infinity, &b.infinity); e != nil {
		return e
	}
	if e := s.infScalarOwner(&a.numeral); e != nil {
		return e
	}
	if e := s.infScalarOwner(&b.numeral); e != nil {
		return e
	}
	if e := s.Manager.Swap(&a.numeral.First, &b.numeral.First); e != nil {
		return e
	}
	return s.Manager.Swap(&a.numeral.Second, &b.numeral.Second)
}
func (s *RationalService) InfEpsInfSetNumeral(v *OwnedInfEpsInfRational, n *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(n); e != nil {
		return e
	}
	if e := s.Manager.Reset(&v.infinity); e != nil {
		return e
	}
	return s.infCopy(&v.numeral, n)
}
func (s *RationalService) InfEpsInfSetPair(v *OwnedInfEpsInfRational, i *OwnedRational, n *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(i); e != nil {
		return e
	}
	if e := s.infScalarOwner(n); e != nil {
		return e
	}
	if e := s.Manager.Copy(&v.infinity, &i.Raw); e != nil {
		return e
	}
	return s.infCopy(&v.numeral, n)
}
func (s *RationalService) InfEpsInfSetInt32(v *OwnedInfEpsInfRational, n int32) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.Manager.Reset(&v.infinity); e != nil {
		return e
	}
	// m_r(n) invokes inf_rational(int): rational(n) initializes its first
	// component through move/elision, not a further copy constructor.
	if e := s.Manager.SetInt64(&v.numeral.First, int64(n)); e != nil {
		return e
	}
	return s.Manager.Reset(&v.numeral.Second)
}
func (s *RationalService) infEpsInfConstant(which uint8) (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	n := &s.InfZeroField
	if which == 1 {
		n = &s.InfOneField
	} else if which == 2 {
		n = &s.InfMinusOneField
	}
	return s.infEpsInfFromNumeralLocked(n)
}
func (s *RationalService) InfEpsInfZero() (OwnedInfEpsInfRational, error) {
	return s.infEpsInfConstant(0)
}
func (s *RationalService) InfEpsInfOne() (OwnedInfEpsInfRational, error) {
	return s.infEpsInfConstant(1)
}
func (s *RationalService) InfEpsInfMinusOne() (OwnedInfEpsInfRational, error) {
	return s.infEpsInfConstant(2)
}
func (s *RationalService) InfEpsInfInfinity() (OwnedInfEpsInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	return s.infEpsInfFromPairLocked(&s.OneField, &s.InfZeroField)
}
func (s *RationalService) InfEpsInfGetNumeral(v *OwnedInfEpsInfRational) (OwnedInfRational, error) {
	if e := s.lock(); e != nil {
		return OwnedInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedInfRational{}, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return OwnedInfRational{}, e
	}
	return s.infClone(&v.numeral)
}
func (s *RationalService) InfEpsInfHash(v *OwnedInfEpsInfRational) (uint32, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return 0, e
	}
	i, e := s.Manager.Hash(&v.infinity)
	if e != nil {
		return 0, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return 0, e
	}
	a, e := s.Manager.Hash(&v.numeral.First)
	if e != nil {
		return 0, e
	}
	b, e := s.Manager.Hash(&v.numeral.Second)
	if e != nil {
		return 0, e
	}
	return i ^ (a ^ (b + 1)), nil
}

// Offset-aware formatting keeps every intermediate byte in the existing
// accounted manager workspace. Only finalString creates host output storage.
func (s *RationalService) infEpsInfPrepareRational(v *Rational, offset uint32) (uint32, error) {
	if e := s.Manager.check(v); e != nil {
		return 0, e
	}
	n, e := s.Manager.Z.prepareInteger(&v.Num, offset)
	if e != nil {
		return 0, e
	}
	if isIntegerRational(v) {
		return n, nil
	}
	n, e = s.Manager.Z.Natural.appendDisplay(n, "/")
	if e != nil {
		return 0, e
	}
	return s.Manager.Z.prepareInteger(&v.Den, n)
}
func (s *RationalService) infEpsInfPrepareNumeral(v *OwnedInfRational, offset uint32) (_lexValue uint32, _lexErr error) {
	if e := s.beginLexical(); e != nil {
		return 0, e
	}
	defer s.endLexicalError(&_lexErr)
	if e := s.infScalarOwner(v); e != nil {
		return 0, e
	}
	zero, e := s.Manager.IsZero(&v.Second)
	if e != nil {
		return 0, e
	}
	if zero {
		return s.infEpsInfPrepareRational(&v.First, offset)
	}
	n, e := s.Manager.Z.Natural.appendDisplay(offset, "(")
	if e != nil {
		return 0, e
	}
	n, e = s.infEpsInfPrepareRational(&v.First, n)
	if e != nil {
		return 0, e
	}
	negative, e := s.Manager.IsNeg(&v.Second)
	if e != nil {
		return 0, e
	}
	sign := " +e*"
	if negative {
		sign = " -e*"
	}
	n, e = s.Manager.Z.Natural.appendDisplay(n, sign)
	if e != nil {
		return 0, e
	}
	abs := NewRational()
	absComplete := false
	defer s.endLexicalLocal(&abs, &absComplete)
	if e = s.Manager.Copy(&abs, &v.Second); e != nil {
		return 0, e
	}
	absComplete = true
	if e = s.Manager.Abs(&abs); e != nil {
		return 0, e
	}
	n, e = s.infEpsInfPrepareRational(&abs, n)
	// abs(...).to_string() full-expression ends BEFORE the closing parenthesis.
	if cleanup := s.destroyLexicalRational(&abs); e == nil {
		e = cleanup
	}
	abs = NewRational()
	if e != nil {
		return 0, e
	}
	return s.Manager.Z.Natural.appendDisplay(n, ")")
}
func (s *RationalService) infEpsInfPrepare(v *OwnedInfEpsInfRational) (uint32, error) {
	zero, e := s.Manager.IsZero(&v.infinity)
	if e != nil {
		return 0, e
	}
	if zero {
		return s.infEpsInfPrepareNumeral(&v.numeral, 0)
	}
	one, e := s.Manager.IsOne(&v.infinity)
	if e != nil {
		return 0, e
	}
	var n uint32
	if one {
		n, e = s.Manager.Z.Natural.appendDisplay(0, "oo")
	} else {
		minus, err := s.Manager.IsMinusOne(&v.infinity)
		if err != nil {
			return 0, err
		}
		if minus {
			n, e = s.Manager.Z.Natural.appendDisplay(0, "-oo")
		} else {
			n, e = s.infEpsInfPrepareRational(&v.infinity, 0)
			if e == nil {
				n, e = s.Manager.Z.Natural.appendDisplay(n, "*oo")
			}
		}
	}
	if e != nil {
		return 0, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return 0, e
	}
	firstZero, e := s.Manager.IsZero(&v.numeral.First)
	if e != nil {
		return 0, e
	}
	if firstZero {
		secondZero, e := s.Manager.IsZero(&v.numeral.Second)
		if e != nil {
			return 0, e
		}
		if secondZero {
			return n, nil
		}
	}
	// Source builds the infinity spelling before checking/printing the numeral.
	// Insert '(' while preserving that completed prefix in owner storage.
	end, e := checkedTextEnd(n, 1)
	if e != nil {
		return 0, e
	}
	if e = s.Manager.Z.Natural.reserveText(end); e != nil {
		return 0, e
	}
	for j := n; j > 0; j-- {
		s.Manager.Z.Natural.textWrite(j, s.Manager.Z.Natural.textRead(j-1))
	}
	s.Manager.Z.Natural.textWrite(0, '(')
	n = end
	n, e = s.Manager.Z.Natural.appendDisplay(n, " + ")
	if e != nil {
		return 0, e
	}
	n, e = s.infEpsInfPrepareNumeral(&v.numeral, n)
	if e != nil {
		return 0, e
	}
	return s.Manager.Z.Natural.appendDisplay(n, ")")
}
func (s *RationalService) InfEpsInfString(v *OwnedInfEpsInfRational) (string, error) {
	if e := s.lock(); e != nil {
		return "", e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return "", e
	}
	n, e := s.infEpsInfPrepare(v)
	if e != nil {
		return "", e
	}
	return s.Manager.Z.Natural.finalString(n)
}
func (s *RationalService) infEpsInfSign(v *OwnedInfEpsInfRational) int32 {
	if v.infinity.Num.Val != 0 {
		return v.infinity.Num.Val
	}
	return s.infSign(&v.numeral)
}
func (s *RationalService) infEpsInfEqual(a, b *OwnedInfEpsInfRational) (bool, error) {
	ok, e := s.Manager.Equal(&a.infinity, &b.infinity)
	if e != nil || !ok {
		return ok, e
	}
	if e := s.infScalarOwner(&a.numeral); e != nil {
		return false, e
	}
	if e := s.infScalarOwner(&b.numeral); e != nil {
		return false, e
	}
	ok, e = s.Manager.Equal(&a.numeral.First, &b.numeral.First)
	if e != nil || !ok {
		return ok, e
	}
	return s.Manager.Equal(&a.numeral.Second, &b.numeral.Second)
}
func (s *RationalService) infEpsInfLess(a, b *OwnedInfEpsInfRational) (bool, error) {
	n, e := s.Manager.Compare(&a.infinity, &b.infinity)
	if e != nil || n < 0 {
		return n < 0, e
	}
	eq, e := s.Manager.Equal(&a.infinity, &b.infinity)
	if e != nil || !eq {
		return false, e
	}
	return s.infLess(&a.numeral, &b.numeral)
}
func (s *RationalService) infEpsInfLessRational(a *OwnedInfEpsInfRational, b *OwnedRational, reverse bool) (bool, error) {
	i := a.infinity.Num.Val
	if reverse {
		if i > 0 {
			return true, nil
		}
		if i != 0 {
			return false, nil
		}
	} else {
		if i < 0 {
			return true, nil
		}
		if i != 0 {
			return false, nil
		}
	}
	// The source infinity-sign branches never inspect the rational operand.
	// The nested finite comparison validates it only after those branches.
	return s.infLessRational(&a.numeral, b, reverse)
}
func (s *RationalService) infEpsInfEqualRational(a *OwnedInfEpsInfRational, b *OwnedRational, reverse bool) (bool, error) {
	left, right := &a.infinity, &b.Raw
	if reverse {
		left, right = right, left
	}
	ok, e := s.Manager.Equal(left, right)
	if e != nil || !ok {
		return false, e
	}
	if e := s.infScalarOwner(&a.numeral); e != nil {
		return false, e
	}
	return ok && a.numeral.First.Num.Val == 0 && a.numeral.Second.Num.Val == 0, e
}
func (s *RationalService) infEpsInfCompound(a, b *OwnedInfEpsInfRational, sub bool) error {
	op := byte('+')
	if sub {
		op = '-'
	}
	if e := s.infRatCompound(&a.infinity, &b.infinity, op); e != nil {
		return e
	}
	return s.infCompound(&a.numeral, &b.numeral, sub)
}
func (s *RationalService) infEpsInfScalar(v *OwnedInfEpsInfRational, r *Rational, op byte) error {
	if op == '*' || op == '/' {
		if e := s.infRatCompound(&v.infinity, r, op); e != nil {
			return e
		}
	}
	return s.infScalar(&v.numeral, r, op)
}
func (s *RationalService) infEpsInfNeg(v *OwnedInfEpsInfRational) error {
	if e := s.Manager.Neg(&v.infinity); e != nil {
		return e
	}
	return s.infNeg(&v.numeral)
}
func (s *RationalService) InfEpsInfNeg(v *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	return s.infEpsInfNeg(v)
}
func (s *RationalService) infEpsInfUnary(v *OwnedInfEpsInfRational, abs bool) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	out, e := s.infEpsInfClone(v)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if !abs || s.infEpsInfSign(&out) < 0 {
		if e = s.infEpsInfNeg(&out); e != nil {
			s.infEpsInfDestroy(&out)
			return OwnedInfEpsInfRational{}, e
		}
	}
	return out, nil
}
func (s *RationalService) InfEpsInfUnaryMinus(v *OwnedInfEpsInfRational) (OwnedInfEpsInfRational, error) {
	return s.infEpsInfUnary(v, false)
}
func (s *RationalService) InfEpsInfAbs(v *OwnedInfEpsInfRational) (OwnedInfEpsInfRational, error) {
	return s.infEpsInfUnary(v, true)
}
func (s *RationalService) infEpsInfFused(v *OwnedInfEpsInfRational, c *OwnedRational, k *OwnedInfEpsInfRational, sub bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.check(c); e != nil {
		return e
	}
	if e := s.infEpsScalarOwner(k); e != nil {
		return e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return e
	}
	if e := s.infScalarOwner(&k.numeral); e != nil {
		return e
	}
	// addmul/submul updates m_infty before m_r. Let each component's
	// operation validate its own storage when reached, so a failure in the
	// nested epsilon term keeps the earlier source-visible updates.
	if e := s.infRatFused(&v.infinity, &c.Raw, &k.infinity, sub); e != nil {
		return e
	}
	if e := s.infRatFused(&v.numeral.First, &c.Raw, &k.numeral.First, sub); e != nil {
		return e
	}
	return s.infRatFused(&v.numeral.Second, &c.Raw, &k.numeral.Second, sub)
}
func (s *RationalService) InfEpsInfAddMulAssign(v *OwnedInfEpsInfRational, c *OwnedRational, k *OwnedInfEpsInfRational) error {
	return s.infEpsInfFused(v, c, k, false)
}
func (s *RationalService) InfEpsInfSubMulAssign(v *OwnedInfEpsInfRational, c *OwnedRational, k *OwnedInfEpsInfRational) error {
	return s.infEpsInfFused(v, c, k, true)
}
func (s *RationalService) infEpsInfRound(v *OwnedInfEpsInfRational, ceil bool) (_lexOut OwnedRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedRational{}, e
	}
	defer s.endLexicalRational(&_lexOut, &_lexErr)
	// The source floor/ceil wrappers ignore m_infty; their finite-domain
	// SASSERTs are commented out. Validate only the reached nested numeral.
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return OwnedRational{}, e
	}
	n := &v.numeral
	out := OwnedRational{Raw: NewRational(), ownerID: s.ownerID}
	var e error
	if isIntegerRational(&n.First) {
		adjust := n.Second.Num.Val < 0
		if ceil {
			adjust = n.Second.Num.Val > 0
		}
		if adjust {
			op := byte('-')
			if ceil {
				op = '+'
			}
			e = s.infRatExpr(&n.First, &s.OneField, &out.Raw, op)
		} else {
			e = s.Manager.Copy(&out.Raw, &n.First)
		}
	} else if ceil {
		e = s.Manager.Ceil(&n.First, &out.Raw)
	} else {
		e = s.Manager.Floor(&n.First, &out.Raw)
	}
	if e != nil {
		if isIntegerRational(&n.First) {
			s.abortLexicalRational(&out.Raw)
		} else {
			s.destroyLexicalRational(&out.Raw)
		}
		return OwnedRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfEpsInfFloor(v *OwnedInfEpsInfRational) (OwnedRational, error) {
	return s.infEpsInfRound(v, false)
}
func (s *RationalService) InfEpsInfCeil(v *OwnedInfEpsInfRational) (OwnedRational, error) {
	return s.infEpsInfRound(v, true)
}
func (s *RationalService) InfEpsInfGetInfinity(v *OwnedInfEpsInfRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.infinity)
}
func (s *RationalService) InfEpsInfGetRational(v *OwnedInfEpsInfRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.numeral.First)
}
func (s *RationalService) InfEpsInfGetInfinitesimal(v *OwnedInfEpsInfRational) (OwnedRational, error) {
	if e := s.lock(); e != nil {
		return OwnedRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedRational{}, e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return OwnedRational{}, e
	}
	return s.copyOutput(&v.numeral.Second)
}
func (s *RationalService) InfEpsInfIsFinite(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	return v.infinity.Num.Val == 0, nil
}

// Source nested predicates short-circuit infinity before the numeral owner.
func (s *RationalService) infEpsScalarOwner(v *OwnedInfEpsInfRational) error {
	if v == nil {
		return ErrRationalServiceOwner
	}
	return s.scalarProjectionOwner(v.ownerID)
}
func (s *RationalService) infEpsScalarFits(v *OwnedInfEpsInfRational, unsigned bool) (bool, error) {
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	return s.infScalarFits(&v.numeral, unsigned)
}
func (s *RationalService) InfEpsInfIsRational(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	return v.numeral.Second.Num.Val == 0, nil
}
func (s *RationalService) InfEpsInfIsInt(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsInt(&v.numeral.First)
	return ok && v.numeral.Second.Num.Val == 0, e
}
func (s *RationalService) InfEpsInfIsInt64(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	return s.infEpsScalarFits(v, false)
}
func (s *RationalService) InfEpsInfIsUint64(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	return s.infEpsScalarFits(v, true)
}
func (s *RationalService) InfEpsInfIsZero(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	return v.numeral.First.Num.Val == 0 && v.numeral.Second.Num.Val == 0, nil
}
func (s *RationalService) InfEpsInfIsOne(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsOne(&v.numeral.First)
	return ok && v.numeral.Second.Num.Val == 0, e
}
func (s *RationalService) InfEpsInfIsMinusOne(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val != 0 {
		return false, nil
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return false, e
	}
	ok, e := s.Manager.IsMinusOne(&v.numeral.First)
	return ok && v.numeral.Second.Num.Val == 0, e
}
func (s *RationalService) InfEpsInfIsPos(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val == 0 {
		if e := s.infScalarOwner(&v.numeral); e != nil {
			return false, e
		}
	}
	return s.infEpsInfSign(v) > 0, nil
}
func (s *RationalService) InfEpsInfIsNeg(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val == 0 {
		if e := s.infScalarOwner(&v.numeral); e != nil {
			return false, e
		}
	}
	return s.infEpsInfSign(v) < 0, nil
}
func (s *RationalService) InfEpsInfIsNonPos(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val == 0 {
		if e := s.infScalarOwner(&v.numeral); e != nil {
			return false, e
		}
	}
	return s.infEpsInfSign(v) <= 0, nil
}
func (s *RationalService) InfEpsInfIsNonNeg(v *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return false, e
	}
	if v.infinity.Num.Val == 0 {
		if e := s.infScalarOwner(&v.numeral); e != nil {
			return false, e
		}
	}
	return s.infEpsInfSign(v) >= 0, nil
}
func (s *RationalService) InfEpsInfInt64(v *OwnedInfEpsInfRational) (int64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return 0, e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infEpsScalarFits(v, false)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_eps_rational.h", 122, "is_int64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return 0, e
	}
	return s.infSignedProjectionLocked(&v.numeral)
}
func (s *RationalService) InfEpsInfUint64(v *OwnedInfEpsInfRational) (uint64, error) {
	if e := s.lock(); e != nil {
		return 0, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return 0, e
	}
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return 0, e
		}
		if enabled {
			ok, e := s.infEpsScalarFits(v, true)
			if e != nil {
				return 0, e
			}
			if !ok {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/inf_eps_rational.h", 127, "is_uint64()"); e != nil {
					return 0, e
				}
			}
		}
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return 0, e
	}
	return s.infUnsignedProjectionLocked(&v.numeral)
}
func (s *RationalService) InfEpsInfEqual(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqual(a, b)
	return ok, e
}
func (s *RationalService) InfEpsInfNotEqual(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqual(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfLess(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLess(a, b)
	return ok, e
}
func (s *RationalService) InfEpsInfGreater(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLess(b, a)
	return ok, e
}
func (s *RationalService) InfEpsInfLessEqual(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLess(b, a)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfGreaterEqual(a, b *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLess(a, b)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfEqualRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqualRational(a, b, false)
	return ok, e
}
func (s *RationalService) InfEpsInfNotEqualRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqualRational(a, b, false)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfLessRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, false)
	return ok, e
}
func (s *RationalService) InfEpsInfGreaterRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, true)
	return ok, e
}
func (s *RationalService) InfEpsInfLessEqualRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, true)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfGreaterEqualRational(a *OwnedInfEpsInfRational, b *OwnedRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, false)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfRationalEqual(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqualRational(a, b, true)
	return ok, e
}
func (s *RationalService) InfEpsInfRationalNotEqual(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	if e := s.rationalScalarOwner(b); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfEqualRational(a, b, true)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfRationalLess(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, true)
	return ok, e
}
func (s *RationalService) InfEpsInfRationalGreater(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, false)
	return ok, e
}
func (s *RationalService) InfEpsInfRationalLessEqual(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, false)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfRationalGreaterEqual(b *OwnedRational, a *OwnedInfEpsInfRational) (bool, error) {
	if e := s.lock(); e != nil {
		return false, e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return false, e
	}
	ok, e := s.infEpsInfLessRational(a, b, true)
	if e != nil {
		return false, e
	}
	return !ok, nil
}
func (s *RationalService) InfEpsInfAddAssign(a, b *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return e
	}
	return s.infEpsInfCompound(a, b, false)
}
func (s *RationalService) InfEpsInfAdd(a, b *OwnedInfEpsInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsScalarOwner(a); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	work, e := s.infEpsInfClone(a)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.infEpsInfDestroy(&work)
	if e = s.infEpsInfCompound(&work, b, false); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	return s.infEpsInfClone(&work)
}
func (s *RationalService) InfEpsInfAddInfAssign(v *OwnedInfEpsInfRational, n *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(n); e != nil {
		return e
	}
	return s.infCompound(&v.numeral, n, false)
}
func (s *RationalService) InfEpsInfSubAssign(a, b *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(a); e != nil {
		return e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return e
	}
	return s.infEpsInfCompound(a, b, true)
}
func (s *RationalService) InfEpsInfSub(a, b *OwnedInfEpsInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsScalarOwner(a); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.infEpsScalarOwner(b); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	work, e := s.infEpsInfClone(a)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.infEpsInfDestroy(&work)
	if e = s.infEpsInfCompound(&work, b, true); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	return s.infEpsInfClone(&work)
}
func (s *RationalService) InfEpsInfSubInfAssign(v *OwnedInfEpsInfRational, n *OwnedInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(n); e != nil {
		return e
	}
	return s.infCompound(&v.numeral, n, true)
}
func (s *RationalService) InfEpsInfAddRationalAssign(v *OwnedInfEpsInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infEpsInfScalar(v, &r.Raw, '+')
}
func (s *RationalService) InfEpsInfSubRationalAssign(v *OwnedInfEpsInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infEpsInfScalar(v, &r.Raw, '-')
}
func (s *RationalService) InfEpsInfMulRationalAssign(v *OwnedInfEpsInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infEpsInfScalar(v, &r.Raw, '*')
}
func (s *RationalService) InfEpsInfMulRational(v *OwnedInfEpsInfRational, r *OwnedRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	out, e := s.infEpsInfClone(v)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e = s.infEpsInfScalar(&out, &r.Raw, '*'); e != nil {
		s.infEpsInfDestroy(&out)
		return OwnedInfEpsInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfEpsInfDivRationalAssign(v *OwnedInfEpsInfRational, r *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return e
	}
	return s.infEpsInfScalar(v, &r.Raw, '/')
}
func (s *RationalService) InfEpsInfDivRational(v *OwnedInfEpsInfRational, r *OwnedRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsScalarOwner(v); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e := s.rationalScalarOwner(r); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	out, e := s.infEpsInfClone(v)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e = s.infEpsInfScalar(&out, &r.Raw, '/'); e != nil {
		s.infEpsInfDestroy(&out)
		return OwnedInfEpsInfRational{}, e
	}
	return out, nil
}
func (s *RationalService) InfEpsInfInc(v *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return e
	}
	return s.infRatStep(&v.numeral.First, false)
}
func (s *RationalService) InfEpsInfPostInc(v *OwnedInfEpsInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsInfCheck(v); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	old, e := s.infEpsInfClone(v)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e = s.infRatStep(&v.numeral.First, false); e != nil {
		s.infEpsInfDestroy(&old)
		return OwnedInfEpsInfRational{}, e
	}
	return old, nil
}
func (s *RationalService) InfEpsInfDec(v *OwnedInfEpsInfRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e := s.infEpsScalarOwner(v); e != nil {
		return e
	}
	if e := s.infScalarOwner(&v.numeral); e != nil {
		return e
	}
	return s.infRatStep(&v.numeral.First, true)
}
func (s *RationalService) InfEpsInfPostDec(v *OwnedInfEpsInfRational) (_lexOut OwnedInfEpsInfRational, _lexErr error) {
	if e := s.lock(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.mu.Unlock()
	if e := s.beginLexical(); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	defer s.endLexicalEps(&_lexOut, &_lexErr)
	if e := s.infEpsInfCheck(v); e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	old, e := s.infEpsInfClone(v)
	if e != nil {
		return OwnedInfEpsInfRational{}, e
	}
	if e = s.infRatStep(&v.numeral.First, true); e != nil {
		s.infEpsInfDestroy(&old)
		return OwnedInfEpsInfRational{}, e
	}
	return old, nil
}
