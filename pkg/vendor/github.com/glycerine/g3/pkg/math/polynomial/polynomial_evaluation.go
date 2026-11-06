package polynomial

// Source: src/math/polynomial/polynomial.cpp:1425-1433,6436-6550,7409-7414;
// src/util/scoped_numeral.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The map is a cold source virtual interface. Values are borrowed live fields;
// neither recursive frames nor the accessor copy their owning numeral tokens.
type PolynomialRationalEvaluation interface {
	Manager() (*unumeral.RationalManager, error)
	Contains(uint32) (bool, error)
	Rational(uint32) (*unumeral.Rational, error)
}
type polynomialEvalRationalFrame struct {
	aux                  unumeral.Rational
	constructed, stopped bool
	stage                uint8
	sourceCause          error
}

// Cached concrete Go frame objects represent lexical stack storage which must
// remain reachable on a stopped destructor. Their actual headers are charged; the
// pointer directories use native slices without simulated container charges.
type polynomialEvaluationScope struct {
	trace                                polynomialEvaluationTraceScope
	rationalCaller                       *algebraicRationalSignScope
	intervalCaller                       *algebraicSignIntervalScope
	rationalFrames                       []*polynomialEvalRationalFrame
	depth                                uint32
	lexGeneration                        uint64
	active, executing                    bool
	rational                             *unumeral.RationalManager
	groups                               [3]uint64 // normalize, addition, power: exact started RAW groups only
	numericPending                       umemory.ObjectRef
	allocatorPending                     [3]umemory.Block
	bodyCause, sourceCause, cleanupCause error
	integerFrames                        []*polynomialEvalIntegerFrame
	intervalFrames                       []*polynomialEvalIntervalFrame
	modular                              *unumeral.IntegerModularManager
	modularPowerPending                  bool
	interval                             *unumeral.BinaryIntervalManager
}

func (m *MultivariatePolynomialManager) evaluationRationalFrame() (*polynomialEvalRationalFrame, error) {
	s := &m.evaluation
	i := int(s.depth)
	if i == len(s.rationalFrames) {
		if e := m.originalBudget.Reserve(uint64(reflect.TypeOf(polynomialEvalRationalFrame{}).Size())); e != nil {
			return nil, e
		}
		s.rationalFrames = append(s.rationalFrames, &polynomialEvalRationalFrame{})
	}
	f := s.rationalFrames[i]
	if f.constructed {
		return nil, ErrPolynomialPending
	}
	*f = polynomialEvalRationalFrame{aux: unumeral.NewRational(), constructed: true}
	s.depth++
	return f, nil
}
func (m *MultivariatePolynomialManager) finishEvaluationRationalFrame(f *polynomialEvalRationalFrame) error {
	// A younger source destructor stopped first. Source destruction does not
	// resume by draining older lexical owners; only the explicit owning
	// adapter may acknowledge that retained stack after the stop.
	if m.evaluation.sourceCause != nil {
		return m.evaluation.sourceCause
	}
	if f.stopped {
		return f.sourceCause
	}
	for f.stage < 2 {
		a := &(*f.aux.BorrowNumerator())
		if f.stage == 1 {
			a = &(*f.aux.BorrowDenominator())
		}
		_, e := upolynomialSourceNumeralDel(m.evaluation.rational.Integers(), a)
		if e != nil {
			f.stopped = true
			f.sourceCause = e
			if m.evaluation.sourceCause == nil {
				m.evaluation.sourceCause = e
			}
			return e
		}
		f.stage++
	}
	f.constructed = false
	return nil
}

const (
	polynomialEvalSet uint8 = iota
	polynomialEvalReset
	polynomialEvalAddInteger
	polynomialEvalAdd
	polynomialEvalPower
	polynomialEvalMultiply
)

