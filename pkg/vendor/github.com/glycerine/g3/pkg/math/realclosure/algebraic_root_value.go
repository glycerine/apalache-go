package realclosure

// Source: src/math/realclosure/realclosure.cpp:1244-1275, 767-770,
// and 1829-1835.

import "github.com/glycerine/g3/pkg/arena"

// newAlgebraicValue creates the rational-function value representing one
// algebraic extension: numerator x, implicit denominator one, and the
// extension's current approximation interval.
func (s *realValueStore) newAlgebraicValue(root *algebraicRoot, one *realValue) (*realValue, error) {
	if s == nil || root == nil || root.owner == nil || root.owner.values != s || root.deleted || one == nil || one.store != s || one.deleted || !one.isRational {
		return nil, arena.ErrInvalidSlot
	}
	isOne, err := s.numbers.IsOne(&one.rational)
	if err != nil {
		return nil, err
	}
	if !isOne {
		return nil, arena.ErrInvalidSlot
	}
	if err := root.retain(); err != nil {
		return nil, err
	}
	v := &realValue{store: s, root: root, refs: 1}
	if err := s.setPolynomial(&v.functionNumerator, []*realValue{nil, one}); err != nil {
		_ = root.release()
		return nil, err
	}
	v.functionInterval, err = NewBinaryInterval(root.owner.intervals)
	if err != nil {
		_ = s.resetPolynomial(&v.functionNumerator)
		_ = root.release()
		return nil, err
	}
	if err := v.functionInterval.SetFrom(root.interval); err != nil {
		_ = v.functionInterval.Close()
		_ = s.resetPolynomial(&v.functionNumerator)
		_ = root.release()
		return nil, err
	}
	return v, nil
}
