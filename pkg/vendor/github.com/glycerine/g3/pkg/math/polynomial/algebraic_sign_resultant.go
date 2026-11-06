package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2106-2188,2195-2211.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicSignResultantScope struct {
	refs              [4]PolynomialRef // yp, p_prime_aux, R, q_i
	constructed       uint8
	variable          polynomialVariableInvocationReceipt
	arithmetic        polynomialArithmeticInvocationReceipt
	toPolynomial      polynomialToPolynomialInvocationReceipt
	resultant         polynomialGCDInvocationReceipt
	dense             polynomialFactorNumeralVectorReceipt
	bound             upolyKnuthInvocationReceipt
	conversionManager *MultivariatePolynomialManager
	limits            [2]unumeral.BinaryRational // mL, L
	limitsConstructed uint8
	saved             []algebraicSavedInterval
	savedCursor       int
	active, running   bool
	sourceStop        error
}

func (m *AlgebraicRootCore) signRoot(values PolynomialAlgebraicEvaluation, x uint32, containsLine, rootLine int32) (*Algebraic, error) {
	containsExpression, rootExpression := "x2v.contains(x_i)", "!v_i.is_basic()"
	if containsLine == 2174 {
		containsExpression, rootExpression = "x2v.contains(x)", "!v.is_basic()"
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return nil, e
	}
	if enabled {
		has, e := values.Contains(x)
		if e != nil {
			return nil, e
		}
		if !has {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", containsLine, containsExpression); e != nil {
				return nil, e
			}
		}
	}
	a, e := values.Algebraic(x)
	if e != nil {
		return nil, e
	}
	if e = m.assertBinaryRoot(a, rootLine, rootExpression); e != nil {
		return nil, e
	}
	return a, nil
}
func (m *AlgebraicRootCore) signVariableDegree(values PolynomialAlgebraicEvaluation, x uint32) (uint32, error) {
	has, e := values.Contains(x)
	if e != nil {
		return 0, e
	}
	if !has {
		return math.MaxUint32, nil
	}
	a, e := values.Algebraic(x)
	if e != nil {
		return 0, e
	}
	return m.Degree(a)
}
func (m *AlgebraicRootCore) signIntervalInsideLimits() (bool, error) {
	s := &m.signResultant
	b := m.config.Binary
	i := &m.signInterval.interval
	yes, e := b.Less(&s.limits[0], i.BorrowLower())
	if e != nil || !yes {
		return false, e
	}
	return b.Less(i.BorrowUpper(), &s.limits[1])
}
func (m *AlgebraicRootCore) cleanupSignResultant() error { return m.finishSignResultant(true) }
func (m *AlgebraicRootCore) finishSignResultant(owning bool) (cause error) {
	s := &m.signResultant
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if !owning {
		if e := m.rootTraceSourceCause(); e != nil {
			s.sourceStop = e
			return e
		}
	}
	if !owning && s.sourceStop != nil {
		return s.sourceStop
	}
	defer func() {
		if !owning && cause != nil && s.sourceStop == nil {
			s.sourceStop = cause
		}
	}()

	mm := m.multivariate
	if mm == nil {
		s.active = false
		return nil
	}
	if !owning {
		if r := &s.bound; r.owner == m.config.Polynomials && r.frame == &r.owner.knuth && r.invocation != 0 && r.invocation == r.frame.invocation && r.history == r.frame.history && r.frame.sourceCause != nil {
			return r.frame.sourceCause
		}
		if e := mm.toPolynomialInvocationSourceCause(&s.toPolynomial); e != nil {
			return e
		}

		if c := m.signInterval.evaluation; c != nil && c.evaluation.intervalCaller == &m.signInterval && c.evaluation.sourceCause != nil {
			return c.evaluation.sourceCause
		}
		if c := s.conversionManager; c != nil && c.conversion.algebraicCaller == s && c.conversion.sourceCause != nil {
			return c.conversion.sourceCause
		}
		if e := mm.gcdInvocationSourceCause(&s.resultant); e != nil {
			return e
		}
		if e := mm.core.arithmeticInvocationSourceCause(&s.arithmetic); e != nil {
			return e
		}
	}
	if owning {
		if c := s.conversionManager; c != nil && c.conversion.algebraicCaller == s {
			if e := c.CleanupConversion(); e != nil {
				return e
			}
		}
	}
	if e := m.config.Polynomials.cleanupKnuthWithReceipt(&s.bound, false); e != nil {
		return e
	}
	if e := mm.cleanupFactorNumeralVector(&s.dense, false); e != nil {
		return e
	}
	if e := mm.cleanupGCDInvocation(&s.resultant, false); e != nil {
		return e
	}
	if e := mm.core.cleanupArithmeticInvocation(&s.arithmetic, false); e != nil {
		return e
	}
	if e := mm.cleanupToPolynomialInvocation(&s.toPolynomial, false); e != nil {
		return e
	}
	for s.savedCursor < len(s.saved) {
		if e := m.finishSavedInterval(&s.saved[s.savedCursor], owning); e != nil {
			return e
		}
		s.savedCursor++
	}
	if owning {
		for i := range s.saved {
			if e := m.finishSavedInterval(&s.saved[i], true); e != nil {
				return e
			}
		}
	}
	for s.limitsConstructed > 0 {
		if e := m.config.Binary.Del(&s.limits[s.limitsConstructed-1]); e != nil {
			return e
		}
		s.limitsConstructed--
	}
	for s.constructed > 0 {
		r := &s.refs[s.constructed-1]
		var e error
		if owning {
			e = r.root.close(mm.core)
		} else {
			e = r.root.closeValue(mm.core, false)
		}
		if e != nil {
			return e
		}
		s.constructed--
	}
	if owning {
		for i := range s.refs {
			if e := s.refs[i].root.abort(mm.core); e != nil {
				return e
			}
		}
		if e := m.config.Polynomials.cleanupKnuthWithReceipt(&s.bound, true); e != nil {
			return e
		}
		if e := mm.cleanupFactorNumeralVector(&s.dense, true); e != nil {
			return e
		}
		if e := mm.cleanupGCDInvocation(&s.resultant, true); e != nil {
			return e
		}
		if e := mm.cleanupToPolynomialInvocation(&s.toPolynomial, true); e != nil {
			return e
		}
		if e := mm.core.cleanupVariableInvocation(&s.variable, true); e != nil {
			return e
		}
		if e := mm.core.cleanupArithmeticInvocation(&s.arithmetic, true); e != nil {
			return e
		}
		s.active = false
	}
	return nil
}