func (m *MultivariatePolynomialManager) evaluationRationalCall(op uint8, a, b *unumeral.Rational, n *unumeral.Int, k uint32, out *unumeral.Rational) error {
	s := &m.evaluation
	q := s.rational
	before := q.RawInvocationState().Active
	beforeCell := q.Integers().PendingAllocationObject()
	beforeRaw, e := q.Integers().SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	switch op {
	case polynomialEvalSet:
		e = q.SetInteger(out, n)
	case polynomialEvalReset:
		e = q.Reset(out)
	case polynomialEvalAddInteger:
		e = q.AddInt(a, n, out)
	case polynomialEvalAdd:
		e = q.Add(a, b, out)
	case polynomialEvalPower:
		e = q.Power(a, k, out)
	case polynomialEvalMultiply:
		e = q.Mul(a, b, out)
	default:
		return arena.ErrBounds
	}
	// Capture only genuinely started groups belonging to this executing call.
	state := q.RawInvocationState()
	after, gens := state.Active, state.Generations
	if s.executing {
		for i := range after {
			if !before[i] && after[i] {
				s.groups[i] = gens[i]
			}
		}
		if beforeCell == (umemory.ObjectRef{}) && q.Integers().PendingAllocationObject() != (umemory.ObjectRef{}) {
			s.numericPending = q.Integers().PendingAllocationObject()
		}
		if beforeRaw == ([3]umemory.Block{}) {
			roots, x := q.Integers().SourceAllocator().PendingRoots()
			if x != nil {
				return errors.Join(e, x)
			}
			if roots != ([3]umemory.Block{}) {
				s.allocatorPending = roots
			}
		}
	}
	return e
}
func (m *MultivariatePolynomialManager) evaluationMaxSmaller(p Polynomial, start, end, x uint32) (uint32, error) {
	y := ^uint32(0)
	for i := start; i < end; i++ {
		a, e := m.core.monomialCore(p, i)
		if e != nil {
			return 0, e
		}
		v, e := m.core.monomials.MaxSmallerThan(a, x)
		if e != nil {
			return 0, e
		}
		if v != ^uint32(0) && (y == ^uint32(0) || v > y) {
			y = v
		}
	}
	return y, nil
}
func (m *MultivariatePolynomialManager) evaluationDegree(p Polynomial, i, x uint32) (uint32, error) {
	a, e := m.core.monomialCore(p, i)
	if e != nil {
		return 0, e
	}
	return m.core.monomials.DegreeOf(a, x)
}
func (m *MultivariatePolynomialManager) evaluationRationalCore(p Polynomial, v PolynomialRationalEvaluation, start, end, x uint32, out *unumeral.Rational) (cause error) {
	if e := m.traceEvaluationEntry(p, start, end, x, &m.evaluation.trace, false); e != nil {
		return e
	}
	if e := m.evaluationCoreSourceAssertions(p, start, end, x); e != nil {
		return e
	}
	f, e := m.evaluationRationalFrame()
	if e != nil {
		return e
	}
	defer func() {
		m.evaluation.depth--
		if cause != nil {
			if m.evaluation.bodyCause == nil {
				m.evaluation.bodyCause = cause
			}
			m.evaluation.executing = false
		}
		e := m.finishEvaluationRationalFrame(f)
		if e != nil {
			m.evaluation.sourceCause = e
			cause = errors.Join(cause, e)
		}
	}()
	defer func() {
		if cause == nil {
			cause = m.traceEvaluationRationalResult(start, end, x, out)
		}
	}()

	aux := &f.aux
	if end == start+1 {
		n, e := m.core.mutableCoefficientCore(p, start)
		if e != nil {
			return e
		}
		if e = m.evaluationRationalCall(polynomialEvalSet, nil, nil, &n, 0, out); e != nil {
			return e
		}
		term, e := m.core.monomialCore(p, start)
		if e != nil {
			return e
		}
		if e = polynomialEvaluationSourceAssert(6447, "m->degree_of(x) > 0", func() (bool, error) {
			d, e := m.core.monomials.DegreeOf(term, x)
			return d > 0, e
		}); e != nil {
			return e
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
			if e = polynomialEvaluationSourceAssert(6453, "x2v.contains(y)", func() (bool, error) {
				return v.Contains(y)
			}); e != nil {
				return e
			}
			d, e := m.core.monomials.degreeAtCore(term, i)
			if e != nil {
				return e
			}
			value, e := v.Rational(y)
			if e != nil {
				return e
			}
			if e = m.evaluationRationalCall(polynomialEvalPower, value, nil, nil, d, aux); e != nil {
				return e
			}
			if e = m.evaluationRationalCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
				return e
			}
		}
		return nil
	}
	if e = polynomialEvaluationSourceAssert(6460, "x2v.contains(x)", func() (bool, error) {
		return v.Contains(x)
	}); e != nil {
		return e
	}
	value, e := v.Rational(x)
	if e != nil {
		return e
	}
	if e = m.evaluationRationalCall(polynomialEvalReset, nil, nil, nil, 0, out); e != nil {
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
				if e = polynomialEvaluationSourceAssert(6470, "end == i+1", func() (bool, error) {
					return end == i+1, nil
				}); e != nil {
					return e
				}
				n, e := m.core.mutableCoefficientCore(p, i)
				if e != nil {
					return e
				}
				return m.evaluationRationalCall(polynomialEvalAddInteger, out, nil, &n, 0, out)
			}
			if e = m.evaluationRationalCore(p, v, i, end, y, aux); e != nil {
				return e
			}
			return m.evaluationRationalCall(polynomialEvalAdd, out, aux, nil, 0, out)
		}
		j, next := i+1, uint32(0)
		for ; j < end; j++ {
			dj, e := m.evaluationDegree(p, j, x)
			if e != nil {
				return e
			}
			if e = polynomialEvaluationSourceAssert(6483, "d_j <= d", func() (bool, error) {
				return dj <= d, nil
			}); e != nil {
				return e
			}
			if dj < d {
				next = dj
				break
			}
		}
		if e = polynomialEvaluationSourceAssert(6489, "j == end || p->m(j)->degree_of(x) < d", func() (bool, error) {
			if j == end {
				return true, nil
			}
			dj, e := m.evaluationDegree(p, j, x)
			return dj < d, e
		}); e != nil {
			return e
		}
		y, e := m.evaluationMaxSmaller(p, i, j, x)
		if e != nil {
			return e
		}
		if y == ^uint32(0) {
			if e = polynomialEvaluationSourceAssert(6492, "j == i+1", func() (bool, error) {
				return j == i+1, nil
			}); e != nil {
				return e
			}
			n, e := m.core.mutableCoefficientCore(p, i)
			if e != nil {
				return e
			}
			if e = m.evaluationRationalCall(polynomialEvalSet, nil, nil, &n, 0, aux); e != nil {
				return e
			}
		} else if e = m.evaluationRationalCore(p, v, i, j, y, aux); e != nil {
			return e
		}
		if e = m.evaluationRationalCall(polynomialEvalAdd, out, aux, nil, 0, out); e != nil {
			return e
		}
		if e = m.evaluationRationalCall(polynomialEvalPower, value, nil, nil, d-next, aux); e != nil {
			return e
		}
		if e = m.evaluationRationalCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
			return e
		}
		i = j
	}
	return nil
}
func (m *MultivariatePolynomialManager) EvaluateRational(p Polynomial, v PolynomialRationalEvaluation, out *unumeral.Rational) error {
	return m.evaluateRationalForAlgebraic(p, v, out, nil)
}
func (m *MultivariatePolynomialManager) evaluateRationalForAlgebraic(p Polynomial, v PolynomialRationalEvaluation, out *unumeral.Rational, caller *algebraicRationalSignScope) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if v == nil {
		return arena.ErrConfig
	}
	q, e := v.Manager()
	if e != nil {
		return e
	}
	if q == nil || q.Integers() != m.core.integer {
		return arena.ErrConfig
	}
	if e = q.CheckOwnerReady(); e != nil {
		return e
	}
	zero, e := m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return m.evaluationEarlyRational(q, nil, out)
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		if e = m.evaluationConstantSourceAssert(p); e != nil {
			return e
		}
		n, e := m.core.mutableCoefficientCore(p, 0)
		if e != nil {
			return e
		}
		return m.evaluationEarlyRational(q, &n, out)
	}
	s := &m.evaluation
	if s.active || m.algebraicEvaluation.active {
		return ErrPolynomialPending
	}
	s.bodyCause = nil
	s.sourceCause = nil
	s.cleanupCause = nil
	s.intervalCaller = nil
	s.rationalCaller = caller
	if caller != nil {
		caller.evaluation = m
	}
	s.active = true
	s.executing = true
	s.rational = q
	defer func() {
		s.executing = false
		if s.bodyCause == nil {
			s.bodyCause = cause
		}
		if !m.evaluationPending() {
			s.active = false
		}
	}()
	if e = m.evaluationLexSort(p); e != nil {
		return e
	}
	n, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	return m.evaluationRationalCore(p, v, 0, n, x, out)
}
func (m *MultivariatePolynomialManager) evaluationPending() bool {
	s := &m.evaluation
	if s.trace.active {
		return true
	}
	for _, f := range s.rationalFrames {
		if f.constructed {
			return true
		}
	}
	for _, f := range s.integerFrames {
		if f.constructed {
			return true
		}
	}
	for _, f := range s.intervalFrames {
		if f.constructed {
			return true
		}
	}
	return s.lexGeneration != 0 || s.groups != ([3]uint64{}) || s.modularPowerPending || s.numericPending != (umemory.ObjectRef{}) || s.allocatorPending != ([3]umemory.Block{})
}
func (m *MultivariatePolynomialManager) CleanupEvaluation() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.evaluation
	if s.executing || m.core.evaluationTraceCallback() {
		return ErrPolynomialPending
	}
	if e := m.cleanupEvaluationTrace(&s.trace); e != nil {
		return e
	}
	if s.lexGeneration != 0 {
		if m.lexPending && m.lexGeneration == s.lexGeneration {
			if e := m.cleanupLexSort(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.lexGeneration = 0
	}
	z := m.core.integer
	for i := len(s.integerFrames) - 1; i >= 0; i-- {
		f := s.integerFrames[i]
		if f.constructed {
			if e := z.Del(&f.aux); e != nil {
				s.cleanupCause = e
				return e
			}
			f.constructed = false
			f.stage = 1
		}
	}
	if s.modularPowerPending {
		if e := s.modular.CleanupPowerTemporary(); e != nil {
			s.cleanupCause = e
			return e
		}
		s.modularPowerPending = false
	}
	for i := len(s.intervalFrames) - 1; i >= 0; i-- {
		f := s.intervalFrames[i]
		if f.constructed {
			for f.stage < 2 {
				a := &(*(*f.aux.BorrowLower()).BorrowNumerator())
				if f.stage == 1 {
					a = &(*(*f.aux.BorrowUpper()).BorrowNumerator())
				}
				if e := z.Del(a); e != nil {
					s.cleanupCause = e
					return e
				}
				f.stage++
			}
			f.constructed = false
		}
	}
	if s.rational != nil {
		q := s.rational
		for i := len(s.rationalFrames) - 1; i >= 0; i-- {
			f := s.rationalFrames[i]
			if !f.constructed {
				continue
			}
			for f.stage < 2 {
				a := &(*f.aux.BorrowNumerator())
				if f.stage == 1 {
					a = &(*f.aux.BorrowDenominator())
				}
				if e := q.Integers().Del(a); e != nil {
					s.cleanupCause = e
					return e
				}
				f.stage++
			}
			f.constructed = false
		}
		if e := q.CleanupRawInvocations(&s.groups); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	if s.numericPending != (umemory.ObjectRef{}) {
		if z.PendingAllocationObject() == s.numericPending {
			if e := z.CleanupPendingAllocation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.numericPending = umemory.ObjectRef{}
	}
	if s.allocatorPending != ([3]umemory.Block{}) {
		current, e := z.SourceAllocator().PendingRoots()
		if e != nil {
			return e
		}
		if current == s.allocatorPending {
			e = z.SourceAllocator().CleanupPending()
			after, inspect := z.SourceAllocator().PendingRoots()
			if inspect != nil {
				return errors.Join(e, inspect)
			}
			s.allocatorPending = after
			if e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.allocatorPending = [3]umemory.Block{}
	}
	s.active = false
	s.depth = 0
	s.cleanupCause = nil
	return nil
}
func (m *MultivariatePolynomialManager) closeEvaluationFrames() error {
	if e := m.closeAlgebraicEvaluationFrames(); e != nil {
		return e
	}
	if m.evaluation.active {
		return ErrPolynomialPending
	}
	size := uint64(reflect.TypeOf(polynomialEvalRationalFrame{}).Size())
	m.originalBudget.Release(size * uint64(len(m.evaluation.rationalFrames)))
	m.evaluation.rationalFrames = nil
	m.originalBudget.Release(uint64(len(m.evaluation.integerFrames)) * uint64(reflect.TypeOf(polynomialEvalIntegerFrame{}).Size()))
	m.originalBudget.Release(uint64(len(m.evaluation.intervalFrames)) * uint64(reflect.TypeOf(polynomialEvalIntervalFrame{}).Size()))
	m.evaluation.integerFrames = nil
	m.evaluation.intervalFrames = nil
	return nil
}

type PolynomialIntervalEvaluation interface {
	Manager() (*unumeral.BinaryIntervalManager, error)
	Contains(uint32) (bool, error)
	Interval(uint32) (*unumeral.BinaryInterval, error)
}
type polynomialEvalIntegerMap struct {
	manager *unumeral.IntegerModularManager
	x       uint32
	value   *unumeral.Int
}

func (v *polynomialEvalIntegerMap) Contains(x uint32) (bool, error) { return x == v.x, nil }
func (v *polynomialEvalIntegerMap) integerValue(x uint32) (*unumeral.Int, error) {
	if e := polynomialEvaluationSourceAssert(6532, "m_x == x", func() (bool, error) {
		return x == v.x, nil
	}); e != nil {
		return nil, e
	}
	return v.value, nil
}

type polynomialEvalIntegerFrame struct {
	aux                  unumeral.Int
	constructed, stopped bool
	stage                uint8
	sourceCause          error
}

func (m *MultivariatePolynomialManager) evaluationIntegerFrame() (*polynomialEvalIntegerFrame, error) {
	s := &m.evaluation
	i := int(s.depth)
	if i == len(s.integerFrames) {
		if e := m.originalBudget.Reserve(uint64(reflect.TypeOf(polynomialEvalIntegerFrame{}).Size())); e != nil {
			return nil, e
		}
		s.integerFrames = append(s.integerFrames, &polynomialEvalIntegerFrame{})
	}
	f := s.integerFrames[i]
	if f.constructed {
		return nil, ErrPolynomialPending
	}
	*f = polynomialEvalIntegerFrame{aux: unumeral.Int{}, constructed: true}
	s.depth++
	return f, nil
}
func (m *MultivariatePolynomialManager) finishEvaluationIntegerFrame(f *polynomialEvalIntegerFrame) error {
	// A younger source destructor stopped first. Source destruction does not
	// resume by draining older lexical owners; only the explicit owning
	// adapter may acknowledge that retained stack after the stop.
	if m.evaluation.sourceCause != nil {
		return m.evaluation.sourceCause
	}
	if f.stopped {
		return f.sourceCause
	}
	for f.stage < 1 {
		a := &f.aux
		_, e := upolynomialSourceNumeralDel(m.core.integer, a)
		if e != nil {
			f.stopped = true
			f.sourceCause = e
			if m.evaluation.sourceCause == nil {
				m.evaluation.sourceCause = e
			}
			return e
		}
		f.stage++
	}
	f.constructed = false
	return nil
}

func (m *MultivariatePolynomialManager) evaluationIntegerCore(p Polynomial, v *polynomialEvalIntegerMap, start, end, x uint32, out *unumeral.Int) (cause error) {
	if e := m.traceEvaluationEntry(p, start, end, x, &m.evaluation.trace, false); e != nil {
		return e
	}
	if e := m.evaluationCoreSourceAssertions(p, start, end, x); e != nil {
		return e
	}
	f, e := m.evaluationIntegerFrame()
	if e != nil {
		return e
	}
	defer func() {
		m.evaluation.depth--
		if cause != nil {
			if m.evaluation.bodyCause == nil {
				m.evaluation.bodyCause = cause
			}
			m.evaluation.executing = false
		}
		e := m.finishEvaluationIntegerFrame(f)
		if e != nil {
			m.evaluation.sourceCause = e
			cause = errors.Join(cause, e)
		}
	}()
	defer func() {
		if cause == nil {
			cause = m.traceEvaluationIntegerResult(start, end, x, out)
		}
	}()

	aux := &f.aux
	if end == start+1 {
		n, e := m.core.mutableCoefficientCore(p, start)
		if e != nil {
			return e
		}
		if e = m.evaluationIntegerCall(polynomialEvalSet, nil, nil, &n, 0, out); e != nil {
			return e
		}
		term, e := m.core.monomialCore(p, start)
		if e != nil {
			return e
		}
		if e = polynomialEvaluationSourceAssert(6447, "m->degree_of(x) > 0", func() (bool, error) {
			d, e := m.core.monomials.DegreeOf(term, x)
			return d > 0, e
		}); e != nil {
			return e
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
			if e = polynomialEvaluationSourceAssert(6453, "x2v.contains(y)", func() (bool, error) {
				return v.Contains(y)
			}); e != nil {
				return e
			}
			d, e := m.core.monomials.degreeAtCore(term, i)
			if e != nil {
				return e
			}
			value, e := v.integerValue(y)
			if e != nil {
				return e
			}
			if e = m.evaluationIntegerCall(polynomialEvalPower, value, nil, nil, d, aux); e != nil {
				return e
			}
			if e = m.evaluationIntegerCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
				return e
			}
		}
		return nil
	}
	if e = polynomialEvaluationSourceAssert(6460, "x2v.contains(x)", func() (bool, error) {
		return v.Contains(x)
	}); e != nil {
		return e
	}
	value, e := v.integerValue(x)
	if e != nil {
		return e
	}
	if e = m.evaluationIntegerCall(polynomialEvalReset, nil, nil, nil, 0, out); e != nil {
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
				if e = polynomialEvaluationSourceAssert(6470, "end == i+1", func() (bool, error) {
					return end == i+1, nil
				}); e != nil {
					return e
				}
				n, e := m.core.mutableCoefficientCore(p, i)
				if e != nil {
					return e
				}
				return m.evaluationIntegerCall(polynomialEvalAddInteger, out, nil, &n, 0, out)
			}
			if e = m.evaluationIntegerCore(p, v, i, end, y, aux); e != nil {
				return e
			}
			return m.evaluationIntegerCall(polynomialEvalAdd, out, aux, nil, 0, out)
		}
		j, next := i+1, uint32(0)
		for ; j < end; j++ {
			dj, e := m.evaluationDegree(p, j, x)
			if e != nil {
				return e
			}
			if e = polynomialEvaluationSourceAssert(6483, "d_j <= d", func() (bool, error) {
				return dj <= d, nil
			}); e != nil {
				return e
			}
			if dj < d {
				next = dj
				break
			}
		}
		if e = polynomialEvaluationSourceAssert(6489, "j == end || p->m(j)->degree_of(x) < d", func() (bool, error) {
			if j == end {
				return true, nil
			}
			dj, e := m.evaluationDegree(p, j, x)
			return dj < d, e
		}); e != nil {
			return e
		}
		y, e := m.evaluationMaxSmaller(p, i, j, x)
		if e != nil {
			return e
		}
		if y == ^uint32(0) {
			if e = polynomialEvaluationSourceAssert(6492, "j == i+1", func() (bool, error) {
				return j == i+1, nil
			}); e != nil {
				return e
			}
			n, e := m.core.mutableCoefficientCore(p, i)
			if e != nil {
				return e
			}
			if e = m.evaluationIntegerCall(polynomialEvalSet, nil, nil, &n, 0, aux); e != nil {
				return e
			}
		} else if e = m.evaluationIntegerCore(p, v, i, j, y, aux); e != nil {
			return e
		}
		if e = m.evaluationIntegerCall(polynomialEvalAdd, out, aux, nil, 0, out); e != nil {
			return e
		}
		if e = m.evaluationIntegerCall(polynomialEvalPower, value, nil, nil, d-next, aux); e != nil {
			return e
		}
		if e = m.evaluationIntegerCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
			return e
		}
		i = j
	}
	return nil
}

