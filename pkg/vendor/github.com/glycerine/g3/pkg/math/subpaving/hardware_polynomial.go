package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1408-1496, concrete
// config_hwf polynomial propagation over node-bound interval views.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// PropagatePolynomial applies the retained single-unbounded-variable rule.
// The source's stored polynomial constant is intentionally absent from this
// calculation; it is retained in the definition record.
func (c *HardwareContext) PropagatePolynomial(variable uint32, n *HardwareNode) error {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if n.conflictVar != math.MaxUint32 {
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
		return c.propagatePolynomialTarget(p, variable, unbounded, n)
	}
	if err := c.propagatePolynomialTarget(p, variable, variable, n); err != nil {
		return err
	}
	for _, term := range p.terms {
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if err := c.propagatePolynomialTarget(p, variable, term.variable, n); err != nil {
			return err
		}
	}
	return nil
}

func (c *HardwareContext) propagatePolynomialTarget(p *HardwarePolynomial, variable, target uint32, n *HardwareNode) (err error) {
	r, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	v, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	av, err := NewHardwareInterval(c.numbers)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, av.Close()) }()
	if target == variable {
		if len(p.terms) == 0 {
			return nil
		}
		for i, term := range p.terms {
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
		if err = v.SetConstant(n, variable); err != nil {
			return err
		}
		if err = r.SetFrom(v); err != nil {
			return err
		}
		var coefficient *hardwarePolynomialTerm
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
	lower, upper, err := r.endpoints()
	if err != nil {
		return err
	}
	if !r.lowerInf {
		if _, err = c.propagateCandidateReason(n, target, lower, true, r.lowerOpen, hardwareReasonVariableDefinition, variable); err != nil {
			return err
		}
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
	}
	if !r.upperInf {
		_, err = c.propagateCandidateReason(n, target, upper, false, r.upperOpen, hardwareReasonVariableDefinition, variable)
	}
	return err
}
