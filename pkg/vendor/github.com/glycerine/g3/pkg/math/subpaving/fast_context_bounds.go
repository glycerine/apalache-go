package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:528-589,1176-1315,
// concrete config_mpff bound normalization and relevance.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fastCompare(m *numeral.FastFloatManager, a, b *numeral.FastFloat) (int, error) {
	less, err := m.Less(a, b)
	if err != nil {
		return 0, err
	}
	if less {
		return -1, nil
	}
	less, err = m.Less(b, a)
	if err != nil {
		return 0, err
	}
	if less {
		return 1, nil
	}
	return 0, nil
}

// The source checks strict order first and calls eq only when openness makes
// equality relevant. Keep that order for the concrete fast-float manager.
func fastGreaterOrEqualWhen(m *numeral.FastFloatManager, a, b *numeral.FastFloat, equalMatters bool) (bool, error) {
	greater, err := m.Greater(a, b)
	if err != nil || greater || !equalMatters {
		return greater, err
	}
	return m.Equal(a, b)
}

func fastLessOrEqualWhen(m *numeral.FastFloatManager, a, b *numeral.FastFloat, equalMatters bool) (bool, error) {
	less, err := m.Less(a, b)
	if err != nil || less || !equalMatters {
		return less, err
	}
	return m.Equal(a, b)
}

func (c *FastContext) checkNode(n *FastNode, variable uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || variable >= uint32(len(c.isInt)) {
		return arena.ErrInvalidSlot
	}
	return n.open()
}

// NormalizeBound updates a scratch numeral and openness as in the source's
// integer-bound normalization before relevance testing.
func (c *FastContext) NormalizeBound(variable uint32, value *numeral.FastFloat, lower bool, open *bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if variable >= uint32(len(c.isInt)) || open == nil {
		return arena.ErrInvalidSlot
	}
	m := c.numbers
	if _, err := m.IsZero(value); err != nil {
		return err
	}
	if !c.isInt[variable] {
		return nil
	}
	integer, err := m.IsInteger(value)
	if err != nil {
		return err
	}
	if !integer {
		*open = false
	}
	if lower {
		err = m.CeilInPlace(value)
	} else {
		err = m.FloorInPlace(value)
	}
	if err != nil {
		return err
	}
	if *open {
		*open = false
		if lower {
			if err = m.SetRounding(false); err != nil {
				return err
			}
			return m.Inc(value)
		}
		if err = m.SetRounding(true); err != nil {
			return err
		}
		return m.Dec(value)
	}
	return nil
}

