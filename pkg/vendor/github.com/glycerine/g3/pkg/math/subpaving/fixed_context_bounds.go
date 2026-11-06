package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:528-589,1176-1315,
// concrete config_mpfx bound normalization and relevance.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *FixedContext) checkNode(n *FixedNode, variable uint32) error {
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
func (c *FixedContext) NormalizeBound(variable uint32, value *numeral.FixedPoint, lower bool, open *bool) error {
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
	integer, err := m.IsInt(value)
	if err != nil {
		return err
	}
	if !integer {
		*open = false
	}
	if lower {
		err = m.Ceil(value)
	} else {
		err = m.Floor(value)
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

// The source checks strict order first and calls eq only when openness makes
// equality relevant. Keep that order for the concrete fixed-point manager.
func fixedGreaterOrEqualWhen(m *numeral.FixedPointManager, a, b *numeral.FixedPoint, equalMatters bool) (bool, error) {
	greater, err := m.Greater(a, b)
	if err != nil || greater || !equalMatters {
		return greater, err
	}
	return m.Equal(a, b)
}

func fixedLessOrEqualWhen(m *numeral.FixedPointManager, a, b *numeral.FixedPoint, equalMatters bool) (bool, error) {
	less, err := m.Less(a, b)
	if err != nil || less || !equalMatters {
		return less, err
	}
	return m.Equal(a, b)
}

// RelevantNewBound follows the source conflict-first, zero-epsilon, threshold
// and epsilon-gap branches. Arithmetic exceptions mark the context failed and
// refuse the candidate, matching the source catch block.
func (c *FixedContext) RelevantNewBound(n *FixedNode, variable uint32, k *numeral.FixedPoint, lower, open bool) (relevant bool, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return false, err
	}
	m := c.numbers
	if _, err = m.IsZero(k); err != nil {
		return false, err
	}
	defer func() {
		if errors.Is(err, numeral.ErrFixedPointDomain) {
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
			conflict, e := fixedGreaterOrEqualWhen(m, k, currentUpper.value, open || currentUpper.open)
			if e != nil {
				return false, e
			}
			if conflict {
				return c.traceRelevantDecision(true, 1218, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentLower != nil {
			irrelevant, e := fixedLessOrEqualWhen(m, k, currentLower.value, currentLower.open || !open)
			if e != nil {
				return false, e
			}
			if irrelevant {
				return c.traceRelevantDecision(false, 1224, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentUpper == nil {
			cmp, e := m.Compare(c.maxBound, k)
			if e != nil {
				return false, e
			}
			if cmp < 0 {
				return c.traceRelevantDecision(false, 1229, "irrelevant because exceeds :max-bound threshold.", nil)
			}
		}
	} else {
		if currentLower != nil {
			conflict, e := fixedGreaterOrEqualWhen(m, currentLower.value, k, open || currentLower.open)
			if e != nil {
				return false, e
			}
			if conflict {
				return c.traceRelevantDecision(true, 1268, "relevant because triggers conflict.", nil)
			}
		}
		if c.zeroEpsilon && currentUpper != nil {
			irrelevant, e := fixedLessOrEqualWhen(m, currentUpper.value, k, currentUpper.open || !open)
			if e != nil {
				return false, e
			}
			if irrelevant {
				return c.traceRelevantDecision(false, 1274, "irrelevant because does not improve existing bound.", nil)
			}
		}
		if currentLower == nil {
			cmp, e := m.Compare(k, c.minusMaxBound)
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
	minimum, absolute, e := fixedScratchPair(m)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, minimum.Close(), absolute.Close()) }()
	delta, e := m.New()
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, delta.Close()) }()
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
		cmp, e := m.Compare(absolute, minimum)
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
	if err = m.SetInt64(delta, 1); err != nil {
		return false, err
	}
	cmp, e := m.Compare(minimum, delta)
	if e != nil {
		return false, e
	}
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
		cmp, e = m.Compare(k, delta)
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
	cmp, e = m.Compare(k, delta)
	if e != nil {
		return false, e
	}
	if cmp >= 0 {
		return c.traceRelevantDecision(false, 1304, "irrelevant because does not improve existing bound to at least ", delta)
	}
	return c.traceRelevantDecision(true, 1310, "new bound is relevant", nil)
}

// PropagateBound creates the normalized bound, queues it, then checks the
// node's new interval for conflict. Relevance testing is the caller's choice,
// as source unit clauses publish directly.
func (c *FixedContext) PropagateBound(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open bool) (err error) {
	return c.propagateBoundReason(n, variable, value, lower, open, fixedBoundReason{})
}

func (c *FixedContext) propagateBoundReason(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open bool, reason fixedBoundReason) error {
	return c.publishBound(n, variable, value, lower, open, true, reason)
}

// DecideBound creates the source's branch decision without putting it on the
// propagation queue. AddRecentBounds queues surviving child decisions later.
func (c *FixedContext) DecideBound(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open bool) error {
	if n == nil || n.parent == nil {
		return arena.ErrInvalidSlot
	}
	return c.publishBound(n, variable, value, lower, open, false, fixedBoundReason{kind: fixedReasonAxiom})
}

func (c *FixedContext) publishBound(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open, enqueue bool, reason fixedBoundReason) (err error) {
	if err = c.checkNode(n, variable); err != nil {
		return err
	}
	if n.inconsistent {
		return arena.ErrInvalidSlot
	}
	m := c.numbers
	copyValue, err := m.New()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, copyValue.Close()) }()
	if err = m.Copy(copyValue, value); err != nil {
		return err
	}
	if err = c.NormalizeBound(variable, copyValue, lower, &open); err != nil {
		return err
	}
	c.numBounds++
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
		c.removeFixedLeaf(n)
	}
	c.timestamp++
	if c.timestamp == math.MaxUint64 {
		return ErrFixedContextTimestamp
	}
	if enqueue {
		c.queue = append(c.queue, b)
	}
	return nil
}

// PropagateCandidate is the source definition-propagation sequence: normalize
// a scratch candidate, check its relevance, then publish it when selected.
func (c *FixedContext) PropagateCandidate(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open bool) (published bool, err error) {
	return c.propagateCandidateReason(n, variable, value, lower, open, fixedBoundReason{})
}

func (c *FixedContext) propagateCandidateReason(n *FixedNode, variable uint32, value *numeral.FixedPoint, lower, open bool, reason fixedBoundReason) (published bool, err error) {
	if err = c.checkNode(n, variable); err != nil {
		return false, err
	}
	scratch, err := c.numbers.New()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, scratch.Close()) }()
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
