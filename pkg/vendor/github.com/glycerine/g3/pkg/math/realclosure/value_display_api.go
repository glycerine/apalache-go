package realclosure

// Source: src/math/realclosure/realclosure.cpp:5780-6011 and manager display
// wrappers at 6241-6254. This covers the live rational and numerator-x
// algebraic values and their rational defining polynomials.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func writeRealClosureString(w io.Writer, text string) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	_, err := io.WriteString(w, text)
	return err
}

func (s *realValueStore) displayPolynomialValue(w io.Writer, coefficients []*realValue, variable string, pretty bool) error {
	if s == nil || w == nil || len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return arena.ErrInvalidSlot
	}
	first := true
	for i := len(coefficients) - 1; i >= 0; i-- {
		coefficient := coefficients[i]
		if coefficient == nil {
			continue
		}
		if coefficient.store != s || coefficient.deleted || coefficient.refs == 0 || !coefficient.isRational {
			return arena.ErrInvalidSlot
		}
		if !first {
			if err := writeRealClosureString(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one, err := s.numbers.IsOne(&coefficient.rational)
		if err != nil {
			return err
		}
		if i == 0 || !one {
			if i > 0 {
				if err := writeRealClosureString(w, "("); err != nil {
					return err
				}
			}
			if err := s.numbers.DisplayTo(w, &coefficient.rational); err != nil {
				return err
			}
			if i > 0 {
				separator := "*"
				if pretty {
					separator = " "
				}
				if err := writeRealClosureString(w, ")"+separator); err != nil {
					return err
				}
			}
		}
		if i > 0 {
			if err := writeRealClosureString(w, variable); err != nil {
				return err
			}
			if i > 1 {
				exponent := "^" + strconv.Itoa(i)
				if pretty {
					exponent = "<sup>" + strconv.Itoa(i) + "</sup>"
				}
				if err := writeRealClosureString(w, exponent); err != nil {
					return err
				}
			}
		}
	}
	if first {
		return writeRealClosureString(w, "0")
	}
	return nil
}

func (s *realValueStore) displayAlgebraicDefinition(w io.Writer, root *algebraicRoot, pretty bool) error {
	if root == nil || root.deleted || root.owner == nil || root.owner.values != s || root.interval == nil || root.isolating == nil {
		return arena.ErrInvalidSlot
	}
	if err := writeRealClosureString(w, "root("); err != nil {
		return err
	}
	if err := s.displayPolynomialValue(w, root.polynomial.coefficients, "x", pretty); err != nil {
		return err
	}
	if err := writeRealClosureString(w, ", "); err != nil {
		return err
	}
	if pretty {
		if err := root.isolating.DisplayPP(w); err != nil {
			return err
		}
	} else if err := root.isolating.Display(w); err != nil {
		return err
	}
	if root.signs == nil {
		return writeRealClosureString(w, ", {})")
	}
	if err := writeRealClosureString(w, ", {condition #"+strconv.Itoa(root.signIndex)+"})"); err != nil {
		return err
	}
	return nil
}

func (s *realValueStore) displayValue(w io.Writer, value *realValue, compact, pretty bool) error {
	if w == nil || s == nil {
		return arena.ErrInvalidSlot
	}
	if value == nil {
		return writeRealClosureString(w, "0")
	}
	if value.store != s || value.deleted || value.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.numbers.DisplayTo(w, &value.rational)
	}
	if value.root == nil || len(value.functionNumerator.coefficients) != 2 || value.functionNumerator.coefficients[0] != nil {
		return arena.ErrInvalidSlot
	}
	index, err := value.root.registryIndex()
	if err != nil {
		return err
	}
	name := "r!" + strconv.Itoa(index)
	if pretty {
		name = "&alpha;<sub>" + strconv.Itoa(index) + "</sub>"
	}
	if !compact {
		return s.displayAlgebraicDefinition(w, value.root, pretty)
	}
	if err := writeRealClosureString(w, "["+name+"; "+name+" := "); err != nil {
		return err
	}
	if err := s.displayAlgebraicDefinition(w, value.root, pretty); err != nil {
		return err
	}
	return writeRealClosureString(w, "]")
}

func (s *realValueStore) displayValueInterval(w io.Writer, value *realValue, bounds *numeral.BinaryRationalManager) error {
	if value == nil {
		return writeRealClosureString(w, "[0, 0]")
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return err
	}
	return interval.Display(w)
}
