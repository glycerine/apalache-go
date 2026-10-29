package polynomial

// Source: src/util/ref_buffer.h:32-108;
// src/math/polynomial/polynomial.cpp:3895-4015.

// GCD's monomial slice owns one manager reference per entry. The cleanup
// cursor records completed decrements if an actual monomial destructor stops.
type polynomialGCDMonomialBuffer struct {
	values                       []Monomial
	cursor                       uint32
	dropping, constructed, ended bool
	sourceCause                  error
}

func (b *polynomialGCDMonomialBuffer) construct() {
	*b = polynomialGCDMonomialBuffer{constructed: true}
}
func (b *polynomialGCDMonomialBuffer) get(i uint32) (Monomial, error) {
	if uint64(i) >= uint64(len(b.values)) {
		if polynomialSourceDebug {
			return Monomial{}, ErrPolynomialAssertion
		}
		return Monomial{}, ErrPolynomialOwner
	}
	return b.values[i], nil
}
func (b *polynomialGCDMonomialBuffer) push(c *MultivariatePolynomialCore, a Monomial) error {
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if uint64(len(b.values)) == uint64(^uint32(0)) {
		return ErrPolynomialOwner
	}
	if e := c.monomials.IncRef(a); e != nil {
		return e
	}
	if c.generatedActive && c.generatedMonomial == a {
		c.transferGeneratedMonomial()
	}
	b.values = append(b.values, a)
	return nil
}
func (b *polynomialGCDMonomialBuffer) reset(c *MultivariatePolynomialCore, owning bool) error {
	if b.sourceCause != nil && !owning {
		return b.sourceCause
	}
	for uint64(b.cursor) < uint64(len(b.values)) {
		r := polynomialDivisionReference{cause: b.sourceCause}
		if e := r.drop(c, &b.values[b.cursor], &b.dropping, owning); e != nil {
			b.sourceCause = e
			return e
		}
		b.cursor++
	}
	b.values = b.values[:0]
	b.cursor = 0
	return nil
}
func (b *polynomialGCDMonomialBuffer) close(c *MultivariatePolynomialCore, owning bool) error {
	if !b.constructed || b.ended {
		return nil
	}
	if e := b.reset(c, owning); e != nil {
		return e
	}
	b.values = nil
	b.ended = true
	return nil
}
