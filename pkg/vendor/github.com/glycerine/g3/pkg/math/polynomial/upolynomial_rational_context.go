package polynomial

// Source: math/polynomial/sexpr2upolynomial.cpp; util scoped-numeral lifetimes.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func NewUPolynomialManagerFromRationalService(s *unumeral.RationalService, c UPolynomialConfig) (out *UPolynomialManager, cause error) {
	cause = s.WithNumericBorrow(func(b *unumeral.RationalNumericBorrow) error {
		if e := b.Ready(); e != nil {
			return e
		}
		if c.Integer != nil && c.Integer != b.Integers() {
			return ErrUPolynomialOwner
		}
		c.Integer = b.Integers()
		var e error
		out, e = NewUPolynomialManager(c)
		return e
	})
	return out, cause
}

// The polynomial context belongs to its actual caller, borrowing primitive
// service access only during the locked callback.
type UPolynomialRationalContext struct {
	inputReceipt *upolyRationalInputInvocationReceipt
	self         *UPolynomialRationalContext
	borrow       *unumeral.RationalNumericBorrow
	polynomial   *UPolynomialManager
	active       bool
}

func numericRationalService_WithUPolynomialRationals(s *unumeral.RationalService, m *UPolynomialManager, body func(*UPolynomialRationalContext) error) error {
	return numericRationalService_WithUPolynomialRationalsReceipt(s, m, nil, body)
}
func numericRationalService_WithUPolynomialRationalsReceipt(s *unumeral.RationalService, m *UPolynomialManager, receipt *unumeral.RationalNumericLexicalReceipt, body func(*UPolynomialRationalContext) error) error {
	return s.WithNumericBorrow(func(b *unumeral.RationalNumericBorrow) error {
		if m == nil || m.config.Integer == nil {
			return ErrUPolynomialOwner
		}
		if e := m.check(); e != nil {
			return e
		}
		if body == nil {
			return arena.ErrConfig
		}
		return b.WithLexicalReceipt(receipt, func() error {
			c := &m.rationalContext
			if c.active {
				return ErrUPolynomialCleanupPending
			}
			*c = UPolynomialRationalContext{self: c, borrow: b, polynomial: m, active: true}
			defer func() { *c = UPolynomialRationalContext{} }()
			return body(c)
		})
	})
}
func (c *UPolynomialRationalContext) valid() bool {
	return c != nil && c.self == c && c.active && c.polynomial != nil && c == &c.polynomial.rationalContext && c.borrow.Valid()
}
func (c *UPolynomialRationalContext) Pending() bool { return c.valid() && c.borrow.Pending() }
func (c *UPolynomialRationalContext) StopVectors(e error) {
	if c.valid() {
		c.borrow.StopVectors(e)
	}
}
func (c *UPolynomialRationalContext) SetConstant(p UPolynomial, a *unumeral.OwnedRational, nonInteger func() error) error {
	if !c.valid() {
		return unumeral.ErrRationalServiceOwner
	}
	// WithCopiedScalar preserves source owner validation before the callback
	// precondition and the reached lexical constructor.
	if nonInteger == nil {
		return c.borrow.WithCopiedScalar(a, nil)
	}
	return c.borrow.WithCopiedScalar(a, func(value unumeral.Rational) error {
		integer, e := c.borrow.IsInt(&value)
		if e != nil {
			return e
		}
		if !integer {
			return nonInteger()
		}
		return c.borrow.WithReceivingInteger(c.polynomial.config.Integer, func() error {
			return c.polynomial.setRationalsFromOwnerWithReceipt(p, c.borrow.Integers(), []unumeral.Rational{value}, c.inputReceipt)
		})
	})
}
func (c *UPolynomialRationalContext) Exponent(a *unumeral.OwnedRational) (uint32, bool, error) {
	if !c.valid() {
		return 0, false, unumeral.ErrRationalServiceOwner
	}
	return c.borrow.Exponent(a)
}
func (c *UPolynomialRationalContext) SetIdentity(p UPolynomial) error {
	if !c.valid() {
		return unumeral.ErrRationalServiceOwner
	}
	return c.borrow.WithReceivingInteger(c.polynomial.config.Integer, func() error {
		return c.polynomial.setRationalsFromOwnerWithReceipt(p, c.polynomial.config.Integer, []unumeral.Rational{unumeral.NewRational(), unumeral.RationalFromBorrowedNumerals(unumeral.NewInt(1), unumeral.NewInt(1))}, c.inputReceipt)
	})
}
func numericRationalService_CleanupUPolynomialRationals(s *unumeral.RationalService, m *UPolynomialManager, vectors func() error) error {
	return numericRationalService_CleanupUPolynomialRationalsReceipt(s, m, nil, vectors)
}
func numericRationalService_CleanupUPolynomialRationalsReceipt(s *unumeral.RationalService, m *UPolynomialManager, receipt *unumeral.RationalNumericLexicalReceipt, vectors func() error) error {
	return s.WithNumericBorrow(func(b *unumeral.RationalNumericBorrow) error {
		return b.WithReceivingInteger(m.config.Integer, func() error { return b.CleanupAfterVectorsReceipt(receipt, vectors) })
	})
}
