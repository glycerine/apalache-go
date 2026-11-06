package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:668-685,741-769,
// concrete config_mpq polynomial definitions and watches.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type rationalWatch struct {
	definition uint32
	clause     *RationalClause
}

type rationalPolynomialTerm struct {
	variable    uint32
	coefficient numeral.Rational
}

// RationalPolynomial owns its constant and each coefficient. The definition
// variable and its term variables are borrowed indices in the context.
type RationalPolynomial struct {
	context    *RationalContext
	variable   uint32
	constant   numeral.Rational
	terms      []rationalPolynomialTerm
	visited    uint64
	closeIndex int
	closed     bool
}

func (p *RationalPolynomial) Close() error {
	if p == nil || p.context == nil {
		return arena.ErrInvalidSlot
	}
	if p.closed {
		return nil
	}
	for p.closeIndex < len(p.terms) {
		if err := p.context.numbers.Del(&p.terms[p.closeIndex].coefficient); err != nil {
			return err
		}
		p.closeIndex++
	}
	if err := p.context.numbers.Del(&p.constant); err != nil {
		return err
	}
	p.closed = true
	return nil
}

// DefinePolynomial follows mk_sum's variable-indexed temporary buffer: the
// last input coefficient for a repeated variable occupies its first sorted
// term, while later occurrences receive the default zero numeral.
func (c *RationalContext) DefinePolynomial(constant *numeral.Rational, coefficients []numeral.Rational, variables []uint32) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || constant == nil || len(coefficients) == 0 || len(coefficients) != len(variables) || len(c.variables) >= math.MaxUint32 {
		return 0, arena.ErrInvalidSlot
	}
	for _, variable := range variables {
		if variable >= uint32(len(c.variables)) {
			return 0, arena.ErrInvalidSlot
		}
	}
	for len(c.numBuffer) < len(c.variables) {
		c.numBuffer = append(c.numBuffer, numeral.NewRational())
	}
	for i, variable := range variables {
		if err := c.numbers.Copy(&c.numBuffer[variable], &coefficients[i]); err != nil {
			return 0, err
		}
	}
	p := &RationalPolynomial{context: c, constant: numeral.NewRational(), terms: make([]rationalPolynomialTerm, len(variables))}
	for i := range p.terms {
		p.terms[i].coefficient = numeral.NewRational()
	}
	c.polynomialOwners = append(c.polynomialOwners, p)
	if err := c.numbers.Copy(&p.constant, constant); err != nil {
		return 0, err
	}
	sorted := append([]uint32(nil), variables...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	for i, variable := range sorted {
		p.terms[i].variable = variable
		if err := c.numbers.Swap(&c.numBuffer[variable], &p.terms[i].coefficient); err != nil {
			return 0, err
		}
	}
	if err := c.tracePolynomialInteger(p); err != nil {
		return 0, err
	}
	integer, err := c.polynomialInteger(p)
	if err != nil {
		return 0, err
	}
	x, err := c.NewVariable(integer)
	if err != nil {
		return 0, err
	}
	p.variable = x
	if c.polynomials == nil {
		c.polynomials = make(map[uint32]*RationalPolynomial)
	}
	c.polynomials[x] = p
	for _, term := range p.terms {
		c.watchOrder[term.variable] = append(c.watchOrder[term.variable], rationalWatch{definition: x})
	}
	return x, nil
}
