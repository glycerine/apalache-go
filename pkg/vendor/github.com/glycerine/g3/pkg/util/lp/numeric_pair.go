package lp

// Source: src/util/lp/numeric_pair.h. Concrete numeric_pair<mpq> used by LAR.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalPair represents X + Y*epsilon, ordered lexicographically. Its fields
// are owned numerals from one caller-owned manager. Initialize with
// NewRationalPair, release with Del, and use Copy instead of shallow copying.
// The manager must outlive every pair. Coordinate operations run X before Y;
// an error can leave a completed X prefix, which remains owned by the caller.
type RationalPair struct {
	X, Y numeral.Rational
}

func NewRationalPair() RationalPair {
	return RationalPair{X: numeral.NewRational(), Y: numeral.NewRational()}
}

func (p *RationalPair) Copy(m *numeral.RationalManager, a *RationalPair) error {
	if err := m.Copy(&p.X, &a.X); err != nil {
		return err
	}
	return m.Copy(&p.Y, &a.Y)
}

// Del releases fields in reverse declaration order. After success the pair
// contains initialized zeros and may be reused with the same manager.
func (p *RationalPair) Del(m *numeral.RationalManager) error {
	if err := m.Del(&p.Y); err != nil {
		return err
	}
	if err := m.Del(&p.X); err != nil {
		return err
	}
	*p = NewRationalPair()
	return nil
}

func (p *RationalPair) Add(m *numeral.RationalManager, a, b *RationalPair) error {
	if err := m.Add(&a.X, &b.X, &p.X); err != nil {
		return err
	}
	return m.Add(&a.Y, &b.Y, &p.Y)
}
func (p *RationalPair) Sub(m *numeral.RationalManager, a, b *RationalPair) error {
	if err := m.Sub(&a.X, &b.X, &p.X); err != nil {
		return err
	}
	return m.Sub(&a.Y, &b.Y, &p.Y)
}

// Scale and Divide permit p == a. As in source compound operators, a scalar
// borrowed from p.X is reread by the Y operation after X has been updated.
func (p *RationalPair) Scale(m *numeral.RationalManager, a *RationalPair, scalar *numeral.Rational) error {
	if err := m.Mul(&a.X, scalar, &p.X); err != nil {
		return err
	}
	return m.Mul(&a.Y, scalar, &p.Y)
}
func (p *RationalPair) Divide(m *numeral.RationalManager, a *RationalPair, scalar *numeral.Rational) error {
	if err := m.Div(&a.X, scalar, &p.X); err != nil {
		return err
	}
	return m.Div(&a.Y, scalar, &p.Y)
}
func (p *RationalPair) Negate(m *numeral.RationalManager) error {
	if err := m.Neg(&p.X); err != nil {
		return err
	}
	return m.Neg(&p.Y)
}
func (p *RationalPair) Compare(m *numeral.RationalManager, a *RationalPair) (int, error) {
	c, err := m.Compare(&p.X, &a.X)
	if err != nil || c != 0 {
		return c, err
	}
	return m.Compare(&p.Y, &a.Y)
}
func (p *RationalPair) Equal(m *numeral.RationalManager, a *RationalPair) (bool, error) {
	eq, err := m.Equal(&p.X, &a.X)
	if err != nil || !eq {
		return false, err
	}
	return m.Equal(&p.Y, &a.Y)
}

// Sign returns the first nonzero source numerator sign. Only its relation to
// zero is meaningful: the numeral manager retains inline magnitudes.
func (p *RationalPair) Sign(m *numeral.RationalManager) (int32, error) {
	x, err := m.Sign(&p.X)
	if err != nil || x != 0 {
		return x, err
	}
	return m.Sign(&p.Y)
}
func (p *RationalPair) IsZero(m *numeral.RationalManager) (bool, error) {
	zero, err := m.IsZero(&p.X)
	if err != nil || !zero {
		return false, err
	}
	return m.IsZero(&p.Y)
}

// Float64 is numeric_traits<numeric_pair<mpq>>::get_double: only X contributes.
func (p *RationalPair) Float64(m *numeral.RationalManager) (float64, error) {
	return m.Float64(&p.X)
}
func (p *RationalPair) String(m *numeral.RationalManager) (string, error) {
	x, err := m.String(&p.X)
	if err != nil {
		return "", err
	}
	y, err := m.String(&p.Y)
	if err != nil {
		return "", err
	}
	return "(" + x + ", " + y + ")", nil
}

// Less and Greater preserve the source scalar-comparison order and structural
// X equality gate. Numerically equal unreduced raw X values do not advance to Y.
// Greater's rational comparisons reverse operands, as rational::operator> does.
func (p *RationalPair) Less(m *numeral.RationalManager, a *RationalPair) (bool, error) {
	c, err := m.Compare(&p.X, &a.X)
	if err != nil || c < 0 {
		return c < 0, err
	}
	equal, err := m.Equal(&p.X, &a.X)
	if err != nil || !equal {
		return false, err
	}
	c, err = m.Compare(&p.Y, &a.Y)
	return c < 0, err
}
func (p *RationalPair) Greater(m *numeral.RationalManager, a *RationalPair) (bool, error) {
	c, err := m.Compare(&a.X, &p.X)
	if err != nil || c < 0 {
		return c < 0, err
	}
	equal, err := m.Equal(&p.X, &a.X)
	if err != nil || !equal {
		return false, err
	}
	c, err = m.Compare(&a.Y, &p.Y)
	return c < 0, err
}