type polynomialEvalIntervalFrame struct {
	aux                  unumeral.BinaryInterval
	constructed, stopped bool
	stage                uint8
	sourceCause          error
}

func (m *MultivariatePolynomialManager) evaluationIntervalFrame() (*polynomialEvalIntervalFrame, error) {
	s := &m.evaluation
	i := int(s.depth)
	if i == len(s.intervalFrames) {
		if e := m.originalBudget.Reserve(uint64(reflect.TypeOf(polynomialEvalIntervalFrame{}).Size())); e != nil {
			return nil, e
		}
		s.intervalFrames = append(s.intervalFrames, &polynomialEvalIntervalFrame{})
	}
	f := s.intervalFrames[i]
	if f.constructed {
		return nil, ErrPolynomialPending
	}
	*f = polynomialEvalIntervalFrame{aux: unumeral.NewBinaryInterval(), constructed: true}
	s.depth++
	return f, nil
}
func (m *MultivariatePolynomialManager) finishEvaluationIntervalFrame(f *polynomialEvalIntervalFrame) error {
	// A younger source destructor stopped first. Source destruction does not
	// resume by draining older lexical owners; only the explicit owning
	// adapter may acknowledge that retained stack after the stop.
	if m.evaluation.sourceCause != nil {
		return m.evaluation.sourceCause
	}
	if f.stopped {
		return f.sourceCause
	}
	for f.stage < 2 {
		a := &(*(*f.aux.BorrowLower()).BorrowNumerator())
		if f.stage == 1 {
			a = &(*(*f.aux.BorrowUpper()).BorrowNumerator())
		}
		_, e := upolynomialSourceNumeralDel(m.core.integer, a)
		if e != nil {
			f.stopped = true
			f.sourceCause = e
			if m.evaluation.sourceCause == nil {
				m.evaluation.sourceCause = e
			}
			return e
		}
		f.stage++
	}
	f.constructed = false
	return nil
}

