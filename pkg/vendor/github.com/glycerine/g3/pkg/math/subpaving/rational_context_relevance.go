package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1204-1317, concrete config_mpq
// default epsilon and maximum-bound policy for definition candidates.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RelevantNewBound evaluates a normalized candidate in source order:
// conflicting opposite bound, source maximum, then the epsilon improvement.
func (c *RationalContext) RelevantNewBound(n *RationalNode, variable uint32, candidate *numeral.Rational, lower, open bool) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if n == nil || n.context != c || !n.registered || variable >= uint32(len(c.variables)) || candidate == nil || n.conflictVar != math.MaxUint32 {
		return false, arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return false, err
	}
	m := c.numbers
	currentLower, currentUpper := n.lower[variable], n.upper[variable]
	if err := c.traceRelevantCandidate(variable, candidate, lower, open, currentLower, currentUpper); err != nil {
		return false, err
	}
	if lower {
		if currentUpper != nil {
			cmp, err := m.Compare(candidate, &currentUpper.value)
			if err != nil {
				return false, err
			}
			if cmp > 0 || cmp == 0 && (open || currentUpper.open) {
				return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentLower != nil {
			cmp, err := m.Compare(candidate, &currentLower.value)
			if err != nil {
				return false, err
			}
			if cmp < 0 || cmp == 0 && (currentLower.open || !open) {
				return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentUpper == nil {
			cmp, err := m.Compare(&c.maxBound, candidate)
			if err != nil {
				return false, err
			}
			if cmp < 0 {
				return c.traceRelevantDecision(false, 1229, "irrelevant because exceeds :max-bound threshold.", nil)
			}
		}
	} else {
		if currentLower != nil {
			cmp, err := m.Compare(&currentLower.value, candidate)
			if err != nil {
				return false, err
			}
			if cmp > 0 || cmp == 0 && (open || currentLower.open) {
				return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentUpper != nil {
			cmp, err := m.Compare(&currentUpper.value, candidate)
			if err != nil {
				return false, err
			}
			if cmp < 0 || cmp == 0 && (currentUpper.open || !open) {
				return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentLower == nil {
			cmp, err := m.Compare(candidate, &c.minusMaxBound)
			if err != nil {
				return false, err
			}
			if cmp < 0 {
				return c.traceRelevantDecision(false, 1279, "irrelevant because exceeds -:max-bound threshold.", nil)
			}
		}
	}
	if c.zeroEpsilon {
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	current := currentUpper
	if lower {
		current = currentLower
	}
	if current == nil {
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	abs, minimum, delta := &c.relevanceScratch[0], &c.relevanceScratch[1], &c.relevanceScratch[2]
	if err := m.Copy(abs, &current.value); err != nil {
		return false, err
	}
	if err := m.Abs(abs); err != nil {
		return false, err
	}
	if currentLower != nil && currentUpper != nil {
		if err := m.Sub(&currentUpper.value, &currentLower.value, minimum); err != nil {
			return false, err
		}
		cmp, err := m.Compare(abs, minimum)
		if err != nil {
			return false, err
		}
		if cmp < 0 {
			if err := m.Copy(minimum, abs); err != nil {
				return false, err
			}
		}
	} else if err := m.Copy(minimum, abs); err != nil {
		return false, err
	}
	if err := m.SetInt32(delta, 1); err != nil {
		return false, err
	}
	cmp, err := m.Compare(minimum, delta)
	if err != nil {
		return false, err
	}
	if cmp > 0 {
		if err := m.Copy(delta, minimum); err != nil {
			return false, err
		}
	}
	if err := m.Mul(delta, &c.epsilon, delta); err != nil {
		return false, err
	}
	if lower {
		if err := m.Add(&currentLower.value, delta, delta); err != nil {
			return false, err
		}
		if err := c.traceRelevantLowerBug(candidate, delta, &currentLower.value, minimum); err != nil {
			return false, err
		}
		cmp, err := m.Compare(candidate, delta)
		if err != nil {
			return false, err
		}
		if cmp <= 0 {
			return c.traceRelevantDecision(false, 1259, "irrelevant because does not improve existing bound to at least ", delta)
		}
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	if err := m.Sub(&currentUpper.value, delta, delta); err != nil {
		return false, err
	}
	cmp, err = m.Compare(candidate, delta)
	if err != nil {
		return false, err
	}
	if cmp >= 0 {
		return c.traceRelevantDecision(false, 1304, "irrelevant because does not improve existing bound to at least ", delta)
	}
	return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
}
