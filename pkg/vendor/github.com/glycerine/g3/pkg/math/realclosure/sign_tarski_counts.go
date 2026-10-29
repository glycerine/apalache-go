package realclosure

// Source: src/math/realclosure/realclosure.cpp:1613-1660 and 1667-1691.

import "github.com/glycerine/g3/pkg/arena"

// countSignsFromTarski follows count_signs_at_zeros_core. The q² Tarski
// result is read only on the source's expensive branch; queriedSquare reports
// that branch even when the initial q query equals +/-numRoots.
func countSignsFromTarski(numRoots, taqQ, taqQ2 int) (equal, positive, negative int, queriedSquare bool, err error) {
	if numRoots <= 0 || taqQ < -numRoots || taqQ > numRoots {
		return 0, 0, 0, false, arena.ErrInvalidSlot
	}
	if taqQ == numRoots {
		equal, positive, negative = 0, numRoots, 0
	} else if taqQ == -numRoots {
		equal, positive, negative = 0, 0, numRoots
	}
	if taqQ == numRoots-1 {
		equal, positive, negative = 1, numRoots-1, 0
	} else if taqQ == -(numRoots - 1) {
		equal, positive, negative = 1, 0, numRoots-1
	} else {
		queriedSquare = true
		if taqQ2 < 0 || taqQ2 > numRoots || (taqQ2+taqQ)%2 != 0 || (taqQ2-taqQ)%2 != 0 {
			return 0, 0, 0, true, arena.ErrInvalidSlot
		}
		equal = numRoots - taqQ2
		positive = (taqQ2 + taqQ) / 2
		negative = (taqQ2 - taqQ) / 2
	}
	if equal < 0 || positive < 0 || negative < 0 || equal+positive+negative != numRoots {
		return 0, 0, 0, queriedSquare, arena.ErrInvalidSlot
	}
	return equal, positive, negative, queriedSquare, nil
}

// advanceFromTarski reaches the sign-determination transition from the two
// source Tarski query results. Computing those queries from p and q remains
// part of the unfinished polynomial/root-isolation owner.
func (s *signDetermination) advanceFromTarski(q []*realValue, taqQ, taqQ2 int, expandedTarski []int) (bool, bool, error) {
	if s == nil {
		return false, false, arena.ErrInvalidSlot
	}
	equal, positive, negative, _, err := countSignsFromTarski(s.numRoots, taqQ, taqQ2)
	if err != nil {
		return false, false, err
	}
	return s.advanceRationalProducts(equal, positive, negative, q, expandedTarski)
}
