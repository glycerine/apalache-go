package realclosure

import "github.com/glycerine/g3/pkg/arena"

// advanceNormalizedUnbounded computes the sign groups for q over p's roots,
// then performs the retained normalized prs*q/q² matrix transition.
func (s *signDetermination) advanceNormalizedUnbounded(p, q []*realValue, expandedTarski []int) (bool, bool, error) {
	if s == nil || s.store == nil {
		return false, false, arena.ErrInvalidSlot
	}
	numRoots, err := s.store.rootCountUnboundedNormalized(p)
	if err != nil || numRoots != s.numRoots {
		if err != nil {
			return false, false, err
		}
		return false, false, arena.ErrInvalidSlot
	}
	equal, positive, negative, err := s.store.signCountsUnboundedNormalized(p, q)
	if err != nil {
		return false, false, err
	}
	return s.advanceAlgebraicProducts(equal, positive, negative, q, expandedTarski)
}
