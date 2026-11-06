package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1179-1317, concrete config_mpf
// candidate normalization and bound relevance.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// NormalizeBound changes an owned candidate in place, matching the source's
// pre-relevance ceil/floor and strict integer endpoint adjustment.
func (c *ArbitraryContext) NormalizeBound(variable uint32, value *numeral.ArbitraryFloat, lower bool, open *bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if variable >= uint32(len(c.variables)) || value == nil || open == nil {
		return arena.ErrInvalidSlot
	}
	if !c.variables[variable] {
		return nil
	}
	m := c.numbers
	integer, err := m.IsInteger(value)
	if err != nil {
		return err
	}
	if !integer {
		*open = false
	}
	if lower {
		if err := m.Ceil(value, value); err != nil {
			return err
		}
	} else if err := m.Floor(value, value); err != nil {
		return err
	}
	if !*open {
		return nil
	}
	*open = false
	m.SetRounding(!lower)
	if lower {
		return m.Increment(value)
	}
	return m.Decrement(value)
}

// RelevantNewBound evaluates conflict, zero-epsilon improvement, max-bound,
// then positive-epsilon gap in the source order. Candidate is normalized.
func (c *ArbitraryContext) RelevantNewBound(n *ArbitraryNode, variable uint32, candidate *numeral.ArbitraryFloat, lower, open bool) (bool, error) {
	if err := c.checkNode(n, variable); err != nil {
		return false, err
	}
	if candidate == nil {
		return false, arena.ErrInvalidSlot
	}
	m := c.numbers
	currentLower, currentUpper := n.lower[variable], n.upper[variable]
	if err := c.traceRelevantCandidate(variable, candidate, lower, open, currentLower, currentUpper); err != nil {
		return false, err
	}
	if lower {
		if currentUpper != nil {
			greater, err := m.Greater(candidate, &currentUpper.value)
			if err != nil {
				return false, err
			}
			if greater {
				return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
			}
			if open || currentUpper.open {
				equal, err := m.Equal(candidate, &currentUpper.value)
				if err != nil {
					return false, err
				}
				if equal {
					return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
				}
			}
		}
		if c.zeroEpsilon && currentLower != nil {
			less, err := m.Less(candidate, &currentLower.value)
			if err != nil {
				return false, err
			}
			if less {
				return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
			}
			if currentLower.open || !open {
				equal, err := m.Equal(candidate, &currentLower.value)
				if err != nil {
					return false, err
				}
				if equal {
					return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
				}
			}
		}
		if currentUpper == nil {
			tooLarge, err := m.Less(c.maxBound, candidate)
			if err != nil {
				return false, err
			}
			if tooLarge {
				return c.traceRelevantDecision(false, 1229, "irrelevant because exceeds :max-bound threshold.", nil)
			}
		}
	} else {
		if currentLower != nil {
			greater, err := m.Greater(&currentLower.value, candidate)
			if err != nil {
				return false, err
			}
			if greater {
				return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
			}
			if open || currentLower.open {
				equal, err := m.Equal(candidate, &currentLower.value)
				if err != nil {
					return false, err
				}
				if equal {
					return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
				}
			}
		}
		if c.zeroEpsilon && currentUpper != nil {
			less, err := m.Less(&currentUpper.value, candidate)
			if err != nil {
				return false, err
			}
			if less {
				return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
			}
			if currentUpper.open || !open {
				equal, err := m.Equal(candidate, &currentUpper.value)
				if err != nil {
					return false, err
				}
				if equal {
					return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
				}
			}
		}
		if currentLower == nil {
			tooSmall, err := m.Less(candidate, c.minusMaxBound)
			if err != nil {
				return false, err
			}
			if tooSmall {
				return c.traceRelevantDecision(false, 1279, "irrelevant because exceeds -:max-bound threshold.", nil)
			}
		}
	}
	if c.zeroEpsilon || lower && currentLower == nil || !lower && currentUpper == nil {
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	current := currentUpper
	if lower {
		current = currentLower
	}
	abs, minimum, delta := &c.relevanceScratch[0], &c.relevanceScratch[1], &c.relevanceScratch[2]
	if err := m.Abs(&current.value, abs); err != nil {
		return false, err
	}
	if currentLower != nil && currentUpper != nil {
		if err := m.Sub(&currentUpper.value, &currentLower.value, minimum); err != nil {
			return false, err
		}
		less, err := m.Less(abs, minimum)
		if err != nil {
			return false, err
		}
		if less {
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
	greater, err := m.Greater(minimum, delta)
	if err != nil {
		return false, err
	}
	if greater {
		if err := m.Copy(delta, minimum); err != nil {
			return false, err
		}
	}
	if err := m.Mul(delta, c.epsilon, delta); err != nil {
		return false, err
	}
	if lower {
		if err := m.Add(&currentLower.value, delta, delta); err != nil {
			return false, err
		}
		if err := c.traceRelevantLowerBug(candidate, delta, &currentLower.value, minimum); err != nil {
			return false, err
		}
		relevant, err := m.Greater(candidate, delta)
		if err != nil {
			return false, err
		}
		if !relevant {
			return c.traceRelevantDecision(false, 1259, "irrelevant because does not improve existing bound to at least ", delta)
		}
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	if err := m.Sub(&currentUpper.value, delta, delta); err != nil {
		return false, err
	}
	relevant, err := m.Less(candidate, delta)
	if err != nil {
		return false, err
	}
	if !relevant {
		return c.traceRelevantDecision(false, 1304, "irrelevant because does not improve existing bound to at least ", delta)
	}
	return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
}

// PropagateCandidate performs the source normalize/relevance/publish sequence
// for definition-derived bounds. The context keeps its reusable MPF scratch.
func (c *ArbitraryContext) PropagateCandidate(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open bool) (bool, error) {
	return c.propagateCandidate(n, variable, value, lower, open, 0, false)
}

func (c *ArbitraryContext) propagateCandidateDefinition(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open bool, definition uint32) (bool, error) {
	return c.propagateCandidate(n, variable, value, lower, open, definition, true)
}

func (c *ArbitraryContext) propagateCandidate(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open bool, definition uint32, fromDefinition bool) (bool, error) {
	if err := c.checkNode(n, variable); err != nil {
		return false, err
	}
	if value == nil {
		return false, arena.ErrInvalidSlot
	}
	if err := c.numbers.Copy(&c.candidateScratch, value); err != nil {
		return false, err
	}
	if err := c.NormalizeBound(variable, &c.candidateScratch, lower, &open); err != nil {
		return false, err
	}
	relevant, err := c.RelevantNewBound(n, variable, &c.candidateScratch, lower, open)
	if err != nil || !relevant {
		return relevant, err
	}
	if err := c.publishBoundWithReason(n, variable, &c.candidateScratch, lower, open, !fromDefinition, nil, definition, fromDefinition, true); err != nil {
		return true, err
	}
	return true, nil
}
