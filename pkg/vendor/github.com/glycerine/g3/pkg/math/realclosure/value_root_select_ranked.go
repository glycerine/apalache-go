package realclosure

// Source: src/math/realclosure/realclosure.cpp:5641-5654.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) selectRankedPrincipalRoot(roots []*realValue, usePrem bool) (selected *realValue, err error) {
	if len(roots) != 1 && len(roots) != 2 {
		return nil, arena.ErrInvalidSlot
	}
	index := 0
	if len(roots) == 2 {
		firstSign, signErr := s.determineRankedRetainedValueSign(roots[0], usePrem)
		if signErr != nil {
			return nil, signErr
		}
		if firstSign <= 0 {
			secondSign, signErr := s.determineRankedRetainedValueSign(roots[1], usePrem)
			if signErr != nil || secondSign <= 0 {
				return nil, errors.Join(signErr, arena.ErrInvalidSlot)
			}
			index = 1
		}
	}
	selected = roots[index]
	for i, root := range roots {
		if i != index {
			err = errors.Join(err, s.release(root))
		}
	}
	return selected, err
}