func (m *MultivariatePolynomialManager) evaluationIntervalCore(p Polynomial, v PolynomialIntervalEvaluation, start, end, x uint32, out *unumeral.BinaryInterval) (cause error) {
	if e := m.traceEvaluationEntry(p, start, end, x, &m.evaluation.trace, false); e != nil {
		return e
	}
	if e := m.evaluationCoreSourceAssertions(p, start, end, x); e != nil {
		return e
	}
	f, e := m.evaluationIntervalFrame()
	if e != nil {
		return e
	}
	defer func() {
		m.evaluation.depth--
		if cause != nil {
			if m.evaluation.bodyCause == nil {
				m.evaluation.bodyCause = cause
			}
			m.evaluation.executing = false
		}
		e := m.finishEvaluationIntervalFrame(f)
		if e != nil {
			m.evaluation.sourceCause = e
			cause = errors.Join(cause, e)
		}
	}()
	defer func() {
		if cause == nil {
			cause = m.traceEvaluationIntervalResult(start, end, x, out)
		}
	}()

	aux := &f.aux
	if end == start+1 {
		n, e := m.core.mutableCoefficientCore(p, start)
		if e != nil {
			return e
		}
		if e = m.evaluationIntervalCall(polynomialEvalSet, nil, nil, &n, 0, out); e != nil {
			return e
		}
		term, e := m.core.monomialCore(p, start)
		if e != nil {
			return e
		}
		if e = polynomialEvaluationSourceAssert(6447, "m->degree_of(x) > 0", func() (bool, error) {
			d, e := m.core.monomials.DegreeOf(term, x)
			return d > 0, e
		}); e != nil {
			return e
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
			if e = polynomialEvaluationSourceAssert(6453, "x2v.contains(y)", func() (bool, error) {
				return v.Contains(y)
			}); e != nil {
				return e
			}
			d, e := m.core.monomials.degreeAtCore(term, i)
			if e != nil {
				return e
			}
			value, e := v.Interval(y)
			if e != nil {
				return e
			}
			if e = m.evaluationIntervalCall(polynomialEvalPower, value, nil, nil, d, aux); e != nil {
				return e
			}
			if e = m.evaluationIntervalCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
				return e
			}
		}
		return nil
	}
	if e = polynomialEvaluationSourceAssert(6460, "x2v.contains(x)", func() (bool, error) {
		return v.Contains(x)
	}); e != nil {
		return e
	}
	value, e := v.Interval(x)
	if e != nil {
		return e
	}
	if e = m.evaluationIntervalCall(polynomialEvalReset, nil, nil, nil, 0, out); e != nil {
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
				if e = polynomialEvaluationSourceAssert(6470, "end == i+1", func() (bool, error) {
					return end == i+1, nil
				}); e != nil {
					return e
				}
				n, e := m.core.mutableCoefficientCore(p, i)
				if e != nil {
					return e
				}
				return m.evaluationIntervalCall(polynomialEvalAddInteger, out, nil, &n, 0, out)
			}
			if e = m.evaluationIntervalCore(p, v, i, end, y, aux); e != nil {
				return e
			}
			return m.evaluationIntervalCall(polynomialEvalAdd, out, aux, nil, 0, out)
		}
		j, next := i+1, uint32(0)
		for ; j < end; j++ {
			dj, e := m.evaluationDegree(p, j, x)
			if e != nil {
				return e
			}
			if e = polynomialEvaluationSourceAssert(6483, "d_j <= d", func() (bool, error) {
				return dj <= d, nil
			}); e != nil {
				return e
			}
			if dj < d {
				next = dj
				break
			}
		}
		if e = polynomialEvaluationSourceAssert(6489, "j == end || p->m(j)->degree_of(x) < d", func() (bool, error) {
			if j == end {
				return true, nil
			}
			dj, e := m.evaluationDegree(p, j, x)
			return dj < d, e
		}); e != nil {
			return e
		}
		y, e := m.evaluationMaxSmaller(p, i, j, x)
		if e != nil {
			return e
		}
		if y == ^uint32(0) {
			if e = polynomialEvaluationSourceAssert(6492, "j == i+1", func() (bool, error) {
				return j == i+1, nil
			}); e != nil {
				return e
			}
			n, e := m.core.mutableCoefficientCore(p, i)
			if e != nil {
				return e
			}
			if e = m.evaluationIntervalCall(polynomialEvalSet, nil, nil, &n, 0, aux); e != nil {
				return e
			}
		} else if e = m.evaluationIntervalCore(p, v, i, j, y, aux); e != nil {
			return e
		}
		if e = m.evaluationIntervalCall(polynomialEvalAdd, out, aux, nil, 0, out); e != nil {
			return e
		}
		if e = m.evaluationIntervalCall(polynomialEvalPower, value, nil, nil, d-next, aux); e != nil {
			return e
		}
		if e = m.evaluationIntervalCall(polynomialEvalMultiply, out, aux, nil, 0, out); e != nil {
			return e
		}
		i = j
	}
	return nil
}

