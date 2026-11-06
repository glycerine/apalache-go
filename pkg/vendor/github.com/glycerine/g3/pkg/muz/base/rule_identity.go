package base

// Source: src/muz/base/dl_rule.h rule_eq_proc/rule_hash_proc and
// src/muz/base/dl_rule.cpp rule_eq_proc::operator(), rule::hash.

import (
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

// Equal reports source structural rule equality. Names and accounting state do
// not participate; head, ordered tails, and predicate-tail polarity do.
func (r *Rule) Equal(other *Rule) (bool, error) {
	left, err := r.open()
	if err != nil {
		return false, err
	}
	right, err := other.open()
	if err != nil {
		return false, err
	}
	if left.manager != right.manager || left.head != right.head || len(left.tail) != len(right.tail) {
		return false, nil
	}
	for i := range left.tail {
		if left.tail[i].handle != right.tail[i].handle || left.tail[i].negated != right.tail[i].negated {
			return false, nil
		}
	}
	return true, nil
}

// Hash returns the source rule hash over the structural equality fields.
func (r *Rule) Hash() (uint32, error) {
	state, err := r.open()
	if err != nil {
		return 0, err
	}
	result, err := state.manager.GetHash(state.head)
	if err != nil {
		return 0, err
	}
	for _, tail := range state.tail {
		h, err := state.manager.GetHash(tail.handle)
		if err != nil {
			return 0, err
		}
		polarity := uint32(0)
		if tail.negated {
			polarity = 1
		}
		result = uhash.Combine(result, uhash.Combine(h, polarity))
	}
	return result, nil
}

// BasicRuleSubsumes is the deliberately cheap source context check: heads
// must be identical and every stronger-rule tail expression must occur in the
// weaker rule. Like the source, polarity does not participate in this check.
func BasicRuleSubsumes(stronger, weaker *Rule) (bool, error) {
	strongerState, err := stronger.open()
	if err != nil {
		return false, err
	}
	weakerState, err := weaker.open()
	if err != nil {
		return false, err
	}
	if strongerState.manager != weakerState.manager || strongerState.head != weakerState.head {
		return false, nil
	}
	for _, candidate := range strongerState.tail {
		found := false
		for _, other := range weakerState.tail {
			if candidate.handle == other.handle {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}
