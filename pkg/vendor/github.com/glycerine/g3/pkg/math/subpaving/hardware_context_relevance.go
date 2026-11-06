package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1200-1317, concrete
// config_hwf candidate-bound relevance and arithmetic failure policy.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// RelevantNewBound applies the source's conflict-first, zero-epsilon,
// maximum-bound, and epsilon-gap tests in that order. The caller supplies an
// already normalized candidate, as do the retained propagation routines.
func (c *HardwareContext) RelevantNewBound(n *HardwareNode, variable uint32, candidate *numeral.HardwareFloat, lower, open bool) (relevant bool, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return false, err
	}
	defer func() {
		if errors.Is(err, numeral.ErrHardwareFloatNonRegular) {
			c.arithmeticFailed = true
			relevant, err = false, nil
		}
	}()
	m := c.numbers
	var checked numeral.HardwareFloat
	if err = m.Set(&checked, candidate); err != nil {
		return false, err
	}
	currentLower, currentUpper := n.lower[variable], n.upper[variable]
	if err = c.traceRelevantCandidate(variable, candidate, lower, open, currentLower, currentUpper); err != nil {
		return false, err
	}
	if lower {
		if currentUpper != nil && (m.Greater(candidate, &currentUpper.value) ||
			(open || currentUpper.open) && m.Equal(candidate, &currentUpper.value)) {
			return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
		}
		if c.zeroEpsilon && currentLower != nil && (m.Less(candidate, &currentLower.value) ||
			(currentLower.open || !open) && m.Equal(candidate, &currentLower.value)) {
			return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
		}
		if currentUpper == nil && m.Less(&c.maxBound, candidate) {
			return c.traceRelevantDecision(false, 1229, "irrelevant because exceeds :max-bound threshold.", nil)
		}
	} else {
		if currentLower != nil && (m.Greater(&currentLower.value, candidate) ||
			(open || currentLower.open) && m.Equal(candidate, &currentLower.value)) {
			return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
		}
		if c.zeroEpsilon && currentUpper != nil && (m.Less(&currentUpper.value, candidate) ||
			(currentUpper.open || !open) && m.Equal(candidate, &currentUpper.value)) {
			return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
		}
		if currentLower == nil && m.Less(candidate, &c.minusMaxBound) {
			return c.traceRelevantDecision(false, 1279, "irrelevant because exceeds -:max-bound threshold.", nil)
		}
	}
	if c.zeroEpsilon || lower && currentLower == nil || !lower && currentUpper == nil {
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	current := currentUpper
	if lower {
		current = currentLower
	}
	var absolute, minimum, delta numeral.HardwareFloat
	if err = m.Abs(&current.value, &absolute); err != nil {
		return false, err
	}
	if currentLower != nil && currentUpper != nil {
		if err = m.Sub(&currentUpper.value, &currentLower.value, &minimum); err != nil {
			return false, err
		}
		if m.Less(&absolute, &minimum) {
			if err = m.Set(&minimum, &absolute); err != nil {
				return false, err
			}
		}
	} else if err = m.Set(&minimum, &absolute); err != nil {
		return false, err
	}
	if err = m.SetInt32(&delta, 1); err != nil {
		return false, err
	}
	if m.Greater(&minimum, &delta) {
		if err = m.Set(&delta, &minimum); err != nil {
			return false, err
		}
	}
	if err = m.Mul(&delta, &c.epsilon, &delta); err != nil {
		return false, err
	}
	if lower {
		if err = m.Add(&currentLower.value, &delta, &delta); err != nil {
			return false, err
		}
		if err = c.traceRelevantLowerBug(candidate, &delta, &currentLower.value, &minimum); err != nil {
			return false, err
		}
		if !m.Greater(candidate, &delta) {
			return c.traceRelevantDecision(false, 1259, "irrelevant because does not improve existing bound to at least ", &delta)
		}
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	if err = m.Sub(&currentUpper.value, &delta, &delta); err != nil {
		return false, err
	}
	if !m.Less(candidate, &delta) {
		return c.traceRelevantDecision(false, 1304, "irrelevant because does not improve existing bound to at least ", &delta)
	}
	return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
}

// PropagateCandidate is the retained definition path: normalize a scalar
// candidate, test relevance, then publish only a selected bound.
func (c *HardwareContext) PropagateCandidate(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool) (bool, error) {
	return c.propagateCandidateReason(n, variable, value, lower, open, hardwareReasonUnknown, 0)
}

func (c *HardwareContext) propagateCandidateReason(n *HardwareNode, variable uint32, value *numeral.HardwareFloat, lower, open bool, reason hardwareBoundReasonKind, reasonVariable uint32) (bool, error) {
	if err := c.checkNode(n, variable); err != nil {
		return false, err
	}
	var candidate numeral.HardwareFloat
	if err := c.numbers.Set(&candidate, value); err != nil {
		return false, err
	}
	if err := c.NormalizeBound(variable, &candidate, lower, &open); err != nil {
		return false, err
	}
	relevant, err := c.RelevantNewBound(n, variable, &candidate, lower, open)
	if err != nil || !relevant {
		return relevant, err
	}
	return true, c.propagateBoundReason(n, variable, &candidate, lower, open, reason, reasonVariable)
}