func (m *MultivariatePolynomialManager) evaluationCaptureNumeric(beforeCell umemory.ObjectRef, beforeRaw [3]umemory.Block, cause error) error {
	s := &m.evaluation
	if !s.executing {
		return cause
	}
	z := m.core.integer
	if beforeCell == (umemory.ObjectRef{}) && z.PendingAllocationObject() != (umemory.ObjectRef{}) {
		s.numericPending = z.PendingAllocationObject()
	}
	if beforeRaw == ([3]umemory.Block{}) {
		roots, e := z.SourceAllocator().PendingRoots()
		if e != nil {
			return errors.Join(cause, e)
		}
		if roots != ([3]umemory.Block{}) {
			s.allocatorPending = roots
		}
	}
	return cause
}
func (m *MultivariatePolynomialManager) evaluationIntegerCall(op uint8, a, b *unumeral.Int, n *unumeral.Int, k uint32, out *unumeral.Int) error {
	z := m.core.integer
	beforeCell := z.PendingAllocationObject()
	beforeRaw, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	vm := m.evaluation.modular
	beforePower := vm.PowerPending
	switch op {
	case polynomialEvalSet:
		e = vm.Copy(out, n)
	case polynomialEvalReset:
		e = vm.Reset(out)
	case polynomialEvalAddInteger:
		e = vm.Add(a, n, out)
	case polynomialEvalAdd:
		e = vm.Add(a, b, out)
	case polynomialEvalPower:
		e = vm.Power(a, k, out)
	case polynomialEvalMultiply:
		e = vm.Mul(a, b, out)
	default:
		return arena.ErrBounds
	}
	if !beforePower && vm.PowerPending {
		m.evaluation.modularPowerPending = true
	}
	return m.evaluationCaptureNumeric(beforeCell, beforeRaw, e)
}
func (m *MultivariatePolynomialManager) evaluationIntervalCall(op uint8, a, b *unumeral.BinaryInterval, n *unumeral.Int, k uint32, out *unumeral.BinaryInterval) error {
	z := m.core.integer
	beforeCell := z.PendingAllocationObject()
	beforeRaw, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	vm := m.evaluation.interval
	switch op {
	case polynomialEvalSet:
		e = vm.SetInteger(out, n)
	case polynomialEvalReset:
		e = vm.Reset(out)
	case polynomialEvalAddInteger:
		e = vm.AddInteger(a, n, out)
	case polynomialEvalAdd:
		e = vm.Add(a, b, out)
	case polynomialEvalPower:
		e = vm.Power(a, k, out)
	case polynomialEvalMultiply:
		e = vm.Mul(a, b, out)
	default:
		return arena.ErrBounds
	}
	return m.evaluationCaptureNumeric(beforeCell, beforeRaw, e)
}
func (m *MultivariatePolynomialManager) UnivariateEvaluate(p Polynomial, x uint32, value, out *unumeral.Int) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if e := polynomialEvaluationSourceAssert(6536, "is_univariate(p)", func() (bool, error) {
		return m.core.IsUnivariate(p)
	}); e != nil {
		return e
	}
	zero, e := m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return m.evaluationEarlyInteger(nil, out)
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		n, e := m.core.coefficientCore(p, 0)
		if e != nil {
			return e
		}
		return m.evaluationEarlyInteger(&n, out)
	}
	s := &m.evaluation
	if s.active || m.algebraicEvaluation.active {
		return ErrPolynomialPending
	}
	s.bodyCause = nil
	s.sourceCause = nil
	s.cleanupCause = nil
	s.rationalCaller = nil
	s.intervalCaller = nil
	s.active = true
	s.executing = true
	s.modular = m.core.modular
	defer func() {
		s.executing = false
		if s.bodyCause == nil {
			s.bodyCause = cause
		}
		if !m.evaluationPending() {
			s.active = false
		}
	}()
	values := polynomialEvalIntegerMap{manager: m.core.modular, x: x, value: value}
	// The source helper calls t_eval, including its repeated zero/constant checks.
	zero, e = m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return m.evaluationIntegerCall(polynomialEvalReset, nil, nil, nil, 0, out)
	}
	constant, e = m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		if e = m.evaluationConstantSourceAssert(p); e != nil {
			return e
		}
		n, e := m.core.mutableCoefficientCore(p, 0)
		if e != nil {
			return e
		}
		return m.evaluationIntegerCall(polynomialEvalSet, nil, nil, &n, 0, out)
	}
	if e = m.evaluationLexSort(p); e != nil {
		return e
	}
	n, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	max, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	return m.evaluationIntegerCore(p, &values, 0, n, max, out)
}
func (m *MultivariatePolynomialManager) EvaluateInterval(p Polynomial, v PolynomialIntervalEvaluation, out *unumeral.BinaryInterval) error {
	return m.evaluateIntervalForAlgebraic(p, v, out, nil)
}
func (m *MultivariatePolynomialManager) evaluateIntervalForAlgebraic(p Polynomial, v PolynomialIntervalEvaluation, out *unumeral.BinaryInterval, caller *algebraicSignIntervalScope) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if v == nil {
		return arena.ErrConfig
	}
	vm, e := v.Manager()
	if e != nil {
		return e
	}
	if vm == nil || vm.BorrowBounds() == nil || vm.BorrowBounds().Integers() != m.core.integer {
		return arena.ErrConfig
	}
	if e = vm.Open(); e != nil {
		return e
	}
	zero, e := m.core.IsZero(p)
	if e != nil {
		return e
	}
	if zero {
		return m.evaluationEarlyInterval(vm, nil, out)
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		if e = m.evaluationConstantSourceAssert(p); e != nil {
			return e
		}
		n, e := m.core.mutableCoefficientCore(p, 0)
		if e != nil {
			return e
		}
		return m.evaluationEarlyInterval(vm, &n, out)
	}
	s := &m.evaluation
	if s.active || m.algebraicEvaluation.active {
		return ErrPolynomialPending
	}
	s.bodyCause = nil
	s.sourceCause = nil
	s.cleanupCause = nil
	s.rationalCaller = nil
	s.intervalCaller = caller
	if caller != nil {
		caller.evaluation = m
	}
	s.active = true
	s.executing = true
	s.interval = vm
	defer func() {
		s.executing = false
		if s.bodyCause == nil {
			s.bodyCause = cause
		}
		if !m.evaluationPending() {
			s.active = false
		}
	}()
	if e = m.evaluationLexSort(p); e != nil {
		return e
	}
	n, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	max, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	return m.evaluationIntervalCore(p, v, 0, n, max, out)
}

