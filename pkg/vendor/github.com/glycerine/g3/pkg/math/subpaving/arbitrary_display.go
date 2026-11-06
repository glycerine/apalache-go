package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:254-281,636-650,1848-1861;
// concrete config_mpf bound and leaf display.

import (
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ArbitraryVariableDisplay is borrowed from the caller. Nil restores x0, x1,
// ... spelling, as in the source context's default display procedure.
type ArbitraryVariableDisplay func(io.Writer, uint32) error

func arbitraryWrite(w io.Writer, part string) error {
	n, err := io.WriteString(w, part)
	if err != nil {
		return err
	}
	if n != len(part) {
		return io.ErrShortWrite
	}
	return nil
}

func (c *ArbitraryContext) SetVariableDisplay(display ArbitraryVariableDisplay) error {
	if err := c.open(); err != nil {
		return err
	}
	c.displayVariable = display
	return nil
}

func (c *ArbitraryContext) writeVariable(w io.Writer, variable uint32) error {
	if c.displayVariable != nil {
		return c.displayVariable(w, variable)
	}
	return arbitraryWrite(w, "x"+strconv.FormatUint(uint64(variable), 10))
}

func (c *ArbitraryContext) displayRelation(w io.Writer, variable uint32, number *numeral.ArbitraryFloat, lower, open bool) error {
	value, err := c.numbers.ToRationalString(number)
	if err != nil {
		return err
	}
	if lower {
		if err := arbitraryWrite(w, value); err != nil {
			return err
		}
		if err := arbitraryWrite(w, " <"); err != nil {
			return err
		}
		if !open {
			if err := arbitraryWrite(w, "="); err != nil {
				return err
			}
		}
		if err := arbitraryWrite(w, " "); err != nil {
			return err
		}
		return c.writeVariable(w, variable)
	}
	if err := c.writeVariable(w, variable); err != nil {
		return err
	}
	if err := arbitraryWrite(w, " <"); err != nil {
		return err
	}
	if !open {
		if err := arbitraryWrite(w, "="); err != nil {
			return err
		}
	}
	if err := arbitraryWrite(w, " "); err != nil {
		return err
	}
	return arbitraryWrite(w, value)
}

func (c *ArbitraryContext) displayBound(w io.Writer, b *ArbitraryBound) error {
	return c.displayRelation(w, b.variable, &b.value, b.lower, b.open)
}

func (c *ArbitraryContext) displayInequality(w io.Writer, a *ArbitraryInequality) error {
	return c.displayRelation(w, a.variable, &a.value, a.lower, a.open)
}

func (c *ArbitraryContext) displayDefinition(w io.Writer, variable uint32, useStar bool) error {
	if m := c.monomials[variable]; m != nil {
		for i, power := range m.powers {
			if i != 0 {
				separator := " "
				if useStar {
					separator = "*"
				}
				if err := arbitraryWrite(w, separator); err != nil {
					return err
				}
			}
			if err := c.writeVariable(w, power.Variable); err != nil {
				return err
			}
			if power.Degree > 1 {
				if err := arbitraryWrite(w, "^"+strconv.FormatUint(uint64(power.Degree), 10)); err != nil {
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
		value, err := c.numbers.ToRationalString(&p.constant)
		if err != nil {
			return err
		}
		if err := arbitraryWrite(w, value); err != nil {
			return err
		}
		first = false
	}
	for _, term := range p.terms {
		if !first {
			if err := arbitraryWrite(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one, err := c.numbers.IsOne(&term.coefficient)
		if err != nil {
			return err
		}
		if !one {
			value, err := c.numbers.ToRationalString(&term.coefficient)
			if err != nil {
				return err
			}
			if err := arbitraryWrite(w, value); err != nil {
				return err
			}
			separator := " "
			if useStar {
				separator = "*"
			}
			if err := arbitraryWrite(w, separator); err != nil {
				return err
			}
		}
		if err := c.writeVariable(w, term.variable); err != nil {
			return err
		}
	}
	return nil
}

// DisplayConstraints emits definitions, axiom units, and clauses in source
// order. The variable printer is borrowed from the shell when installed.
func (c *ArbitraryContext) DisplayConstraints(w io.Writer, useStar bool) error {
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
		if err := arbitraryWrite(w, " = "); err != nil {
			return err
		}
		if err := c.displayDefinition(w, x, useStar); err != nil {
			return err
		}
		if err := arbitraryWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, unit := range c.unitClauses {
		if err := c.displayInequality(w, unit.atom); err != nil {
			return err
		}
		if err := arbitraryWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, clause := range c.clauses {
		for i, atom := range clause.atoms {
			if i != 0 {
				if err := arbitraryWrite(w, " or "); err != nil {
					return err
				}
			}
			if err := c.displayInequality(w, atom); err != nil {
				return err
			}
		}
		if err := arbitraryWrite(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *ArbitraryContext) displayNodeBounds(w io.Writer, n *ArbitraryNode) error {
	for variable := range c.variables {
		lower, upper := n.lower[variable], n.upper[variable]
		if lower != nil {
			if err := c.displayBound(w, lower); err != nil {
				return err
			}
			if err := arbitraryWrite(w, " "); err != nil {
				return err
			}
		}
		if upper != nil {
			if err := c.displayBound(w, upper); err != nil {
				return err
			}
		}
		if lower != nil || upper != nil {
			if err := arbitraryWrite(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayBounds visits consistent leaves in source tree order, including a
// selected leaf temporarily removed from the search queue.
func (c *ArbitraryContext) DisplayBounds(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if c.root == nil {
		return nil
	}
	children := make(map[*ArbitraryNode][]*ArbitraryNode, len(c.nodes))
	for _, n := range c.nodes {
		if !n.closed && n.parent != nil {
			children[n.parent] = append(children[n.parent], n)
		}
	}
	first := true
	var visit func(*ArbitraryNode) error
	visit = func(n *ArbitraryNode) error {
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
			if err := arbitraryWrite(w, "=========\n"); err != nil {
				return err
			}
		}
		first = false
		return c.displayNodeBounds(w, n)
	}
	return visit(c.root)
}
