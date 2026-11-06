package arith

// Source: src/tactic/arith/bound_propagator.cpp:891-963.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func writeBoundText(out io.Writer, value string) error {
	n, err := io.WriteString(out, value)
	if err != nil {
		return err
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func (s *BoundEquationStore) displayVarBounds(out io.Writer, x uint32, approximate, precise bool) error {
	v := &s.variables[x]
	if v.lower == nil {
		if err := writeBoundText(out, "-oo <"); err != nil {
			return err
		}
	} else {
		if precise {
			value, err := s.manager.String(&v.lower.value)
			if err != nil {
				return err
			}
			if err = writeBoundText(out, value); err != nil {
				return err
			}
		}
		if precise && approximate {
			if err := writeBoundText(out, " | "); err != nil {
				return err
			}
		}
		if approximate {
			if err := writeBoundText(out, strconv.FormatFloat(v.lower.approx, 'g', 6, 64)); err != nil {
				return err
			}
		}
		separator := " <="
		if v.lower.strict {
			separator = " <"
		}
		if err := writeBoundText(out, separator); err != nil {
			return err
		}
	}
	if err := writeBoundText(out, " x"+strconv.FormatUint(uint64(x), 10)+" "); err != nil {
		return err
	}
	if v.upper == nil {
		return writeBoundText(out, "< oo")
	}
	separator := "<= "
	if v.upper.strict {
		separator = "< "
	}
	if err := writeBoundText(out, separator); err != nil {
		return err
	}
	if precise {
		value, err := s.manager.String(&v.upper.value)
		if err != nil {
			return err
		}
		if err = writeBoundText(out, value); err != nil {
			return err
		}
	}
	if precise && approximate {
		if err := writeBoundText(out, " | "); err != nil {
			return err
		}
	}
	if approximate {
		return writeBoundText(out, strconv.FormatFloat(v.upper.approx, 'g', 6, 64))
	}
	return nil
}

// displayBoundsOf is the reached equation-specific diagnostic used by exact
// propagation TRACE. It includes every variable in equation order.
func (s *BoundEquationStore) displayBoundsOf(out io.Writer, eq *LinearEquation) error {
	for _, x := range eq.xs {
		if err := s.displayVarBounds(out, x, true, true); err != nil {
			return err
		}
		if err := writeBoundText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// Display follows source variable-slot order, skips dead variables and then
// prints every retained equation in insertion order, including dead marks.
func (s *BoundEquationStore) Display(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	for x := range s.variables {
		if !s.variables[x].active {
			continue
		}
		if err := s.displayVarBounds(out, uint32(x), true, true); err != nil {
			return err
		}
		if err := writeBoundText(out, "\n"); err != nil {
			return err
		}
	}
	for i := range s.constraints {
		if err := s.equations.Display(out, s.constraints[i].eq); err != nil {
			return err
		}
		if err := writeBoundText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