// Observe only this explicit dependency invocation; unrelated operations while
// a scope remains active cannot acquire the evaluator's cleanup authority.
func (m *MultivariatePolynomialManager) evaluationLexSort(p Polynomial) error {
	before := m.lexGeneration
	e := m.LexSort(p)
	if m.evaluation.executing && m.lexGeneration != before && m.lexPending {
		m.evaluation.lexGeneration = m.lexGeneration
	}
	return e
}

// Source early reset/set routes construct no recursive auxiliary or rational
// RAW group. Preserve any older invocation, while separately recording only a
// genuinely newly started Integer allocation at this reached VM call.
func (m *MultivariatePolynomialManager) evaluationEarlyRoots(beforeCell umemory.ObjectRef, beforeRaw [3]umemory.Block, cause error) error {
	z := m.core.integer
	s := &m.evaluation
	if beforeCell == (umemory.ObjectRef{}) && z.PendingAllocationObject() != (umemory.ObjectRef{}) {
		s.numericPending = z.PendingAllocationObject()
	}
	if beforeRaw == ([3]umemory.Block{}) {
		after, e := z.SourceAllocator().PendingRoots()
		if e != nil {
			return errors.Join(cause, e)
		}
		if after != ([3]umemory.Block{}) {
			s.allocatorPending = after
		}
	}
	if m.evaluationPending() {
		if !s.active {
			s.rationalCaller = nil
			s.intervalCaller = nil
			s.bodyCause = cause
		}
		s.active = true
	}
	return cause
}
func (m *MultivariatePolynomialManager) evaluationEarlyRational(q *unumeral.RationalManager, n *unumeral.Int, out *unumeral.Rational) error {
	z := m.core.integer
	beforeCell := z.PendingAllocationObject()
	beforeRaw, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	if n == nil {
		e = q.Reset(out)
	} else {
		e = q.SetInteger(out, n)
	}
	return m.evaluationEarlyRoots(beforeCell, beforeRaw, e)
}
func (m *MultivariatePolynomialManager) evaluationEarlyInterval(vm *unumeral.BinaryIntervalManager, n *unumeral.Int, out *unumeral.BinaryInterval) error {
	z := m.core.integer
	beforeCell := z.PendingAllocationObject()
	beforeRaw, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	if n == nil {
		e = vm.Reset(out)
	} else {
		e = vm.SetInteger(out, n)
	}
	return m.evaluationEarlyRoots(beforeCell, beforeRaw, e)
}

func (m *MultivariatePolynomialManager) evaluationEarlyInteger(n *unumeral.Int, out *unumeral.Int) error {
	z := m.core.integer
	beforeCell := z.PendingAllocationObject()
	beforeRaw, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	// univ_eval uses SET0, not RESET, before constructing its single-var map.
	if n == nil {
		e = m.core.modular.SetInt32(out, 0)
	} else {
		e = m.core.modular.Copy(out, n)
	}
	return m.evaluationEarlyRoots(beforeCell, beforeRaw, e)
}
