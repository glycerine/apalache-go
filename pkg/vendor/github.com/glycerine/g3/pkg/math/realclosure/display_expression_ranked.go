package realclosure

// Source: src/math/realclosure/realclosure.cpp:5849-5863, 5912-5942.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRankedExpression(w io.Writer, value *realValue, compact, pretty bool) error {
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
	denominator := value.functionDenominator.coefficients
	if len(denominator) != 0 && !s.rankedDenominatorIsOne(denominator) {
		if s.rankedNumeratorIsOne(value.functionNumerator.coefficients) {
			if err := writeRealClosureString(w, "1/("); err != nil {
				return err
			}
			if err := s.displayRankedPolynomial(w, denominator, func() error {
				return s.displayRankedExtensionVariable(w, value.root, compact, pretty)
			}, compact, pretty); err != nil {
				return err
			}
			return writeRealClosureString(w, ")")
		}
		if err := writeRealClosureString(w, "("); err != nil {
			return err
		}
		if err := s.displayRankedPolynomial(w, value.functionNumerator.coefficients, func() error {
			return s.displayRankedExtensionVariable(w, value.root, compact, pretty)
		}, compact, pretty); err != nil {
			return err
		}
		if err := writeRealClosureString(w, ")/("); err != nil {
			return err
		}
		if err := s.displayRankedPolynomial(w, denominator, func() error {
			return s.displayRankedExtensionVariable(w, value.root, compact, pretty)
		}, compact, pretty); err != nil {
			return err
		}
		return writeRealClosureString(w, ")")
	}
	return s.displayRankedPolynomial(w, value.functionNumerator.coefficients, func() error {
		return s.displayRankedExtensionVariable(w, value.root, compact, pretty)
	}, compact, pretty)
}

func (s *realValueStore) displayRankedExtensionVariable(w io.Writer, root *algebraicRoot, compact, pretty bool) error {
	if root.kind != realExtensionAlgebraic || compact {
		return writeRealClosureString(w, rankedRootName(root, pretty))
	}
	return s.displayRankedAlgebraicDefinition(w, root, compact, pretty)
}

func (s *realValueStore) rankedDenominatorIsOne(denominator []*realValue) bool {
	if len(denominator) == 0 {
		return true
	}
	if len(denominator) != 1 || denominator[0] == nil || !denominator[0].isRational {
		return false
	}
	isOne, err := s.numbers.IsOne(&denominator[0].rational)
	return err == nil && isOne
}

func (s *realValueStore) rankedNumeratorIsOne(numerator []*realValue) bool {
	return s.rankedDenominatorIsOne(numerator)
}