// RelevantNewBound follows the source conflict-first, zero-epsilon, threshold
// and epsilon-gap branches. Arithmetic exceptions mark the context failed and
// refuse the candidate, matching the source catch block.
func (c *FastContext) RelevantNewBound(n *FastNode, variable uint32, k *numeral.FastFloat, lower, open bool) (relevant bool, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return false, err
	}
	m := c.numbers
	if _, err = m.IsZero(k); err != nil {
		return false, err
	}
	defer func() {
		if errors.Is(err, numeral.ErrFastFloatDomain) {
			c.arithmeticFailed = true
			relevant, err = false, nil
		}
	}()
	currentLower, currentUpper := n.lower[variable], n.upper[variable]
	if err = c.traceRelevantCandidate(variable, k, lower, open, currentLower, currentUpper); err != nil {
		return false, err
	}
	if lower {
		if currentUpper != nil {
			conflict, e := fastGreaterOrEqualWhen(m, k, currentUpper.value, open || currentUpper.open)
			if e != nil {
				return false, e
			}
			if conflict {
				return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentLower != nil {
			irrelevant, e := fastLessOrEqualWhen(m, k, currentLower.value, currentLower.open || !open)
			if e != nil {
				return false, e
			}
			if irrelevant {
				return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentUpper == nil {
			cmp, e := fastCompare(m, c.maxBound, k)
			if e != nil {
				return false, e
			}
			if cmp < 0 {
				return c.traceRelevantDecision(false, 1229, "irrelevant because exceeds :max-bound threshold.", nil)
			}
		}
	} else {
		if currentLower != nil {
			conflict, e := fastGreaterOrEqualWhen(m, currentLower.value, k, open || currentLower.open)
			if e != nil {
				return false, e
			}
			if conflict {
				return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentUpper != nil {
			irrelevant, e := fastLessOrEqualWhen(m, currentUpper.value, k, currentUpper.open || !open)
			if e != nil {
				return false, e
			}
			if irrelevant {
				return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentLower == nil {
			cmp, e := fastCompare(m, k, c.minusMaxBound)
			if e != nil {
				return false, e
			}
			if cmp < 0 {
				return c.traceRelevantDecision(false, 1279, "irrelevant because exceeds -:max-bound threshold.", nil)
			}
		}
	}
	if c.zeroEpsilon || lower && currentLower == nil || !lower && currentUpper == nil {
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	minimumOwner, absoluteOwner, e := fastScratchPair(m)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, minimumOwner.Close(), absoluteOwner.Close()) }()
	minimum, e := minimumOwner.Get()
	if e != nil {
		return false, e
	}
	absolute, e := absoluteOwner.Get()
	if e != nil {
		return false, e
	}
	deltaOwner, e := numeral.NewScopedFastFloat(m)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, deltaOwner.Close()) }()
	delta, e := deltaOwner.Get()
	if e != nil {
		return false, e
	}
	current := currentUpper
	other := currentLower
	if lower {
		current, other = currentLower, currentUpper
	}
	if err = m.Copy(absolute, current.value); err != nil {
		return false, err
	}
	if err = m.Abs(absolute); err != nil {
		return false, err
	}
	if other != nil {
		if err = m.Sub(currentUpper.value, currentLower.value, minimum); err != nil {
			return false, err
		}
		cmp, e := fastCompare(m, absolute, minimum)
		if e != nil {
			return false, e
		}
		if cmp < 0 {
			if err = m.Copy(minimum, absolute); err != nil {
				return false, err
			}
		}
	} else {
		if err = m.Copy(minimum, absolute); err != nil {
			return false, err
		}
	}
	if err = m.SetInt32(delta, 1); err != nil {
		return false, err
	}
	cmp, e := fastCompare(m, minimum, delta)
	if e != nil {
		return false, e
	}
	// Source uses max(minimum, 1) before multiplying by epsilon.
	if cmp > 0 {
		if err = m.Copy(delta, minimum); err != nil {
			return false, err
		}
	}
	if err = m.Mul(delta, c.epsilon, delta); err != nil {
		return false, err
	}
	if lower {
		if err = m.Add(currentLower.value, delta, delta); err != nil {
			return false, err
		}
		if err = c.traceRelevantLowerBug(k, delta, currentLower.value, minimum); err != nil {
			return false, err
		}
		cmp, e = fastCompare(m, k, delta)
		if e != nil {
			return false, e
		}
		if cmp <= 0 {
			return c.traceRelevantDecision(false, 1259, "irrelevant because does not improve existing bound to at least ", delta)
		}
		return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
	}
	if err = m.Sub(currentUpper.value, delta, delta); err != nil {
		return false, err
	}
	cmp, e = fastCompare(m, k, delta)
	if e != nil {
		return false, e
	}
	if cmp >= 0 {
		return c.traceRelevantDecision(false, 1304, "irrelevant because does not improve existing bound to at least ", delta)
	}
	return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
}

// PropagateBound creates the normalized bound, checks the node's interval for
// conflict, then queues it after mk_bound completes. Relevance testing is the
// caller's choice, as source unit clauses publish directly.
func (c *FastContext) PropagateBound(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open bool) (err error) {
	return c.propagateBoundReason(n, variable, value, lower, open, fastBoundReason{})
}

func (c *FastContext) propagateBoundReason(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open bool, reason fastBoundReason) error {
	return c.publishBound(n, variable, value, lower, open, true, reason)
}

// DecideBound creates the source's branch decision without putting it on the
// propagation queue. AddRecentBounds queues surviving child decisions later.
func (c *FastContext) DecideBound(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open bool) error {
	if n == nil || n.parent == nil {
		return arena.ErrInvalidSlot
	}
	return c.publishBound(n, variable, value, lower, open, false, fastBoundReason{kind: fastReasonAxiom})
}

func (c *FastContext) publishBound(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open, enqueue bool, reason fastBoundReason) (err error) {
	if err = c.checkNode(n, variable); err != nil {
		return err
	}
	if n.inconsistent {
		return arena.ErrInvalidSlot
	}
	c.numBounds++
	m := c.numbers
	copyOwner, err := numeral.NewScopedFastFloat(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, copyOwner.Close()) }()
	copyValue, err := copyOwner.Get()
	if err != nil {
		return err
	}
	if err = m.Copy(copyValue, value); err != nil {
		return err
	}
	if err = c.NormalizeBound(variable, copyValue, lower, &open); err != nil {
		return err
	}
	if err = n.PushBound(variable, copyValue, lower, open); err != nil {
		return err
	}
	b := n.trail
	b.timestamp = c.timestamp
	b.decision = !enqueue
	b.reason = reason
	if err = c.traceNewBound(b); err != nil {
		return err
	}
	conflict, e := n.ConflictingBounds(variable)
	if e != nil {
		return e
	}
	if conflict {
		if err = c.traceBoundConflict(n); err != nil {
			return err
		}
		n.inconsistent, n.conflictVar = true, variable
		c.numConflicts++
		c.removeLeaf(n)
	}
	c.timestamp++
	if c.timestamp == math.MaxUint64 {
		return ErrFastContextTimestamp
	}
	if enqueue {
		c.queue = append(c.queue, b)
	}
	return nil
}

// PropagateCandidate is the source definition-propagation sequence: normalize
// a scratch candidate, check its relevance, then publish it when selected.
func (c *FastContext) PropagateCandidate(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open bool) (published bool, err error) {
	return c.propagateCandidateReason(n, variable, value, lower, open, fastBoundReason{})
}

func (c *FastContext) propagateCandidateReason(n *FastNode, variable uint32, value *numeral.FastFloat, lower, open bool, reason fastBoundReason) (published bool, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return false, err
	}
	scratchOwner, err := numeral.NewScopedFastFloat(c.numbers)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, scratchOwner.Close()) }()
	scratch, err := scratchOwner.Get()
	if err != nil {
		return false, err
	}
	if err = c.numbers.Copy(scratch, value); err != nil {
		return false, err
	}
	if err = c.NormalizeBound(variable, scratch, lower, &open); err != nil {
		return false, err
	}
	published, err = c.RelevantNewBound(n, variable, scratch, lower, open)
	if err != nil || !published {
		return published, err
	}
	return true, c.propagateBoundReason(n, variable, scratch, lower, open, reason)
}
