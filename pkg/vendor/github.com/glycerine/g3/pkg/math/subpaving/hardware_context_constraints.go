package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:666-687,741-780,
// concrete config_hwf variable, polynomial, and inequality construction.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// HardwarePolynomialTerm borrows its coefficient during DefinePolynomial.
type HardwarePolynomialTerm struct {
	Variable    uint32
	Coefficient *numeral.HardwareFloat
}

type hardwarePolynomialTerm struct {
	variable    uint32
	coefficient numeral.HardwareFloat
}

// HardwarePolynomial owns scalar copies of its constant and coefficients.
type HardwarePolynomial struct {
	constant numeral.HardwareFloat
	terms    []hardwarePolynomialTerm
	visited  uint64
}

// HardwareInequality owns a scalar copy of the directed bound.
type HardwareInequality struct {
	context  *HardwareContext
	variable uint32
	value    numeral.HardwareFloat
	lower    bool
	open     bool
	refs     uint32
}

// NewVariable appends a source mk_var entry before constraint construction.
func (c *HardwareContext) NewVariable(integer bool) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || len(c.isInt) >= math.MaxUint32 {
		return 0, arena.ErrInvalidSlot
	}
	x := uint32(len(c.isInt))
	c.isInt = append(c.isInt, integer)
	c.watchers = append(c.watchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	return x, nil
}

// DefinePolynomial follows mk_sum's variable-indexed coefficient buffer.
// A repeated variable takes its last input coefficient at its first sorted
// occurrence; later occurrences take the zero left by the first swap.
func (c *HardwareContext) DefinePolynomial(constant *numeral.HardwareFloat, terms []HardwarePolynomialTerm) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || len(c.isInt) >= math.MaxUint32 {
		return 0, arena.ErrInvalidSlot
	}
	m := c.numbers
	p := &HardwarePolynomial{terms: make([]hardwarePolynomialTerm, len(terms))}
	if err := m.Set(&p.constant, constant); err != nil {
		return 0, err
	}
	last := make(map[uint32]numeral.HardwareFloat, len(terms))
	variables := make([]uint32, len(terms))
	for i, term := range terms {
		if term.Variable >= uint32(len(c.isInt)) {
			return 0, arena.ErrInvalidSlot
		}
		var coefficient numeral.HardwareFloat
		if err := m.Set(&coefficient, term.Coefficient); err != nil {
			return 0, err
		}
		last[term.Variable] = coefficient
		variables[i] = term.Variable
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i] < variables[j] })
	used := make(map[uint32]bool, len(last))
	for i, variable := range variables {
		p.terms[i].variable = variable
		if !used[variable] {
			p.terms[i].coefficient = last[variable]
			used[variable] = true
		}
	}
	if err := c.tracePolynomialInteger(p); err != nil {
		return 0, err
	}
	integer, err := c.polynomialInteger(p)
	if err != nil {
		return 0, err
	}
	x := uint32(len(c.isInt))
	c.isInt = append(c.isInt, integer)
	c.watchers = append(c.watchers, nil)
	c.watchOrder = append(c.watchOrder, nil)
	for _, term := range p.terms {
		c.watchers[term.variable] = append(c.watchers[term.variable], x)
		c.watchOrder[term.variable] = append(c.watchOrder[term.variable], hardwareWatch{definition: x})
	}
	if c.polynomials == nil {
		c.polynomials = make(map[uint32]*HardwarePolynomial)
	}
	c.polynomials[x] = p
	return x, nil
}

// NewInequality copies the rounded scalar and records the source flags.
func (c *HardwareContext) NewInequality(variable uint32, value *numeral.HardwareFloat, lower, open bool) (*HardwareInequality, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if variable >= uint32(len(c.isInt)) {
		return nil, arena.ErrInvalidSlot
	}
	a := &HardwareInequality{context: c, variable: variable, lower: lower, open: open}
	if err := c.numbers.Set(&a.value, value); err != nil {
		return nil, err
	}
	c.inequalities = append(c.inequalities, a)
	return a, nil
}
