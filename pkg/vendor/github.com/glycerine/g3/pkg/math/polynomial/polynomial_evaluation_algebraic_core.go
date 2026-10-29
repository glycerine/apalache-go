package polynomial

// Source: src/math/polynomial/polynomial.cpp:6436-6506. Concrete algebraic instantiation.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MultivariatePolynomialManager) evaluationAlgebraicCore(p Polynomial, v PolynomialAlgebraicEvaluation, start, end, x uint32, out *Algebraic) (cause error) {
	if e := m.traceEvaluationEntry(p, start, end, x, &m.algebraicEvaluation.trace, true); e != nil {
		return e
	}
	debugOwner, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && start >= end {
		return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6440, "start < end")
	}
	debugOwner, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		n, e := m.core.readWord(p, 8)
		if e != nil {
			return e
		}
		if end > n {
			return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6441, "end <= p->size()")
		}
	}
	debugOwner, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		valid, e := m.core.monomials.IsValidVar(x)
		if e != nil {
			return e
		}
		if !valid {
			return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6442, "is_valid(x)")
		}
	}
	f, e := m.evaluationAlgebraicFrame()
	if e != nil {
		return e
	}
	defer func() {
		m.algebraicEvaluation.depth--
		if cause != nil {
			if m.algebraicEvaluation.bodyCause == nil {
				m.algebraicEvaluation.bodyCause = cause
			}
		}
		e := m.finishEvaluationAlgebraicFrame(f, false)
		if e != nil {
			m.algebraicEvaluation.sourceCause = e
			cause = errors.Join(cause, e)
		}
	}()
	defer func() {
		if cause == nil {
			cause = m.traceEvaluationAlgebraicResult(start, end, x, out)
		}
	}()

	aux := &f.aux
	if end == start+1 {
		n, e := m.core.mutableCoefficientCore(p, start)
		if e != nil {
			return e
		}
		if e = m.evaluationAlgebraicCall(polynomialEvalSet, nil, nil, &n, 0, out); e != nil {
			return e
		}
		term, e := m.core.monomialCore(p, start)
		if e != nil {
			return e
		}
		debugOwner, enabled, e := algebraicAssertionSwitch()
		if e != nil {
			return e
		}
		if enabled {
			d, e := m.core.monomials.DegreeOf(term, x)
			if e != nil {
				return e
			}
			if d == 0 {
				return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6447, "m->degree_of(x) > 0")
			}
		}
		sz, e := m.core.monomials.sizeCore(term)
		if e != nil {
			return e
		}
		for i := uint32(0); i < sz; i++ {
			y, e := m.core.monomials.varCore(term, i)
			if e != nil {
				return e
			}
			if y > x {
				break
			}
			debugOwner, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled {
				ok, e := v.Contains(y)
				if e != nil {
					return e
				}
				if !ok {
					return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6453, "x2v.contains(y)")
				}
			}
			d, e := m.core.monomials.degreeAtCore(term, i)
			if e != nil {
				return e
			}
			value, e := v.Algebraic(y)
			if e != nil {
				return e
			}
			if e = m.evaluationAlgebraicCall(polynomialEvalPower, value, nil, nil, d, aux); e != nil {
				return e
			}
			if e = m.evaluationAlgebraicCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
				return e
			}
		}
		return nil
	}
	debugOwner, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		ok, e := v.Contains(x)
		if e != nil {
			return e
		}
		if !ok {
			return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6460, "x2v.contains(x)")
		}
	}
	value, e := v.Algebraic(x)
	if e != nil {
		return e
	}
	if e = m.evaluationAlgebraicCall(polynomialEvalReset, nil, nil, nil, 0, out); e != nil {
		return e
	}
	for i := start; i < end; {
		if e = m.core.checkpoint(); e != nil {
			return e
		}
		d, e := m.evaluationDegree(p, i, x)
		if e != nil {
			return e
		}
		if d == 0 {
			y, e := m.evaluationMaxSmaller(p, i, end, x)
			if e != nil {
				return e
			}
			if y == ^uint32(0) {
				debugOwner, enabled, e := algebraicAssertionSwitch()
				if e != nil {
					return e
				}
				if enabled && end != i+1 {
					return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6470, "end == i+1")
				}
				n, e := m.core.mutableCoefficientCore(p, i)
				if e != nil {
					return e
				}
				return m.evaluationAlgebraicCall(polynomialEvalAddInteger, out, nil, &n, 0, out)
			}
			if e = m.evaluationAlgebraicCore(p, v, i, end, y, aux); e != nil {
				return e
			}
			return m.evaluationAlgebraicCall(polynomialEvalAdd, out, aux, nil, 0, out)
		}
		j, next := i+1, uint32(0)
		for ; j < end; j++ {
			dj, e := m.evaluationDegree(p, j, x)
			if e != nil {
				return e
			}
			debugOwner, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled && dj > d {
				return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6483, "d_j <= d")
			}
			if dj < d {
				next = dj
				break
			}
		}
		debugOwner, enabled, e := algebraicAssertionSwitch()
		if e != nil {
			return e
		}
		if enabled && j != end {
			dj, e := m.evaluationDegree(p, j, x)
			if e != nil {
				return e
			}
			if dj >= d {
				return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6489, "j == end || p->m(j)->degree_of(x) < d")
			}
		}
		y, e := m.evaluationMaxSmaller(p, i, j, x)
		if e != nil {
			return e
		}
		if y == ^uint32(0) {
			debugOwner, enabled, e := algebraicAssertionSwitch()
			if e != nil {
				return e
			}
			if enabled && j != i+1 {
				return diagnostic.DebugAssertionFailure(debugOwner, "src/math/polynomial/polynomial.cpp", 6492, "j == i+1")
			}
			n, e := m.core.mutableCoefficientCore(p, i)
			if e != nil {
				return e
			}
			if e = m.evaluationAlgebraicCall(polynomialEvalSet, nil, nil, &n, 0, aux); e != nil {
				return e
			}
		} else if e = m.evaluationAlgebraicCore(p, v, i, j, y, aux); e != nil {
			return e
		}
		if e = m.evaluationAlgebraicCall(polynomialEvalAdd, out, aux, nil, 0, out); e != nil {
			return e
		}
		if e = m.evaluationAlgebraicCall(polynomialEvalPower, value, nil, nil, d-next, aux); e != nil {
			return e
		}
		if e = m.evaluationAlgebraicCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
			return e
		}
		i = j
	}
	return nil
}
