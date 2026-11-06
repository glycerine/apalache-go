package realclosure

// Source: src/math/realclosure/realclosure.cpp:233-299, 1808-1835, and
// manager-facing extension queries.

import "github.com/glycerine/g3/pkg/arena"

func (r *algebraicRoot) registryIndex() (int, error) {
	if !r.isRegistered() {
		return 0, arena.ErrInvalidSlot
	}
	return r.index, nil
}

func (r *algebraicRoot) definingDegree() (int, error) {
	if r == nil || r.kind != realExtensionAlgebraic {
		return 0, arena.ErrInvalidSlot
	}
	if _, err := r.registryIndex(); err != nil {
		return 0, err
	}
	return len(r.polynomial.coefficients) - 1, nil
}

func (r *algebraicRoot) signConditionIndex() (int, bool, error) {
	if r == nil || r.kind != realExtensionAlgebraic {
		return 0, false, arena.ErrInvalidSlot
	}
	if _, err := r.registryIndex(); err != nil {
		return 0, false, err
	}
	if r.signs == nil {
		return -1, false, nil
	}
	return r.signIndex, true, nil
}

func (r *algebraicRoot) cloneDefiningInterval() (*BinaryInterval, error) {
	if _, err := r.registryIndex(); err != nil {
		return nil, err
	}
	if r.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	// Every retained root reference is currently held by a numerator-x value.
	// Find that value only through the caller in normal use; root metadata itself
	// returns its owned current enclosure without mutating it.
	result, err := NewBinaryInterval(r.owner.intervals)
	if err != nil {
		return nil, err
	}
	if err = result.SetFrom(r.interval); err != nil {
		_ = result.Close()
		return nil, err
	}
	return result, nil
}
