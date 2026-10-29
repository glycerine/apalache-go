package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2268-2451,2879-2881.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// IsolateMultivariateRoots isolates in the one variable left unassigned by
// values. Rational assignments are substituted first; algebraic assignments
// are eliminated by their actual defining polynomials and roots are filtered
// at the original assignment. Output mutation is incremental, as in C++.
// After an error, call CleanupRetained and reset any partial output before retry.
func (m *AlgebraicRootCore) IsolateMultivariateRoots(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector) error {
	return m.isolateMultivariateRootsEntry(pm, p, values, roots, nil)
}
func (m *AlgebraicRootCore) isolateMultivariateRootsEntry(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector, caller *algebraicIsolationSignsScope) error {
	if m != nil && m.self == m && m.rootConstruction.active {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.isolationSigns.active && caller != &m.isolationSigns {
		return ErrAlgebraicPending
	}
	if e := m.check(); e != nil {
		return e
	}
	if pm == nil || values == nil || roots == nil {
		return arena.ErrConfig
	}
	if e := pm.ready(); e != nil {
		return e
	}
	if roots.core != m {
		return ErrAlgebraicOwner
	}
	if e := roots.check(); e != nil {
		return e
	}
	if m.multivariateIsolationRunning {
		return ErrAlgebraicPending
	}
	for _, s := range m.multivariateIsolationFrames {
		if s.active {
			return ErrAlgebraicPending
		}
	}
	m.multivariateIsolationRunning = true
	defer func() { m.multivariateIsolationRunning = false }()
	return m.isolateMultivariateRoots(pm, p, values, roots, false)
}
func (m *AlgebraicRootCore) isolateMultivariateRoots(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector, nested bool) (cause error) {
	if e := m.traceMultivariateIsolationPolynomial(pm, p, 2288, "isolating roots of: "); e != nil {
		return e
	}
	if e := algebraicIsolationAssert(len(roots.values) == 0, 2289, "roots.empty()"); e != nil {
		return e
	}
	zero, e := pm.core.IsZero(p)
	if e != nil {
		return e
	}
	constant := false
	if !zero {
		constant, e = pm.core.IsConstant(p)
		if e != nil {
			return e
		}
	}
	if zero || constant {
		return m.traceMultivariateIsolationLiteral(2292, "p is zero or the constant polynomial\n")
	}
	univariate, e := pm.core.IsUnivariate(p)
	if e != nil {
		return e
	}
	if univariate {
		if e = m.traceMultivariateIsolationLiteral(2297, "p is univariate, using univariate procedure\n"); e != nil {
			return e
		}
		return m.isolatePolynomialRootsForMultivariate(pm, p, roots, nil)
	}
	s, e := m.newMultivariateIsolationFrame(pm)
	if e != nil {
		return e
	}
	s.assignments = values
	defer func() { m.multivariateIsolationDepth--; cause = s.finishBody(cause) }()
	// p_prime is the oldest lexical local. Reuse the concrete rational fragment
	// substitution owner, independently of the later EvalSign filter invocation.
	s.substitution = algebraicSignSubstitutionScope{active: true, prime: PolynomialRef{owner: pm.core}}
	s.substitution.prime.self = &s.substitution.prime
	s.stack[s.depth] = 12
	s.depth++
	basic := algebraicBasicRationals{algebraicOptimisticRationals: algebraicOptimisticRationals{owner: m, values: values}}
	prime, e := pm.core.substituteRationalForAlgebraic(p, &basic, &s.substitution)
	if e != nil {
		return e
	}
	if e = s.substitution.prime.Assign(prime); e != nil {
		return e
	}
	if e = m.traceMultivariateIsolationPolynomial(pm, prime, 2306, "p after applying (rational fragment of) x2v:\n"); e != nil {
		return e
	}
	zero, e = pm.core.IsZero(prime)
	if e != nil {
		return e
	}
	constant = false
	if !zero {
		constant, e = pm.core.IsConstant(prime)
		if e != nil {
			return e
		}
	}
	if zero || constant {
		return m.traceMultivariateIsolationLiteral(2309, "p is zero or the constant polynomial after applying (rational fragment of) x2v\n")
	}
	univariate, e = pm.core.IsUnivariate(prime)
	if e != nil {
		return e
	}
	if univariate {
		x, e := pm.core.MaxVariable(prime)
		if e != nil {
			return e
		}
		assigned, e := values.Contains(x)
		if e != nil || assigned {
			return e
		}
		if e = m.traceMultivariateIsolationLiteral(2320, "p is univariate after applying (rational fragment of) x2v... using univariate procedure\n"); e != nil {
			return e
		}
		return m.isolatePolynomialRootsForMultivariate(pm, prime, roots, s)
	}
	xs := &m.isolateRootsVariables
	if !xs.constructed {
		variableEmbedded(xs, pm.core.memory, true)
	} else {
		xs.runtime = pm.core.memory
	}
	if e = pm.Vars(prime, xs); e != nil {
		return e
	}
	if e = algebraicIsolationAssert(len(xs.values) > 1, 2328, "xs.size() > 1"); e != nil {
		return e
	}
	var sortError error
	sort.SliceStable(xs.values, func(i, j int) bool {
		if sortError != nil {
			return false
		}
		a, e := m.signVariableDegree(values, xs.values[i])
		if e != nil {
			sortError = e
			return false
		}
		b, e := m.signVariableDegree(values, xs.values[j])
		if e != nil {
			sortError = e
			return false
		}
		return a < b
	})
	if sortError != nil {
		return sortError
	}
	if len(xs.values) == 0 {
		return arena.ErrBounds
	}
	if e = m.traceMultivariateIsolationVariableCount(uint32(len(xs.values) - 1)); e != nil {
		return e
	}
	x := xs.values[len(xs.values)-1]
	assigned, e := values.Contains(x)
	if e != nil {
		return e
	}
	if assigned {
		if polynomialSourceDebug {
			for _, y := range xs.values {
				d, enabled, e := algebraicAssertionSwitch()
				if e != nil {
					return e
				}
				if enabled {
					has, e := values.Contains(y)
					if e != nil {
						return e
					}
					if !has {
						if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2341, "x2v.contains(xs[i])"); e != nil {
							return e
						}
					}
				}
			}
		}
		return nil
	}
	s.constructRef(0, pm)
	if e = s.refs[0].Assign(prime); e != nil {
		return e
	}
	s.constructRef(1, pm)
	for i := 0; i < len(xs.values)-1; i++ {
		if e = m.checkpoint(); e != nil {
			return e
		}
		y := xs.values[i]
		a, e := m.isolationAssignedRoot(values, y)
		if e != nil {
			return e
		}
		l, e := m.location(a)
		if e != nil {
			return e
		}
		if l == nil || l.kind != 1 {
			return ErrAlgebraicOwner
		}
		record, e := m.load(l)
		if e != nil {
			return e
		}
		descriptor, e := m.config.Polynomials.descriptor(record.poly)
		if e != nil {
			return e
		}
		py, e := pm.toPolynomialWithReceipt(record.size, m.config.Polynomials.values(&descriptor.state), y, &s.toPolynomial)
		if e != nil {
			return e
		}
		if e = s.refs[1].Assign(py); e != nil {
			return e
		}
		if e = pm.resultantWithReceipt(s.refs[0].root.value, py, y, &s.refs[0].root, &s.resultant); e != nil {
			return e
		}
		if e = m.traceMultivariateIsolationResultant(pm, uint32(i), y, py, s.refs[0].root.value); e != nil {
			return e
		}
		zero, e = pm.core.IsZero(s.refs[0].root.value)
		if e != nil {
			return e
		}
		if zero {
			if e = algebraicIsolationAssert(!nested, 2363, "!nested_call"); e != nil {
				return e
			}
			break
		}
	}
	q := s.refs[0].root.value
	zero, e = pm.core.IsZero(q)
	if e != nil {
		return e
	}
	if zero {
		if e = m.traceMultivariateIsolationLiteral(2369, "q vanished\n"); e != nil {
			return e
		}
		return m.isolateVanishedResultant(s, prime, values, x, roots, nested)
	}
	constant, e = pm.core.IsConstant(q)
	if e != nil {
		return e
	}
	if constant {
		return m.traceMultivariateIsolationLiteral(2443, "q is the constant polynomial, so p does not contain any roots at x2v\n")
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		yes, e := pm.core.IsUnivariate(q)
		if e != nil {
			return e
		}
		if !yes {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2446, "is_univariate(q)"); e != nil {
				return e
			}
		}
	}
	if e = m.isolatePolynomialRootsForMultivariate(pm, q, roots, s); e != nil {
		return e
	}
	return m.filterRoots(pm, prime, values, x, roots)
}
func (m *AlgebraicRootCore) isolationAssignedRoot(values PolynomialAlgebraicEvaluation, y uint32) (*Algebraic, error) {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return nil, e
	}
	if enabled {
		has, e := values.Contains(y)
		if e != nil {
			return nil, e
		}
		if !has {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2354, "x2v.contains(y)"); e != nil {
				return nil, e
			}
		}
	}
	a, e := values.Algebraic(y)
	if e != nil {
		return nil, e
	}
	if e = m.assertBinaryRoot(a, 2356, "!v.is_basic()"); e != nil {
		return nil, e
	}
	return a, nil
}
func (m *AlgebraicRootCore) isolateVanishedResultant(s *algebraicMultivariateIsolationFrame, prime Polynomial, values PolynomialAlgebraicEvaluation, x uint32, roots *AlgebraicVector, nested bool) error {
	if e := algebraicIsolationAssert(!nested, 2375, "!nested_call"); e != nil {
		return e
	}
	pm := s.pm
	n, e := pm.core.Degree(prime, x)
	if e != nil {
		return e
	}
	if e = algebraicIsolationAssert(n > 0, 2377, "n > 0"); e != nil {
		return e
	}
	if n == 1 {
		if e = m.traceMultivariateIsolationLiteral(2380, "p is linear after applying (rational fragment) of x2v\n"); e != nil {
			return e
		}
		s.constructRef(2, pm)
		s.constructRef(3, pm)
		c0, e := pm.core.coeffWithReceipts(prime, x, 0, &s.coefficient, &s.append, &s.factory)
		if e != nil {
			return e
		}
		if e = s.refs[2].root.assignWithFactoryReceipt(pm.core, c0, &s.factory); e != nil {
			return e
		}
		c1, e := pm.core.coeffWithReceipts(prime, x, 1, &s.coefficient, &s.append, &s.factory)
		if e != nil {
			return e
		}
		if e = s.refs[3].root.assignWithFactoryReceipt(pm.core, c1, &s.factory); e != nil {
			return e
		}
		s.constructValue(0)
		s.constructValue(1)
		if e = s.evaluate(c0, &s.values[0]); e != nil {
			return e
		}
		if e = s.evaluate(c1, &s.values[1]); e != nil {
			return e
		}
		zero, e := m.IsZero(&s.values[1])
		if e != nil {
			return e
		}
		if zero {
			return m.traceMultivariateIsolationLiteral(2391, "coefficient of degree 1 vanished, so p does not have roots at x2v\n")
		}
		root := Algebraic{}
		if e = roots.pushTake(&root); e != nil {
			return e
		}
		previous := m.polynomialCaller
		m.polynomialCaller = &s.calls
		e = m.Div(&s.values[0], &s.values[1], &roots.values[0])
		m.polynomialCaller = previous
		if e != nil {
			return e
		}
		if e = m.negWithReceipt(&roots.values[0], &s.neg); e != nil {
			return e
		}
		return m.traceMultivariateIsolationLinearRoot(&roots.values[0])
	}
	s.constructRef(2, pm)
	s.constructValue(0)
	// Source uses a signed int cursor, with the same conversion from unsigned n.
	i := int32(n)
	for ; i >= 1; i-- {
		c, e := pm.core.coeffWithReceipts(prime, x, uint32(i), &s.coefficient, &s.append, &s.factory)
		if e != nil {
			return e
		}
		if e = s.refs[2].root.assignWithFactoryReceipt(pm.core, c, &s.factory); e != nil {
			return e
		}
		if e = s.evaluate(c, &s.values[0]); e != nil {
			return e
		}
		zero, e := m.IsZero(&s.values[0])
		if e != nil {
			return e
		}
		if !zero {
			break
		}
	}
	if i == 0 {
		return m.traceMultivariateIsolationLiteral(2413, "all coefficients vanished... polynomial does not have roots\n")
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		zero, e := m.IsZero(&s.values[0])
		if e != nil {
			return e
		}
		if zero {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 2416, "!is_zero(a)"); e != nil {
				return e
			}
		}
	}
	xs := m.isolateRootsVariables.values
	if e = algebraicIsolationAssert(len(xs) > 0, 2270, "!xs.empty()"); e != nil {
		return e
	}
	if len(xs) == 0 {
		return arena.ErrBounds
	}
	max := xs[0]
	for _, y := range xs[1:] {
		if y > max {
			max = y
		}
	}
	if max >= math.MaxUint32-1 {
		return arena.ErrBounds
	}
	z := max + 1
	if e = m.ensureMultivariate(); e != nil {
		return e
	}
	local := m.multivariate
	for local.core.monomials.nextVar < z+1 {
		if _, e = local.core.monomials.MkVar(); e != nil {
			return e
		}
	}
	if e = algebraicIsolationAssert(local.core.monomials.nextVar >= z+1, 2283, "pm().num_vars() >= sz"); e != nil {
		return e
	}
	s.constructRef(4, local)
	s.constructRef(5, local)
	s.constructRef(6, local)
	s.constructRef(7, local)
	if e = algebraicIsolationAssert(i >= 1, 2429, "i >= 1"); e != nil {
		return e
	}
	q2, e := convertPolynomialForIsolation(pm, prime, local, x, uint32(i-1), nil, s)
	if e != nil {
		return e
	}
	if e = s.refs[4].Assign(q2); e != nil {
		return e
	}
	xi, e := local.core.mkVariableWithReceipt(x, uint32(i), &s.variables[0])
	if e != nil {
		return e
	}
	if e = s.refs[6].Assign(xi); e != nil {
		return e
	}
	zp, e := local.core.mkVariableWithReceipt(z, 1, &s.variables[1])
	if e != nil {
		return e
	}
	if e = s.refs[5].Assign(zp); e != nil {
		return e
	}
	s.constructRef(8, local)
	product, e := local.core.mulWithReceipt(zp, xi, &s.arithmetic[0])
	if e != nil {
		return e
	}
	if e = s.refs[8].Assign(product); e != nil {
		return e
	}
	combined, e := local.core.addWithReceipt(product, q2, &s.arithmetic[1])
	if e != nil {
		return e
	}
	if e = s.refs[4].Assign(combined); e != nil {
		return e
	}
	if e = s.refs[8].root.closeValue(local.core, false); e != nil {
		return e
	}
	s.depth--
	if e = m.traceMultivariateIsolationRecovery(local, combined, z, &s.values[0]); e != nil {
		return e
	}
	extended := algebraicExtendedAssignment{base: values, x: z, value: &s.values[0]}
	return m.isolateMultivariateRoots(local, combined, &extended, roots, true)
}
