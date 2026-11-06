package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:254-281,636-650,1848-1861;
// concrete config_mpff constraint display.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastVariableDisplay is borrowed from the caller. Nil restores x0, x1,
// ... spelling, as in the source context's default display procedure.
type FastVariableDisplay func(io.Writer, uint32) error

func fastWrite(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}

func (c *FastContext) SetVariableDisplay(display FastVariableDisplay) error {
	if err := c.open(); err != nil {
		return err
	}
	c.displayVariable = display
	return nil
}

func (c *FastContext) writeVariable(w io.Writer, variable uint32) error {
	if c.displayVariable != nil {
		return c.displayVariable(w, variable)
	}
	return fastWrite(w, "x"+strconv.FormatUint(uint64(variable), 10))
}

func (c *FastContext) displayRelation(w io.Writer, variable uint32, number *numeral.FastFloat, lower, open bool) error {
	if lower {
		value, err := c.numbers.String(number)
		if err != nil {
			return err
		}
		if err := fastWrite(w, value); err != nil {
			return err
		}
		if err := fastWrite(w, " <"); err != nil {
			return err
		}
		if !open {
			if err := fastWrite(w, "="); err != nil {
				return err
			}
		}
		if err := fastWrite(w, " "); err != nil {
			return err
		}
		return c.writeVariable(w, variable)
	}
	if err := c.writeVariable(w, variable); err != nil {
		return err
	}
	if err := fastWrite(w, " <"); err != nil {
		return err
	}
	if !open {
		if err := fastWrite(w, "="); err != nil {
			return err
		}
	}
	if err := fastWrite(w, " "); err != nil {
		return err
	}
	value, err := c.numbers.String(number)
	if err != nil {
		return err
	}
	return fastWrite(w, value)
}

func (c *FastContext) displayBound(w io.Writer, b *FastBound) error {
	return c.displayRelation(w, b.variable, b.value, b.lower, b.open)
}

func (c *FastContext) displayInequality(w io.Writer, a *FastInequality) error {
	return c.displayRelation(w, a.variable, a.value, a.lower, a.open)
}

func (c *FastContext) displayDefinition(w io.Writer, variable uint32, useStar bool) error {
	if m := c.monomials[variable]; m != nil {
		for i, power := range m.powers {
			if i != 0 {
				separator := " "
				if useStar {
					separator = "*"
				}
				if err := fastWrite(w, separator); err != nil {
					return err
				}
			}
			if err := c.writeVariable(w, power.Variable); err != nil {
				return err
			}
			if power.Degree > 1 {
				if err := fastWrite(w, "^"+strconv.FormatUint(uint64(power.Degree), 10)); err != nil {
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
		value, err := c.numbers.String(p.constant)
		if err != nil {
			return err
		}
		if err := fastWrite(w, value); err != nil {
			return err
		}
		first = false
	}
	for _, term := range p.terms {
		if !first {
			if err := fastWrite(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one, err := c.numbers.IsOne(term.Coefficient)
		if err != nil {
			return err
		}
		if !one {
			value, err := c.numbers.String(term.Coefficient)
			if err != nil {
				return err
			}
			if err := fastWrite(w, value); err != nil {
				return err
			}
			separator := " "
			if useStar {
				separator = "*"
			}
			if err := fastWrite(w, separator); err != nil {
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
func (c *FastContext) DisplayConstraints(w io.Writer, useStar bool) error {
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
		if err := fastWrite(w, " = "); err != nil {
			return err
		}
		if err := c.displayDefinition(w, x, useStar); err != nil {
			return err
		}
		if err := fastWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, unit := range c.unitClauses {
		if err := c.displayInequality(w, unit.atom); err != nil {
			return err
		}
		if err := fastWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, clause := range c.clauses {
		for i, atom := range clause.atoms {
			if i != 0 {
				if err := fastWrite(w, " or "); err != nil {
					return err
				}
			}
			if err := c.displayInequality(w, atom); err != nil {
				return err
			}
		}
		if err := fastWrite(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *FastContext) displayNodeBounds(w io.Writer, n *FastNode) error {
	for variable := range c.isInt {
		lower, upper := n.lower[variable], n.upper[variable]
		if lower != nil {
			if err := c.displayBound(w, lower); err != nil {
				return err
			}
			if err := fastWrite(w, " "); err != nil {
				return err
			}
		}
		if upper != nil {
			if err := c.displayBound(w, upper); err != nil {
				return err
			}
		}
		if lower != nil || upper != nil {
			if err := fastWrite(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayBounds visits consistent leaves in source creation order. Children
// remain caller-owned; closed children are omitted from the current tree.
func (c *FastContext) DisplayBounds(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := c.root.open(); err != nil {
		return err
	}
	children := make(map[*FastNode][]*FastNode, len(c.nodes))
	for _, n := range c.nodes {
		if !n.closed && n.parent != nil {
			children[n.parent] = append(children[n.parent], n)
		}
	}
	todo := []*FastNode{c.root}
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
			if err := fastWrite(w, "=========\n"); err != nil {
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
