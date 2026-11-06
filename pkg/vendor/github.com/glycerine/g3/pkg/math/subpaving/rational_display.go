package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:254-282,376-416,636-650,
// 1135-1155,1848-1861; concrete config_mpq constraint and leaf display.

import (
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalVariableDisplay is borrowed from the caller. Nil restores the
// source context's x0, x1, ... default spelling.
type RationalVariableDisplay func(io.Writer, uint32) error

func rationalWrite(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}

func (c *RationalContext) SetVariableDisplay(display RationalVariableDisplay) error {
	if err := c.open(); err != nil {
		return err
	}
	c.displayVariable = display
	return nil
}

func (c *RationalContext) writeVariable(w io.Writer, variable uint32) error {
	if c.displayVariable != nil {
		return c.displayVariable(w, variable)
	}
	return rationalWrite(w, "x"+strconv.FormatUint(uint64(variable), 10))
}

func (c *RationalContext) displayRelation(w io.Writer, variable uint32, number *numeral.Rational, lower, open bool) error {
	value, err := c.numbers.String(number)
	if err != nil {
		return err
	}
	if lower {
		if err := rationalWrite(w, value); err != nil {
			return err
		}
		if err := rationalWrite(w, " <"); err != nil {
			return err
		}
		if !open {
			if err := rationalWrite(w, "="); err != nil {
				return err
			}
		}
		if err := rationalWrite(w, " "); err != nil {
			return err
		}
		return c.writeVariable(w, variable)
	}
	if err := c.writeVariable(w, variable); err != nil {
		return err
	}
	if err := rationalWrite(w, " <"); err != nil {
		return err
	}
	if !open {
		if err := rationalWrite(w, "="); err != nil {
			return err
		}
	}
	if err := rationalWrite(w, " "); err != nil {
		return err
	}
	return rationalWrite(w, value)
}

func (c *RationalContext) displayBound(w io.Writer, b *RationalBound) error {
	return c.displayRelation(w, b.variable, &b.value, b.lower, b.open)
}

func (c *RationalContext) displayInequality(w io.Writer, a *RationalInequality) error {
	return c.displayRelation(w, a.variable, &a.value, a.lower, a.open)
}

func (c *RationalContext) displayDefinition(w io.Writer, variable uint32, useStar bool) error {
	if m := c.monomials[variable]; m != nil {
		for i, power := range m.powers {
			if i != 0 {
				separator := " "
				if useStar {
					separator = "*"
				}
				if err := rationalWrite(w, separator); err != nil {
					return err
				}
			}
			if err := c.writeVariable(w, power.Variable); err != nil {
				return err
			}
			if power.Degree > 1 {
				if err := rationalWrite(w, "^"+strconv.FormatUint(uint64(power.Degree), 10)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	p := c.polynomials[variable]
	if p == nil {
		return arena.ErrInvalidSlot
	}
	first := true
	zero, err := c.numbers.IsZero(&p.constant)
	if err != nil {
		return err
	}
	if !zero {
		value, err := c.numbers.String(&p.constant)
		if err != nil {
			return err
		}
		if err := rationalWrite(w, value); err != nil {
			return err
		}
		first = false
	}
	for _, term := range p.terms {
		if !first {
			if err := rationalWrite(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one, err := c.numbers.IsOne(&term.coefficient)
		if err != nil {
			return err
		}
		if !one {
			value, err := c.numbers.String(&term.coefficient)
			if err != nil {
				return err
			}
			if err := rationalWrite(w, value); err != nil {
				return err
			}
			separator := " "
			if useStar {
				separator = "*"
			}
			if err := rationalWrite(w, separator); err != nil {
				return err
			}
		}
		if err := c.writeVariable(w, term.variable); err != nil {
			return err
		}
	}
	return nil
}

// DisplayConstraints emits definitions, units and clauses in source order.
func (c *RationalContext) DisplayConstraints(w io.Writer, useStar bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	for variable := range c.variables {
		x := uint32(variable)
		if c.monomials[x] == nil && c.polynomials[x] == nil {
			continue
		}
		if err := c.writeVariable(w, x); err != nil {
			return err
		}
		if err := rationalWrite(w, " = "); err != nil {
			return err
		}
		if err := c.displayDefinition(w, x, useStar); err != nil {
			return err
		}
		if err := rationalWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, unit := range c.unitClauses {
		if err := c.displayInequality(w, unit.atom); err != nil {
			return err
		}
		if err := rationalWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, clause := range c.clauses {
		for i, atom := range clause.atoms {
			if i != 0 {
				if err := rationalWrite(w, " or "); err != nil {
					return err
				}
			}
			if err := c.displayInequality(w, atom); err != nil {
				return err
			}
		}
		if err := rationalWrite(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *RationalContext) displayNodeBounds(w io.Writer, n *RationalNode) error {
	for variable := range c.variables {
		lower, upper := n.lower[variable], n.upper[variable]
		if lower != nil {
			if err := c.displayBound(w, lower); err != nil {
				return err
			}
			if err := rationalWrite(w, " "); err != nil {
				return err
			}
		}
		if upper != nil {
			if err := c.displayBound(w, upper); err != nil {
				return err
			}
		}
		if lower != nil || upper != nil {
			if err := rationalWrite(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayBounds visits consistent leaves in source tree order.
func (c *RationalContext) DisplayBounds(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if c.root == nil {
		return nil
	}
	children := make(map[*RationalNode][]*RationalNode, len(c.nodes))
	for _, n := range c.nodes {
		if !n.closed && n.parent != nil {
			children[n.parent] = append(children[n.parent], n)
		}
	}
	first := true
	var visit func(*RationalNode) error
	visit = func(n *RationalNode) error {
		if descendants := children[n]; len(descendants) != 0 {
			for _, child := range descendants {
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		}
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if !first {
			if err := rationalWrite(w, "=========\n"); err != nil {
				return err
			}
		}
		first = false
		return c.displayNodeBounds(w, n)
	}
	return visit(c.root)
}
