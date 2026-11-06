package realclosure

// Source: src/math/realclosure/realclosure.cpp:5865-5897.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRankedAlgebraicDefinition(w io.Writer, root *algebraicRoot, compact, pretty bool) error {
	if root == nil || root.kind != realExtensionAlgebraic || !root.isRegistered() || root.owner.values != s || root.isolating == nil {
		return arena.ErrInvalidSlot
	}
	if err := writeRealClosureString(w, "root("); err != nil {
		return err
	}
	if err := s.displayRankedPolynomial(w, root.polynomial.coefficients, func() error {
		return writeRealClosureString(w, "x")
	}, compact, pretty); err != nil {
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
	return writeRealClosureString(w, ", {condition})")
}
