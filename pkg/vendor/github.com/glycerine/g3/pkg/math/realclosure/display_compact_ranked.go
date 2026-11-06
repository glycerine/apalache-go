package realclosure

// Source: src/math/realclosure/realclosure.cpp:5944-5978.

import "io"

func (s *realValueStore) displayCompactRankedValue(w io.Writer, value *realValue, pretty bool) error {
	roots, err := s.collectRankedAlgebraicRoots(value)
	if err != nil {
		return err
	}
	if len(roots) == 0 {
		return s.displayRankedExpression(w, value, true, pretty)
	}
	if err := writeRealClosureString(w, "["); err != nil {
		return err
	}
	if err := s.displayRankedExpression(w, value, true, pretty); err != nil {
		return err
	}
	for _, root := range roots {
		if err := writeRealClosureString(w, "; "+rankedRootName(root, pretty)+" := "); err != nil {
			return err
		}
		if err := s.displayRankedAlgebraicDefinition(w, root, true, pretty); err != nil {
			return err
		}
	}
	return writeRealClosureString(w, "]")
}
