package numeral

// Source: src/util/mpfx.cpp:251-315,704-743,758-787,842-854.

import (
	"io"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

// SetFromInteger is the reached subpaving mpz -> mpfx conversion. The source
// decomposes the integer into its configured integer-word part.
func (m *FixedPointManager) SetFromInteger(dst *FixedPoint, integers *IntegerManager, src *Int) error {
	if err := m.check(dst); err != nil {
		return err
	}
	if integers == nil {
		return ErrFixedPointOwner
	}
	text, err := integers.String(src)
	if err != nil {
		return err
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return ErrFixedPointDomain
	}
	n.Mul(n, &m.scale)
	if err := m.publish(dst, n); err != nil {
		m.publishSignBeforeWords(dst, n.Sign() < 0)
		return err
	}
	return nil
}

// SetFromRational follows the source's two conversion branches. For a
// noninteger input, mpfx.cpp:285 assigns the destination's existing sign to
// the new result before rounding. The input numerator's sign is then removed.
func (m *FixedPointManager) SetFromRational(dst *FixedPoint, rationals *RationalManager, src *Rational) error {
	if err := m.check(dst); err != nil {
		return err
	}
	if rationals == nil {
		return ErrFixedPointOwner
	}
	text, err := rationals.String(src)
	if err != nil {
		return err
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return ErrFixedPointDomain
	}
	if q.IsInt() {
		var scaled big.Int
		scaled.Mul(q.Num(), &m.scale)
		if err := m.publish(dst, &scaled); err != nil {
			m.publishSignBeforeWords(dst, scaled.Sign() < 0)
			return err
		}
		return nil
	}
	negative := dst.sourceNegative()
	var numerator big.Int
	numerator.Abs(q.Num())
	numerator.Mul(&numerator, &m.scale)
	if negative {
		numerator.Neg(&numerator)
	}
	rounded, err := m.roundMagnitude(&numerator, q.Denom())
	if err != nil {
		return err
	}
	if err := m.publish(dst, rounded); err != nil {
		m.publishSignBeforeWords(dst, negative)
		return err
	}
	if rounded.Sign() == 0 {
		// The noninteger source branch allocates n before division and
		// copies the zero quotient words without resetting its slot.
		dst.sourceZeroSlot = true
		dst.sourceZeroNegative = negative
	}
	return nil
}

// ToInteger is the reached exactness check after integer-to-fixed conversion.
func (m *FixedPointManager) ToInteger(src *FixedPoint, integers *IntegerManager, dst *Int) error {
	if err := m.check(src); err != nil {
		return err
	}
	if integers == nil {
		return ErrFixedPointOwner
	}
	var whole, rem big.Int
	whole.QuoRem(&src.scaled, &m.scale, &rem)
	if rem.Sign() != 0 {
		return ErrFixedPointDomain
	}
	return integers.SetString(dst, whole.String())
}

// ToRational copies the exact scaled value into a caller-owned mpq numeral.
func (m *FixedPointManager) ToRational(src *FixedPoint, rationals *RationalManager, dst *Rational) error {
	r, err := m.Rational(src)
	if err != nil {
		return err
	}
	if rationals == nil {
		return ErrFixedPointOwner
	}
	return rationals.SetString(dst, r.RatString())
}

// String follows mpfx_manager::display: finite binary fractions use the
// shortest numerator and a power-of-two denominator.
func (m *FixedPointManager) String(a *FixedPoint) (string, error) {
	r, err := m.Rational(a)
	if err != nil {
		return "", err
	}
	if a.sourceZeroSlot && a.sourceZeroNegative {
		return "-0", nil
	}
	if r.IsInt() {
		return r.Num().String(), nil
	}
	k := r.Denom().BitLen() - 1
	suffix := "/2"
	if k > 1 {
		suffix += "^" + strconv.Itoa(k)
	}
	return r.Num().String() + suffix, nil
}

func (m *FixedPointManager) ToRationalString(a *FixedPoint) (string, error) {
	return m.String(a)
}

func (m *FixedPointManager) Display(w io.Writer, a *FixedPoint) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := m.check(a); err != nil {
		return err
	}
	if a.sourceNegative() {
		if err := writeFixedPointPart(w, "-"); err != nil {
			return err
		}
	}
	r, err := m.Rational(a)
	if err != nil {
		return err
	}
	num := r.Num()
	num.Abs(num)
	if err := writeFixedPointPart(w, num.String()); err != nil {
		return err
	}
	if r.IsInt() {
		return nil
	}
	if err := writeFixedPointPart(w, "/2"); err != nil {
		return err
	}
	k := r.Denom().BitLen() - 1
	if k <= 1 {
		return nil
	}
	if err := writeFixedPointPart(w, "^"); err != nil {
		return err
	}
	return writeFixedPointPart(w, strconv.Itoa(k))
}

func writeFixedPointPart(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}
