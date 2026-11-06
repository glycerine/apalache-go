package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1408-1496, concrete config_mpq
// polynomial forward/reverse propagation over live node-bound intervals.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// PropagatePolynomial uses the source's single-unbounded-variable rule. The
// stored constant is intentionally absent from the source interval sum.
func (c *RationalContext) PropagatePolynomial(variable uint32, n *RationalNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered || variable >= uint32(len(c.variables)) || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	p := c.polynomials[variable]
	if p == nil || p.closed {
		return arena.ErrInvalidSlot
	}
	if err := c.tracePolynomialVariable("propagate_polynomial", variable, true, "propagate_polynomial: ", 1467); err != nil {
		return err
	}
	if err := c.tracePolynomialBounds(n); err != nil {
		return err
	}
	c.numVisited++
	p.visited = c.timestamp
	unbounded := uint32(math.MaxUint32)
	if n.lower[variable] == nil && n.upper[variable] == nil {
		unbounded = variable
	}
	for _, term := range p.terms {
		x := term.variable
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
		return c.propagatePolynomialTarget(p, unbounded, n)
	}
	if err := c.propagatePolynomialTarget(p, variable, n); err != nil {
		return err
	}
	for _, term := range p.terms {
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if err := c.propagatePolynomialTarget(p, term.variable, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *RationalContext) propagatePolynomialTarget(p *RationalPolynomial, target uint32, n *RationalNode) (err error) {
	r, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	v, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	av, err := NewRationalInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, av.Close()) }()
	if target == p.variable {
		for i := range p.terms {
			term := &p.terms[i]
			if err = v.SetConstant(n, term.variable); err != nil {
				return err
			}
			if err = av.MulScalar(&term.coefficient, v); err != nil {
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
		if err = v.SetConstant(n, p.variable); err != nil {
			return err
		}
		if err = r.SetFrom(v); err != nil {
			return err
		}
		var coefficient *rationalPolynomialTerm
		for i := range p.terms {
			term := &p.terms[i]
			if term.variable == target {
				coefficient = term
				if err = c.tracePolynomialCoefficient(&term.coefficient); err != nil {
					return err
				}
				continue
			}
			if err = v.SetConstant(n, term.variable); err != nil {
				return err
			}
			if err = av.MulScalar(&term.coefficient, v); err != nil {
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
		if err = r.DivScalar(r, &coefficient.coefficient); err != nil {
			return err
		}
		if err = c.tracePolynomialInterval(r, 1446, "r after mul 1/a:  "); err != nil {
			return err
		}
	}
	lower, upper, err := r.resolved()
	if err != nil {
		return err
	}
	if !r.lowerInf {
		if err := c.propagateRationalDefinitionCandidate(n, target, p.variable, lower, true, r.lowerOpen); err != nil {
			return err
		}
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
	}
	if !r.upperInf {
		return c.propagateRationalDefinitionCandidate(n, target, p.variable, upper, false, r.upperOpen)
	}
	return nil
}

func (c *RationalContext) propagateRationalDefinitionCandidate(n *RationalNode, target, definition uint32, value *numeral.Rational, lower, open bool) (err error) {
	adjusted := numeral.NewRational()
	defer func() { err = errors.Join(err, c.numbers.Del(&adjusted)) }()
	if c.variables[target] {
		integer, e := c.numbers.IsInt(value)
		if e != nil {
			return e
		}
		if !integer {
			open = false
		}
		if lower {
			err = c.numbers.Ceil(value, &adjusted)
		} else {
			err = c.numbers.Floor(value, &adjusted)
		}
		if err != nil {
			return err
		}
		if open {
			open = false
			if lower {
				err = c.numbers.Inc(&adjusted)
			} else {
				err = c.numbers.Dec(&adjusted)
			}
			if err != nil {
				return err
			}
		}
	} else if err = c.numbers.Copy(&adjusted, value); err != nil {
		return err
	}
	relevant, err := c.RelevantNewBound(n, target, &adjusted, lower, open)
	if err != nil || !relevant {
		return err
	}
	if c.timestamp == math.MaxUint64 {
		return ErrRationalContextTimestamp
	}
	c.numBounds++
	if err := c.publishNormalized(n, target, &adjusted, lower, open, false, nil, definition); err != nil {
		return err
	}
	c.queue = append(c.queue, n.trail)
	return nil
}
