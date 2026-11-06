package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:254-281,636-650,1848-1861;
// concrete config_mpfx constraint and leaf display.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fixedWrite(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}

func (c *FixedContext) displayRelation(w io.Writer, variable uint32, number *numeral.FixedPoint, lower, open bool) error {
	if lower {
		value, err := c.numbers.ToRationalString(number)
		if err != nil {
			return err
		}
		if err := fixedWrite(w, value); err != nil {
			return err
		}
		if err := fixedWrite(w, " <"); err != nil {
			return err
		}
		if !open {
			if err := fixedWrite(w, "="); err != nil {
				return err
			}
		}
		if err := fixedWrite(w, " "); err != nil {
			return err
		}
		return c.writeVariable(w, variable)
	}
	if err := c.writeVariable(w, variable); err != nil {
		return err
	}
	if err := fixedWrite(w, " <"); err != nil {
		return err
	}
	if !open {
		if err := fixedWrite(w, "="); err != nil {
			return err
		}
	}
	if err := fixedWrite(w, " "); err != nil {
		return err
	}
	value, err := c.numbers.ToRationalString(number)
	if err != nil {
		return err
	}
	return fixedWrite(w, value)
}

func (c *FixedContext) displayBound(w io.Writer, b *FixedBound) error {
	return c.displayRelation(w, b.variable, b.value, b.lower, b.open)
}

func (c *FixedContext) displayInequality(w io.Writer, a *FixedInequality) error {
	return c.displayRelation(w, a.variable, a.value, a.lower, a.open)
}

func (c *FixedContext) displayDefinition(w io.Writer, variable uint32, useStar bool) error {
	if m := c.monomials[variable]; m != nil {
		for i, power := range m.powers {
			if i != 0 {
				separator := " "
				if useStar {
					separator = "*"
				}
				if err := fixedWrite(w, separator); err != nil {
					return err
				}
			}
			if err := c.writeVariable(w, power.Variable); err != nil {
				return err
			}
			if power.Degree > 1 {
				if err := fixedWrite(w, "^"+strconv.FormatUint(uint64(power.Degree), 10)); err != nil {
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
	zero, err := c.numbers.IsZero(p.constant)
	if err != nil {
		return err
	}
	if !zero {
		value, err := c.numbers.ToRationalString(p.constant)
		if err != nil {
			return err
		}
		if err := fixedWrite(w, value); err != nil {
			return err
		}
		first = false
	}
	for _, term := range p.terms {
		if !first {
			if err := fixedWrite(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one, err := c.numbers.IsOne(term.Coefficient)
		if err != nil {
			return err
		}
		if !one {
			value, err := c.numbers.ToRationalString(term.Coefficient)
			if err != nil {
				return err
			}
			if err := fixedWrite(w, value); err != nil {
				return err
			}
			separator := " "
			if useStar {
				separator = "*"
			}
			if err := fixedWrite(w, separator); err != nil {
				return err
			}
		}
		if err := c.writeVariable(w, term.Variable); err != nil {
			return err
		}
	}
	return nil
}

// DisplayConstraints emits definitions, axiom units, and clauses in source
// order. The variable printer is borrowed from the shell when installed.
func (c *FixedContext) DisplayConstraints(w io.Writer, useStar bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	for variable := range c.isInt {
		x := uint32(variable)
		if c.monomials[x] == nil && c.polynomials[x] == nil {
			continue
		}
		if err := c.writeVariable(w, x); err != nil {
			return err
		}
		if err := fixedWrite(w, " = "); err != nil {
			return err
		}
		if err := c.displayDefinition(w, x, useStar); err != nil {
			return err
		}
		if err := fixedWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, unit := range c.unitClauses {
		if err := c.displayInequality(w, unit.atom); err != nil {
			return err
		}
		if err := fixedWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, clause := range c.clauses {
		for i, atom := range clause.atoms {
			if i != 0 {
				if err := fixedWrite(w, " or "); err != nil {
					return err
				}
			}
			if err := c.displayInequality(w, atom); err != nil {
				return err
			}
		}
		if err := fixedWrite(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *FixedContext) displayNodeBounds(w io.Writer, n *FixedNode) error {
	for variable := range c.isInt {
		lower, upper := n.lower[variable], n.upper[variable]
		if lower != nil {
			if err := c.displayBound(w, lower); err != nil {
				return err
			}
			if err := fixedWrite(w, " "); err != nil {
				return err
			}
		}
		if upper != nil {
			if err := c.displayBound(w, upper); err != nil {
				return err
			}
		}
		if lower != nil || upper != nil {
			if err := fixedWrite(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayBounds visits consistent leaves in source creation order. Closed
// caller-created children are omitted from the current tree.
func (c *FixedContext) DisplayBounds(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := c.root.open(); err != nil {
		return err
	}
	children := make(map[*FixedNode][]*FixedNode, len(c.nodes))
	for _, n := range c.nodes {
		if !n.closed && n.parent != nil {
			children[n.parent] = append(children[n.parent], n)
		}
	}
	todo := []*FixedNode{c.root}
	first := true
	for len(todo) != 0 {
		last := len(todo) - 1
		n := todo[last]
		todo = todo[:last]
		if descendants := children[n]; len(descendants) != 0 {
			for i := len(descendants) - 1; i >= 0; i-- {
				todo = append(todo, descendants[i])
			}
			continue
		}
		if n.inconsistent {
			continue
		}
		if !first {
			if err := fixedWrite(w, "=========\n"); err != nil {
				return err
			}
		}
		first = false
		if err := c.displayNodeBounds(w, n); err != nil {
			return err
		}
	}
	return nil
}
