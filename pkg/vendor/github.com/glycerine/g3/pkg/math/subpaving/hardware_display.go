package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:254-287,376-415,
// 614-650,1067-1084,1135-1156,1848-1861; concrete config_hwf.

import (
	"io"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// HardwareVariableDisplay is the concrete shell's variable printer. A nil
// printer uses the source default x0, x1, ... spelling.
type HardwareVariableDisplay func(io.Writer, uint32) error

// SetVariableDisplay borrows the tactic's printer for subsequent display
// calls. A nil printer restores the context's default variable spelling.
func (c *HardwareContext) SetVariableDisplay(display HardwareVariableDisplay) error {
	if err := c.open(); err != nil {
		return err
	}
	c.displayVariable = display
	return nil
}

func hardwareWrite(w io.Writer, value string) error {
	n, err := io.WriteString(w, value)
	if err != nil {
		return err
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func hardwareWriteVariable(w io.Writer, variable uint32, display HardwareVariableDisplay) error {
	if display != nil {
		return display(w, variable)
	}
	return hardwareWrite(w, "x"+strconv.FormatUint(uint64(variable), 10))
}

func (c *HardwareContext) displayInequality(w io.Writer, variable uint32, value *numeral.HardwareFloat, lower, open bool, display HardwareVariableDisplay) error {
	text, err := c.numbers.ToRationalString(value)
	if err != nil {
		return err
	}
	operator := " < "
	if !open {
		operator = " <= "
	}
	if lower {
		if err := hardwareWrite(w, text+operator); err != nil {
			return err
		}
		return hardwareWriteVariable(w, variable, display)
	}
	if err := hardwareWriteVariable(w, variable, display); err != nil {
		return err
	}
	return hardwareWrite(w, operator+text)
}

func (c *HardwareContext) displayDefinition(w io.Writer, variable uint32, useStar bool, display HardwareVariableDisplay) error {
	if m := c.monomials[variable]; m != nil {
		for i, power := range m.powers {
			if i != 0 {
				separator := " "
				if useStar {
					separator = "*"
				}
				if err := hardwareWrite(w, separator); err != nil {
					return err
				}
			}
			if err := hardwareWriteVariable(w, power.Variable, display); err != nil {
				return err
			}
			if power.Degree > 1 {
				if err := hardwareWrite(w, "^"+strconv.FormatUint(uint64(power.Degree), 10)); err != nil {
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
	if !c.numbers.IsZero(&p.constant) {
		value, err := c.numbers.ToRationalString(&p.constant)
		if err != nil {
			return err
		}
		if err := hardwareWrite(w, value); err != nil {
			return err
		}
		first = false
	}
	for _, term := range p.terms {
		if !first {
			if err := hardwareWrite(w, " + "); err != nil {
				return err
			}
		}
		first = false
		if !c.numbers.IsOne(&term.coefficient) {
			value, err := c.numbers.ToRationalString(&term.coefficient)
			if err != nil {
				return err
			}
			separator := " "
			if useStar {
				separator = "*"
			}
			if err := hardwareWrite(w, value+separator); err != nil {
				return err
			}
		}
		if err := hardwareWriteVariable(w, term.variable, display); err != nil {
			return err
		}
	}
	return nil
}

// DisplayConstraints emits definitions, axiom units, then clauses in their
// source insertion order. The source tactic supplies its AST variable printer.
func (c *HardwareContext) DisplayConstraints(w io.Writer, useStar bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	display := c.displayVariable
	for variable := range c.isInt {
		x := uint32(variable)
		if c.monomials[x] == nil && c.polynomials[x] == nil {
			continue
		}
		if err := hardwareWriteVariable(w, x, display); err != nil {
			return err
		}
		if err := hardwareWrite(w, " = "); err != nil {
			return err
		}
		if err := c.displayDefinition(w, x, useStar, display); err != nil {
			return err
		}
		if err := hardwareWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, unit := range c.unitClauses {
		a := unit.atom
		if err := c.displayInequality(w, a.variable, &a.value, a.lower, a.open, display); err != nil {
			return err
		}
		if err := hardwareWrite(w, "\n"); err != nil {
			return err
		}
	}
	for _, clause := range c.clauses {
		if clause.lemma {
			continue
		}
		for i, a := range clause.atoms {
			if i != 0 {
				if err := hardwareWrite(w, " or "); err != nil {
					return err
				}
			}
			if err := c.displayInequality(w, a.variable, &a.value, a.lower, a.open, display); err != nil {
				return err
			}
		}
		if err := hardwareWrite(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *HardwareContext) displayNodeBounds(w io.Writer, n *HardwareNode, display HardwareVariableDisplay) error {
	for variable := range c.isInt {
		l, u := n.lower[variable], n.upper[variable]
		if l != nil {
			if err := c.displayInequality(w, l.variable, &l.value, true, l.open, display); err != nil {
				return err
			}
			if err := hardwareWrite(w, " "); err != nil {
				return err
			}
		}
		if u != nil {
			if err := c.displayInequality(w, u.variable, &u.value, false, u.open, display); err != nil {
				return err
			}
		}
		if l != nil || u != nil {
			if err := hardwareWrite(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayBounds visits consistent leaves in the source tree order, including
// selected leaves removed from the search queue.
func (c *HardwareContext) DisplayBounds(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	display := c.displayVariable
	if c.root == nil {
		return nil
	}
	children := make(map[*HardwareNode][]*HardwareNode, len(c.nodes))
	for _, n := range c.nodes {
		if !n.closed && n.parent != nil {
			children[n.parent] = append(children[n.parent], n)
		}
	}
	first := true
	var visit func(*HardwareNode) error
	visit = func(n *HardwareNode) error {
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
			if err := hardwareWrite(w, "=========\n"); err != nil {
				return err
			}
		}
		first = false
		return c.displayNodeBounds(w, n, display)
	}
	return visit(c.root)
}
