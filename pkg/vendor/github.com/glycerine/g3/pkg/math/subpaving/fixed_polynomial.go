package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:669-683,741-768,1408-1496,
// concrete config_mpfx polynomial definitions and propagation.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FixedPolynomialTerm borrows its coefficient while DefinePolynomial runs.
type FixedPolynomialTerm struct {
	Variable    uint32
	Coefficient *numeral.FixedPoint
}

type FixedPolynomial struct {
	constant *numeral.FixedPoint
	terms    []FixedPolynomialTerm
	visited  uint64
}

func (p *FixedPolynomial) close() (err error) {
	for _, term := range p.terms {
		err = errors.Join(err, term.Coefficient.Close())
	}
	if p.constant != nil {
		err = errors.Join(err, p.constant.Close())
	}
	return err
}

// DefinePolynomial creates one output variable. Definitions are installed
// before child nodes exist, as in the source's initialization sequence.
func (c *FixedContext) DefinePolynomial(constant *numeral.FixedPoint, terms []FixedPolynomialTerm) (_ uint32, err error) {
	if err = c.open(); err != nil {
		return 0, err
	}
	if len(terms) == 0 || len(c.isInt) >= math.MaxUint32 || c.rootInitStarted || c.root.children != 0 {
		return 0, arena.ErrInvalidSlot
	}
	m := c.numbers
	if _, err = m.IsInt(constant); err != nil {
		return 0, err
	}
	last := make(map[uint32]*numeral.FixedPoint, len(terms))
	variables := make([]uint32, len(terms))
	for i, term := range terms {
		if term.Variable >= uint32(len(c.isInt)) {
			return 0, arena.ErrInvalidSlot
		}
		if _, err = m.IsInt(term.Coefficient); err != nil {
			return 0, err
		}
		last[term.Variable] = term.Coefficient
		variables[i] = term.Variable
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i] < variables[j] })
	p := &FixedPolynomial{}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.close())
		}
	}()
	if p.constant, err = m.New(); err != nil {
		return 0, err
	}
	if err = m.Copy(p.constant, constant); err != nil {
		return 0, err
	}
	used := make(map[uint32]bool, len(last))
	for _, variable := range variables {
		coefficient, e := m.New()
		if e != nil {
			return 0, e
		}
		p.terms = append(p.terms, FixedPolynomialTerm{Variable: variable, Coefficient: coefficient})
		// Source mk_sum swaps the variable's numeral buffer with each sorted
		// term. Duplicate variables receive the last input coefficient once;
		// subsequent copies receive the buffer's reset zero.
		if !used[variable] {
			if err = m.Copy(coefficient, last[variable]); err != nil {
				return 0, err
			}
			used[variable] = true
		}
	}
	if err = c.tracePolynomialInteger(p); err != nil {
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
	c.root.lower = append(c.root.lower, nil)
	c.root.upper = append(c.root.upper, nil)
	for _, term := range p.terms {
		c.watchers[term.Variable] = append(c.watchers[term.Variable], x)
		c.watchOrder[term.Variable] = append(c.watchOrder[term.Variable], fixedWatch{definition: x})
	}
	c.polynomials[x] = p
	return x, nil
}

// PropagatePolynomial follows the source's single-unbounded-variable rule.
func (c *FixedContext) PropagatePolynomial(variable uint32, n *FixedNode) error {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if n.inconsistent {
		return arena.ErrInvalidSlot
	}
	p := c.polynomials[variable]
	if p == nil {
		return arena.ErrInvalidSlot
	}
	if err := c.tracePolynomialVariable("propagate_polynomial", variable, true, "propagate_polynomial: ", 1467); err != nil {
		return err
	}
	if err := c.tracePolynomialBounds(n); err != nil {
		return err
	}
	p.visited = c.timestamp
	unbounded := uint32(math.MaxUint32)
	if n.lower[variable] == nil && n.upper[variable] == nil {
		unbounded = variable
	}
	for _, term := range p.terms {
		x := term.Variable
		if n.lower[x] == nil && n.upper[x] == nil {
			if unbounded != math.MaxUint32 {
				return nil
			}
			unbounded = x
		}
	}
	if err := c.tracePolynomialVariable("propagate_polynomial", unbounded, unbounded != math.MaxUint32, "unbounded_var: ", 1484); err != nil {
		return err
	}
	if unbounded != math.MaxUint32 {
		return c.propagatePolynomialTarget(p, variable, unbounded, n)
	}
	if err := c.propagatePolynomialTarget(p, variable, variable, n); err != nil {
		return err
	}
	for _, term := range p.terms {
		if n.inconsistent {
			return nil
		}
		if err := c.propagatePolynomialTarget(p, variable, term.Variable, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *FixedContext) propagatePolynomialTarget(p *FixedPolynomial, variable, target uint32, n *FixedNode) (err error) {
	m := c.numbers
	r, err := NewFixedInterval(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	v, err := NewFixedInterval(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	av, err := NewFixedInterval(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, av.Close()) }()
	if target == variable {
		for i, term := range p.terms {
			if err = v.SetConstant(n, term.Variable); err != nil {
				return err
			}
			if err = av.MulScalar(term.Coefficient, v); err != nil {
				return err
			}
			if i == 0 {
				err = r.SetFrom(av)
			} else {
				err = r.Add(r, av)
			}
			if err != nil {
				return err
			}
		}
	} else {
		if err = v.SetConstant(n, variable); err != nil {
			return err
		}
		if err = r.SetFrom(v); err != nil {
			return err
		}
		var coefficient *numeral.FixedPoint
		for _, term := range p.terms {
			if term.Variable == target {
				coefficient = term.Coefficient
				if err = c.tracePolynomialCoefficient(coefficient); err != nil {
					return err
				}
				continue
			}
			if err = v.SetConstant(n, term.Variable); err != nil {
				return err
			}
			if err = av.MulScalar(term.Coefficient, v); err != nil {
				return err
			}
			if err = r.Sub(r, av); err != nil {
				return err
			}
		}
		if coefficient == nil {
			return arena.ErrInvalidSlot
		}
		if err = c.tracePolynomialInterval(r, 1444, "r before mul 1/a: "); err != nil {
			return err
		}
		if err = r.DivScalar(r, coefficient); err != nil {
			return err
		}
		if err = c.tracePolynomialInterval(r, 1446, "r after mul 1/a:  "); err != nil {
			return err
		}
	}
	// The retained C++ propagation path stores p.m_c but omits it here.
	if !r.lowerInf {
		if _, err = c.propagateCandidateReason(n, target, r.lower, true, r.lowerOpen, fixedBoundReason{kind: fixedReasonVariableDefinition, variable: variable}); err != nil {
			return err
		}
		if n.inconsistent {
			return nil
		}
	}
	if !r.upperInf {
		_, err = c.propagateCandidateReason(n, target, r.upper, false, r.upperOpen, fixedBoundReason{kind: fixedReasonVariableDefinition, variable: variable})
	}
	return err
}
