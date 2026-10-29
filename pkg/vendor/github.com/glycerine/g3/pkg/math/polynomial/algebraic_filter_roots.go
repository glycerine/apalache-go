package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2212-2231,2234-2266.

import "github.com/glycerine/g3/pkg/arena"

// The retained ext_var2num overrides one borrowed assignment cell. A matching
// variable never touches the underlying mapping, including its Contains hook.
type algebraicExtendedAssignment struct {
	base  PolynomialAlgebraicEvaluation
	x     uint32
	value *Algebraic
}

func (v *algebraicExtendedAssignment) Contains(x uint32) (bool, error) {
	if x == v.x {
		return true, nil
	}
	return v.base.Contains(x)
}
func (v *algebraicExtendedAssignment) Algebraic(x uint32) (*Algebraic, error) {
	if x == v.x {
		return v.value, nil
	}
	return v.base.Algebraic(x)
}

// filterRoots preserves source candidate order. A failed checkpoint or sign
// evaluation leaves the already copied prefix and the original vector length;
// suffix destruction and shrink are reached only after every candidate.
func (m *AlgebraicRootCore) filterRoots(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, x uint32, roots *AlgebraicVector) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.filterRootsRunning {
		return ErrAlgebraicPending
	}
	if pm == nil || values == nil || roots == nil {
		return arena.ErrConfig
	}
	if roots.core != m {
		return ErrAlgebraicOwner
	}
	if e := roots.check(); e != nil {
		return e
	}
	m.filterRootsRunning = true
	defer func() { m.filterRootsRunning = false }()
	if e := m.traceFilterRoots(roots, x, false); e != nil {
		return e
	}
	size := len(roots.values)
	j := 0
	for i := 0; i < size; i++ {
		if e := m.checkpoint(); e != nil {
			return e
		}
		extended := algebraicExtendedAssignment{base: values, x: x, value: &roots.values[i]}
		if e := m.traceFilterRootCandidate(uint32(i), x, &roots.values[i]); e != nil {
			return e
		}
		sign, e := m.EvalSign(pm, p, &extended)
		if e != nil {
			return e
		}
		if e = m.traceFilterRootSign(uint32(i), sign); e != nil {
			return e
		}
		if sign != 0 {
			continue
		}
		if i != j {
			if e = m.Copy(&roots.values[j], &roots.values[i]); e != nil {
				return e
			}
		}
		j++
	}
	for i := j; i < size; i++ {
		if e := m.Del(&roots.values[i]); e != nil {
			return e
		}
	}
	roots.values = roots.values[:j]
	return m.traceFilterRoots(roots, x, true)
}