// exactSignResultant uses the retained p_prime, variables and ri from the
// preceding phases. Its restart is consumed by the source outer loop.
func (m *AlgebraicRootCore) exactSignResultant(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation) (sign int, restart bool, cause error) {
	s := &m.signResultant
	if s.active {
		return 0, false, ErrAlgebraicPending
	}
	*s = algebraicSignResultantScope{active: true, running: true}
	defer func() {
		s.running = false
		cause = errors.Join(cause, m.finishSignResultant(false))
		if cause == nil {
			cause = m.finishSignResultant(true)
		}
	}()
	if !m.signInterval.active || !m.signInterval.constructed || len(m.evalSignVariables.values) == 0 {
		return 0, false, ErrAlgebraicOwner
	}
	if e := m.ensureMultivariate(); e != nil {
		return 0, false, e
	}
	mm := m.multivariate
	xs := m.evalSignVariables.values
	max := xs[0]
	for _, x := range xs {
		if x > max {
			max = x
		}
	}
	if max >= math.MaxUint32-1 {
		return 0, false, arena.ErrBounds
	}
	y := max + 1
	for mm.core.monomials.nextVar < y+1 {
		if _, e := mm.core.monomials.MkVar(); e != nil {
			return 0, false, e
		}
	}
	construct := func(i int) { s.refs[i] = PolynomialRef{owner: mm.core}; s.refs[i].self = &s.refs[i]; s.constructed++ }
	construct(0)
	yp, e := mm.core.mkVariableWithReceipt(y, 1, &s.variable)
	if e != nil {
		return 0, false, e
	}
	if e = s.refs[0].Assign(yp); e != nil {
		return 0, false, e
	}
	construct(1)
	aux, e := convertPolynomialForAlgebraic(pm, p, mm, math.MaxUint32, 0, s)
	if e != nil {
		return 0, false, e
	}
	if e = s.refs[1].Assign(aux); e != nil {
		return 0, false, e
	}
	construct(2)
	result, e := mm.core.subWithReceipt(yp, aux, &s.arithmetic)
	if e != nil {
		return 0, false, e
	}
	if e = s.refs[2].Assign(result); e != nil {
		return 0, false, e
	}
	construct(3)
	var sortError error
	sort.SliceStable(xs, func(i, j int) bool {
		if sortError != nil {
			return false
		}
		left, e := m.signVariableDegree(values, xs[i])
		if e != nil {
			sortError = e
			return false
		}
		right, e := m.signVariableDegree(values, xs[j])
		if e != nil {
			sortError = e
			return false
		}
		return left < right
	})
	if sortError != nil {
		return 0, false, sortError
	}
	for _, x := range xs {
		if e = m.checkpoint(); e != nil {
			return 0, false, e
		}
		a, e := m.signRoot(values, x, 2122, 2124)
		if e != nil {
			return 0, false, e
		}
		l, e := m.location(a)
		if e != nil {
			return 0, false, e
		}
		if l == nil || l.kind != 1 {
			return 0, false, ErrAlgebraicOwner
		}
		r, e := m.load(l)
		if e != nil {
			return 0, false, e
		}
		d, e := m.config.Polynomials.descriptor(r.poly)
		if e != nil {
			return 0, false, e
		}
		qi, e := mm.toPolynomialWithReceipt(r.size, m.config.Polynomials.values(&d.state), x, &s.toPolynomial)
		if e != nil {
			return 0, false, e
		}
		if e = s.refs[3].Assign(qi); e != nil {
			return 0, false, e
		}
		if e = mm.resultantWithReceipt(s.refs[2].root.value, qi, x, &s.refs[2].root, &s.resultant); e != nil {
			return 0, false, e
		}
		if e = m.assertSignResultant(false, 2129); e != nil {
			return 0, false, e
		}
	}
	if e = m.assertSignResultant(true, 2131); e != nil {
		return 0, false, e
	}
	up := m.config.Polynomials
	if m.evalSignTmp.slot == 0 {
		m.evalSignTmp, e = up.NewPolynomial()
		if e != nil {
			return 0, false, e
		}
	}
	d, e := up.descriptor(m.evalSignTmp)
	if e != nil {
		return 0, false, e
	}
	if e = mm.toNumeralVectorIntoSource(s.refs[2].root.value, 0, false, up, &d.state, &s.dense); e != nil {
		return 0, false, e
	}
	view, e := up.wholeView(m.evalSignTmp)
	if e != nil {
		return 0, false, e
	}
	k, e := up.nonzeroRootLowerBoundWithReceipt(view, &s.bound)
	if e != nil {
		return 0, false, e
	}
	if e = m.traceSignResultant(k); e != nil {
		return 0, false, e
	}
	s.limitsConstructed = 2
	b := m.config.Binary
	if e = b.SetInt32(&s.limits[0], -1); e != nil {
		return 0, false, e
	}
	if e = b.SetInt32(&s.limits[1], 1); e != nil {
		return 0, false, e
	}
	if e = b.Div2k(&s.limits[0], k); e != nil {
		return 0, false, e
	}
	if e = b.Div2k(&s.limits[1], k); e != nil {
		return 0, false, e
	}
	inside, e := m.signIntervalInsideLimits()
	if e != nil {
		return 0, false, e
	}
	if inside {
		return 0, false, nil
	}
	for _, x := range xs {
		a, e := m.signRoot(values, x, 2152, 2154)
		if e != nil {
			return 0, false, e
		}
		s.saved = append(s.saved, algebraicSavedInterval{})
		if e = m.beginSavedInterval(&s.saved[len(s.saved)-1], a); e != nil {
			return 0, false, e
		}
	}
	mapping := algebraicRootIntervals{owner: m, values: values}
	for {
		if e = m.checkpoint(); e != nil {
			return 0, false, e
		}
		if e = pm.evaluateIntervalForAlgebraic(p, &mapping, &m.signInterval.interval, &m.signInterval); e != nil {
			return 0, false, e
		}
		if e = m.traceSignInterval(true); e != nil {
			return 0, false, e
		}
		contains, e := m.config.Intervals.ContainsZero(&m.signInterval.interval)
		if e != nil {
			return 0, false, e
		}
		if !contains {
			positive, e := m.config.Intervals.IsPos(&m.signInterval.interval)
			if e != nil {
				return 0, false, e
			}
			if positive {
				return 1, false, nil
			}
			return -1, false, nil
		}
		inside, e := m.signIntervalInsideLimits()
		if e != nil {
			return 0, false, e
		}
		if inside {
			return 0, false, nil
		}
		for _, x := range xs {
			a, e := m.signRoot(values, x, 2174, 2176)
			if e != nil {
				return 0, false, e
			}
			refinable, e := m.refine(a)
			if e != nil {
				return 0, false, e
			}
			if !refinable {
				return 0, true, nil
			}
			if e = m.traceSignLiteral(2182, "refined algebraic interval\n"); e != nil {
				return 0, false, e
			}
			if e = m.assertBinaryRoot(a, 2183, "!v.is_basic()"); e != nil {
				return 0, false, e
			}
		}
	}
}
func (m *AlgebraicRootCore) assertSignResultant(univariate bool, line int32) error {
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	c := m.multivariate.core
	p := m.signResultant.refs[2].root.value
	var valid bool
	expression := "!pm().is_zero(R)"
	if univariate {
		valid, e = c.IsUnivariate(p)
		expression = "pm().is_univariate(R)"
	} else {
		var zero bool
		zero, e = c.IsZero(p)
		valid = !zero
	}
	if e != nil {
		return e
	}
	if !valid {
		return diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", line, expression)
	}
	return nil
}
