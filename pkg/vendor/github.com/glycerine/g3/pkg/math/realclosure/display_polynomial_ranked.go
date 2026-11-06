package realclosure

// Source: src/math/realclosure/realclosure.cpp:5790-5847.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRankedPolynomial(w io.Writer, coefficients []*realValue, variable func() error, compact, pretty bool) error {
	if s == nil || w == nil || variable == nil || len(coefficients) == 0 || coefficients[len(coefficients)-1] == nil {
		return arena.ErrInvalidSlot
	}
	first := true
	for i := len(coefficients) - 1; i >= 0; i-- {
		coefficient := coefficients[i]
		if coefficient == nil {
			continue
		}
		if !first {
			if err := writeRealClosureString(w, " + "); err != nil {
				return err
			}
		}
		first = false
		one := false
		if coefficient.isRational {
			var err error
			one, err = s.numbers.IsOne(&coefficient.rational)
			if err != nil {
				return err
			}
		}
		if i == 0 || !one {
			parenthesize := i > 0 && rankedCoefficientNeedsParentheses(coefficient)
			if parenthesize {
				if err := writeRealClosureString(w, "("); err != nil {
					return err
				}
			}
			if err := s.displayRankedExpression(w, coefficient, compact, pretty); err != nil {
				return err
			}
			if parenthesize {
				if err := writeRealClosureString(w, ")"); err != nil {
					return err
				}
			}
			if i > 0 {
				separator := "*"
				if pretty {
					separator = " "
				}
				if err := writeRealClosureString(w, separator); err != nil {
					return err
				}
			}
		}
		if i > 0 {
			if err := variable(); err != nil {
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
	return nil
}
