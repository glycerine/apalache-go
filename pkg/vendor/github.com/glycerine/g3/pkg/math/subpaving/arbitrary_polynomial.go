package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:668-685,741-769,
// concrete config_mpf polynomial definitions.

import (
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type arbitraryPolynomialTerm struct {
	variable    uint32
	coefficient numeral.ArbitraryFloat
}

// ArbitraryPolynomial owns the copied constant and sorted term numerals.
type ArbitraryPolynomial struct {
	context    *ArbitraryContext
	variable   uint32
	constant   numeral.ArbitraryFloat
	terms      []arbitraryPolynomialTerm
	visited    uint64
	closeIndex int
}

// DefinePolynomial follows context_t<config_mpf>::mk_sum. The scratch buffer
// is indexed by variable, so a repeated variable retains its last input
// coefficient and later sorted occurrences receive the default numeral.
func (c *ArbitraryContext) DefinePolynomial(constant *numeral.ArbitraryFloat, coefficients []numeral.ArbitraryFloat, variables []uint32) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || constant == nil || len(coefficients) != len(variables) {
		return 0, arena.ErrInvalidSlot
	}
	for _, x := range variables {
		if x >= uint32(len(c.variables)) {
			return 0, arena.ErrInvalidSlot
		}
	}
	if len(c.numBuffer) < len(c.variables) {
		c.numBuffer = append(c.numBuffer, make([]numeral.ArbitraryFloat, len(c.variables)-len(c.numBuffer))...)
	}
	for i, x := range variables {
		if err := c.numbers.Copy(&c.numBuffer[x], &coefficients[i]); err != nil {
			return 0, err
		}
	}
	p := &ArbitraryPolynomial{context: c, terms: make([]arbitraryPolynomialTerm, len(variables))}
	c.polynomialOwners = append(c.polynomialOwners, p)
	if err := c.numbers.Copy(&p.constant, constant); err != nil {
		return 0, err
	}
	sorted := append([]uint32(nil), variables...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	for i, x := range sorted {
		p.terms[i].variable = x
		if err := c.numbers.Swap(&c.numBuffer[x], &p.terms[i].coefficient); err != nil {
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
		c.polynomials = make(map[uint32]*ArbitraryPolynomial)
	}
	c.polynomials[x] = p
	for _, term := range p.terms {
		c.watchers[term.variable] = append(c.watchers[term.variable], x)
		c.watchOrder[term.variable] = append(c.watchOrder[term.variable], arbitraryWatch{definition: x})
	}
	return x, nil
}
