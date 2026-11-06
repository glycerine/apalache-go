package polynomial

// Source: src/math/polynomial/polynomial.cpp:4653-4890,7248-7284;
// src/util/scoped_numeral.h; src/util/obj_ref.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialCompositionScope struct {
	a                                       unumeral.Int
	raw                                     [4]unumeral.Int
	rawMask                                 uint8
	aCompleted                              bool
	active, executing, stopped              bool
	local                                   polynomialReference
	localCompleted                          bool
	arithmetic                              polynomialArithmeticInvocationReceipt
	degreeInput                             Polynomial
	degreeCompleted                         bool
	degreeStored                            uint32
	generated                               Monomial
	coreGenerated                           Monomial
	freshResult                             Polynomial
	orphans                                 []Monomial
	drain                                   uint32
	calleeGeneration, factoryGeneration     uint64
	singleton                               Monomial
	singletonGeneration, constantGeneration uint64
	numericPending                          umemory.ObjectRef
	integerRaw, monomialRaw                 [3]umemory.Block
	bodyCause, sourceCause, cleanupCause    error
}

func (m *MultivariatePolynomialManager) beginComposition() error {
	s := &m.composition
	if s.active {
		return ErrPolynomialPending
	}
	orphans, drain := s.orphans, s.drain
	*s = polynomialCompositionScope{active: true, executing: true, orphans: orphans, drain: drain}
	m.core.compositionOwner = s
	return nil
}
func (m *MultivariatePolynomialManager) finishComposition(cause error) error {
	s := &m.composition
	s.executing = false
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	if e := m.core.arithmeticInvocationSourceCause(&s.arithmetic); e != nil {
		s.stopped = true
		if s.sourceCause == nil {
			s.sourceCause = e
		}
	}
	if s.constantGeneration != 0 && m.core.constantGeneration == s.constantGeneration && m.core.constantScope.stopped {
		s.stopped = true
		if s.sourceCause == nil {
			s.sourceCause = m.core.constantScope.sourceCause
		}
	}
	if s.stopped {
		return errors.Join(cause, s.sourceCause)
	}
	if s.aCompleted {
		_, e := upolynomialSourceNumeralDel(m.core.integer, &s.a)
		if e != nil {
			s.stopped = true
			s.sourceCause = e
			return errors.Join(cause, e)
		}
		s.aCompleted = false
	}
	if s.degreeCompleted {
		if e := m.resetCompositionDegrees(s.degreeInput); e != nil {
			s.stopped = true
			s.sourceCause = e
			return errors.Join(cause, e)
		}
		s.degreeCompleted = false
	}
	if s.localCompleted {
		if e := s.local.closeValue(m.core, false); e != nil {
			s.stopped = true
			s.sourceCause = e
			return errors.Join(cause, e)
		}
		s.localCompleted = false
	}
	if cause == nil && s.freshResult == (Polynomial{}) && s.generated == (Monomial{}) && s.factoryGeneration == 0 && s.calleeGeneration == 0 && s.numericPending == (umemory.ObjectRef{}) && s.integerRaw == ([3]umemory.Block{}) && s.monomialRaw == ([3]umemory.Block{}) && s.rawMask == 0 {
		s.active = false
	}
	return cause
}
func (m *MultivariatePolynomialManager) compositionNumeral(op uint8, n *unumeral.Int, k uint32, out *unumeral.Int) error {
	z := m.core.integer
	cell := z.PendingAllocationObject()
	roots, e := z.SourceAllocator().PendingRoots()
	if e != nil {
		return e
	}
	switch op {
	case 0:
		e = m.core.modular.Copy(out, n)
	case 1:
		e = m.core.modular.Neg(out)
	case 2:
		e = m.core.modular.Reset(out)
	case 3:
		e = m.core.modular.SetUint64(out, uint64(k))
	default:
		return arena.ErrBounds
	}
	s := &m.composition
	if s.executing {
		if cell == (umemory.ObjectRef{}) && z.PendingAllocationObject() != (umemory.ObjectRef{}) {
			s.numericPending = z.PendingAllocationObject()
		}
		if roots == ([3]umemory.Block{}) {
			after, x := z.SourceAllocator().PendingRoots()
			if x != nil {
				return errors.Join(e, x)
			}
			if after != ([3]umemory.Block{}) {
				s.integerRaw = after
			}
		}
	}
	return e
}
func (m *MultivariatePolynomialManager) compositionVariable(x, d uint32, powers []MonomialPower, product bool) (Monomial, error) {
	s := &m.composition
	c := m.core
	if s.generated != (Monomial{}) {
		return Monomial{}, ErrPolynomialPending
	}
	before, e := c.monomials.allocator.PendingRoots()
	if e != nil {
		return Monomial{}, e
	}
	outcome := monomialFactoryOutcome{}
	var a Monomial
	if product {
		a, e = c.monomials.mkMonomialCreated(powers, &outcome)
	} else {
		a, e = c.monomials.mkVariableCreated(x, d, &outcome)
	}
	if outcome.started {
		c.recordGeneratedCallee(outcome)
		if c.monomials.creation.generation == outcome.generation && c.monomials.creation.parent == c {
			s.calleeGeneration = outcome.generation
		}
	}
	if before == ([3]umemory.Block{}) {
		after, x := c.monomials.allocator.PendingRoots()
		if x != nil {
			return Monomial{}, errors.Join(e, x)
		}
		if after != ([3]umemory.Block{}) {
			s.monomialRaw = after
		}
	}
	if e == nil && outcome.created {
		s.generated = a
	}
	return a, e
}
func (m *MultivariatePolynomialManager) compositionAdd(a *unumeral.Int, t Monomial) error {
	s := &m.composition
	consumed := false
	e := m.core.cheap.addConsumed(a, t, false, &consumed)
	if s.generated == t {
		if consumed {
			s.generated = Monomial{}
		} else if e == nil {
			i := uint32(len(s.orphans))
			old := s.orphans
			s.orphans = append(s.orphans, t)
			if len(old) != 0 && &old[0] != &s.orphans[0] {
				clear(old)
			}
			l, x := m.core.monomials.location(t)
			if x != nil {
				return x
			}
			l.compositionCohort = s
			l.compositionSlot = i
			s.generated = Monomial{}
		}
	}
	return e
}
func (m *MultivariatePolynomialManager) compositionCheapReady(line int32) error {
	return polynomialCompositionSourceAssert(line, "m_cheap_som_buffer.empty()", func() (bool, error) {
		n, e := m.core.cheap.ms.size(m.core)
		return n == 0, e
	})
}
func (m *MultivariatePolynomialManager) compositionUnivariate(p Polynomial, line int32) error {
	return polynomialCompositionSourceAssert(line, "is_univariate(p)", func() (bool, error) {
		return m.core.IsUnivariate(p)
	})
}
func (m *MultivariatePolynomialManager) compositionCheapResult() (Polynomial, error) {
	p, e := m.core.cheap.mk(false)
	if e == nil {
		m.composition.factoryGeneration = 0
	}
	return p, e
}

func (m *MultivariatePolynomialManager) ComposeReciprocal(p Polynomial) (result Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := m.compositionUnivariate(p, 4655); e != nil {
		return Polynomial{}, e
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return Polynomial{}, e
	}
	if constant {
		return p, nil
	}
	if e = m.compositionCheapReady(4658); e != nil {
		return Polynomial{}, e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return Polynomial{}, e
	}
	n, e := m.core.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.beginComposition(); e != nil {
		return Polynomial{}, e
	}
	defer func() { result, cause = m.finishCompositionResult(result, cause) }()
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = polynomialCompositionSourceAssert(4664, "m->size() <= 1", func() (bool, error) {
			sz, e := m.core.monomials.sizeCore(t)
			return sz <= 1, e
		}); e != nil {
			return Polynomial{}, e
		}
		k, e := m.core.monomials.DegreeOf(t, x)
		if e != nil {
			return Polynomial{}, e
		}
		next, e := m.compositionVariable(x, n-k, nil, false)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.core.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.compositionAdd(&a, next); e != nil {
			return Polynomial{}, e
		}
	}
	return m.compositionCheapResult()
}
func (m *MultivariatePolynomialManager) ComposeXDivY(p Polynomial, y uint32) (result Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := m.compositionUnivariate(p, 4677); e != nil {
		return Polynomial{}, e
	}
	if e := polynomialCompositionSourceAssert(4678, "max_var(p) != y", func() (bool, error) {
		x, e := m.core.MaxVariable(p)
		return x != y, e
	}); e != nil {
		return Polynomial{}, e
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return Polynomial{}, e
	}
	if constant {
		return p, nil
	}
	if e = m.compositionCheapReady(4681); e != nil {
		return Polynomial{}, e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return Polynomial{}, e
	}
	n, e := m.core.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.beginComposition(); e != nil {
		return Polynomial{}, e
	}
	defer func() { result, cause = m.finishCompositionResult(result, cause) }()
	// source sbuffer<power> is lexical inline storage; at most two actual powers
	// are initialized on this path. No Runtime allocation ticket is introduced.
	var powers [16]MonomialPower
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		k, e := m.core.monomials.DegreeOf(t, x)
		if e != nil {
			return Polynomial{}, e
		}
		count := 0
		if x < y {
			if k > 0 {
				powers[count] = MonomialPower{Var: x, Degree: k}
				count++
			}
			if n-k > 0 {
				powers[count] = MonomialPower{Var: y, Degree: n - k}
				count++
			}
		} else {
			if n-k > 0 {
				powers[count] = MonomialPower{Var: y, Degree: n - k}
				count++
			}
			if k > 0 {
				powers[count] = MonomialPower{Var: x, Degree: k}
				count++
			}
		}
		next, e := m.compositionVariable(0, 0, powers[:count], true)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.core.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.compositionAdd(&a, next); e != nil {
			return Polynomial{}, e
		}
	}
	return m.compositionCheapResult()
}
func (m *MultivariatePolynomialManager) ComposeVariable(p Polynomial, y uint32) (result Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := polynomialCompositionSourceAssert(4704, "is_valid(y)", func() (bool, error) {
		return y < m.core.monomials.nextVar, nil
	}); e != nil {
		return Polynomial{}, e
	}
	if e := m.compositionUnivariate(p, 4705); e != nil {
		return Polynomial{}, e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return Polynomial{}, e
	}
	if y == x {
		return p, nil
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return Polynomial{}, e
	}
	if constant {
		return p, nil
	}
	if e = m.compositionCheapReady(4708); e != nil {
		return Polynomial{}, e
	}
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.beginComposition(); e != nil {
		return Polynomial{}, e
	}
	defer func() { result, cause = m.finishCompositionResult(result, cause) }()
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = polynomialCompositionSourceAssert(4712, "m->size() <= 1", func() (bool, error) {
			n, e := m.core.monomials.sizeCore(t)
			return n <= 1, e
		}); e != nil {
			return Polynomial{}, e
		}
		sz, e := m.core.monomials.sizeCore(t)
		if e != nil {
			return Polynomial{}, e
		}
		next := t
		if sz != 0 {
			d, e := m.core.monomials.degreeAtCore(t, 0)
			if e != nil {
				return Polynomial{}, e
			}
			next, e = m.compositionVariable(y, d, nil, false)
			if e != nil {
				return Polynomial{}, e
			}
		}
		a, e := m.core.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.compositionAdd(&a, next); e != nil {
			return Polynomial{}, e
		}
	}
	return m.compositionCheapResult()
}
func (m *MultivariatePolynomialManager) ComposeMinusX(p Polynomial) (result Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := m.compositionUnivariate(p, 4724); e != nil {
		return Polynomial{}, e
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return Polynomial{}, e
	}
	if constant {
		return p, nil
	}
	if e = m.compositionCheapReady(4727); e != nil {
		return Polynomial{}, e
	}
	if e = m.beginComposition(); e != nil {
		return Polynomial{}, e
	}
	defer func() { result, cause = m.finishCompositionResult(result, cause) }()
	s := &m.composition
	s.aCompleted = true
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		d, e := m.core.monomials.degreeCore(t)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.core.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if d%2 == 0 {
			if e = m.compositionAdd(&a, t); e != nil {
				return Polynomial{}, e
			}
		} else {
			if e = m.compositionNumeral(0, &a, 0, &s.a); e != nil {
				return Polynomial{}, e
			}
			if e = m.compositionNumeral(1, nil, 0, &s.a); e != nil {
				return Polynomial{}, e
			}
			if e = m.compositionAdd(&s.a, t); e != nil {
				return Polynomial{}, e
			}
		}
	}
	return m.compositionCheapResult()
}

func (m *MultivariatePolynomialManager) reserveCompositionDegrees(n uint32) {
	old := len(m.degree2pos)
	if uint64(n) <= uint64(old) {
		return
	}
	m.degree2pos = append(m.degree2pos, make([]uint32, int(n)-old)...)
	for i := old; i < len(m.degree2pos); i++ {
		m.degree2pos[i] = ^uint32(0)
	}
}
func (m *MultivariatePolynomialManager) compositionDegreePosition(i uint32) (uint32, error) {
	if uint64(i) >= uint64(len(m.degree2pos)) {
		if polynomialSourceDebug {
			return 0, ErrPolynomialAssertion
		}
		return 0, arena.ErrBounds
	}
	return m.degree2pos[i], nil
}
func (m *MultivariatePolynomialManager) saveCompositionDegrees(p Polynomial) error {
	if e := m.compositionUnivariate(p, 4748); e != nil {
		return e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	n, e := m.core.Degree(p, x)
	if e != nil {
		return e
	}
	m.reserveCompositionDegrees(n + 1)
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return e
		}
		if e = polynomialCompositionSourceAssert(4755, "m->size() <= 1", func() (bool, error) {
			sz, e := m.core.monomials.sizeCore(t)
			return sz <= 1, e
		}); e != nil {
			return e
		}
		if e = polynomialCompositionSourceAssert(4756, "m_degree2pos[m->total_degree()] == UINT_MAX", func() (bool, error) {
			d, e := m.core.monomials.degreeCore(t)
			if e != nil {
				return false, e
			}
			pos, e := m.compositionDegreePosition(d)
			return pos == ^uint32(0), e
		}); e != nil {
			return e
		}
		d, e := m.core.monomials.degreeCore(t)
		if e != nil {
			return e
		}
		if _, e = m.compositionDegreePosition(d); e != nil {
			return e
		}
		m.degree2pos[d] = i
		m.composition.degreeStored = i + 1
	}
	return nil
}
func (m *MultivariatePolynomialManager) resetCompositionDegrees(p Polynomial) error {
	if e := m.compositionUnivariate(p, 4765); e != nil {
		return e
	}
	size, e := m.core.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		t, e := m.core.monomialCore(p, i)
		if e != nil {
			return e
		}
		if e = polynomialCompositionSourceAssert(4769, "m->size() <= 1", func() (bool, error) {
			sz, e := m.core.monomials.sizeCore(t)
			return sz <= 1, e
		}); e != nil {
			return e
		}
		if e = polynomialCompositionSourceAssert(4770, "m_degree2pos[m->total_degree()] == i", func() (bool, error) {
			d, e := m.core.monomials.degreeCore(t)
			if e != nil {
				return false, e
			}
			pos, e := m.compositionDegreePosition(d)
			return pos == i, e
		}); e != nil {
			return e
		}
		d, e := m.core.monomials.degreeCore(t)
		if e != nil {
			return e
		}
		if _, e = m.compositionDegreePosition(d); e != nil {
			return e
		}
		m.degree2pos[d] = ^uint32(0)
	}
	m.composition.degreeStored = 0
	return nil
}
func (m *MultivariatePolynomialManager) compositionAssign(r *PolynomialRef, p Polynomial) error {
	s := &m.composition
	executing := s.executing
	s.executing = false
	e := r.Assign(p)
	s.executing = executing
	if s.freshResult == p && (r.root.value == p || r.root.incoming == p) {
		s.freshResult = Polynomial{}
	}
	return e
}
func (m *MultivariatePolynomialManager) compositionAssignLocal(p Polynomial) error {
	s := &m.composition
	executing := s.executing
	s.executing = false
	e := s.local.assign(m.core, p)
	s.executing = executing
	if s.freshResult == p && (s.local.value == p || s.local.incoming == p) {
		s.freshResult = Polynomial{}
	}
	return e
}
func (m *MultivariatePolynomialManager) composeBody(p, q Polynomial, r *PolynomialRef) error {
	if e := m.compositionUnivariate(p, 4799); e != nil {
		return e
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		return m.compositionAssign(r, p)
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	d, e := m.core.Degree(p, x)
	if e != nil {
		return e
	}
	s := &m.composition
	s.degreeInput = p
	if e = m.saveCompositionDegrees(p); e != nil {
		return e
	}
	s.degreeCompleted = true
	s.a = unumeral.Int{}
	s.aCompleted = true
	pos, e := m.compositionDegreePosition(d)
	if e != nil {
		return e
	}
	coefficient, e := m.core.coefficientCore(p, pos)
	if e != nil {
		return e
	}
	if e = m.compositionNumeral(0, &coefficient, 0, &s.a); e != nil {
		return e
	}
	first, e := m.core.MkConstantTake(&s.a)
	if e != nil {
		return e
	}
	if e = m.compositionAssign(r, first); e != nil {
		return e
	}
	for i := uint32(1); i <= d; i++ {
		pos, e = m.compositionDegreePosition(d - i)
		if e != nil {
			return e
		}
		if pos != ^uint32(0) {
			coefficient, e = m.core.coefficientCore(p, pos)
			if e != nil {
				return e
			}
			if e = m.compositionNumeral(0, &coefficient, 0, &s.a); e != nil {
				return e
			}
		} else if e = m.compositionNumeral(2, nil, 0, &s.a); e != nil {
			return e
		}
		next, e := m.core.mulAddWithReceipt(q, r.root.value, &s.a, &s.arithmetic)
		if e != nil {
			return e
		}
		if e = m.compositionAssign(r, next); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialManager) compositionOutput(r *PolynomialRef) error {
	if r == nil {
		return arena.ErrConfig
	}
	if e := r.open(); e != nil {
		return e
	}
	if r.owner != m.core {
		return arena.ErrConfig
	}
	return nil
}
func (m *MultivariatePolynomialManager) Compose(p, q Polynomial, r *PolynomialRef) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.compositionOutput(r); e != nil {
		return e
	}
	if e := m.compositionUnivariate(p, 4799); e != nil {
		return e
	}
	constant, e := m.core.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		return r.Assign(p)
	}
	if e = m.beginComposition(); e != nil {
		return e
	}
	defer func() { cause = m.finishComposition(cause) }()
	return m.composeBody(p, q, r)
}
func (m *MultivariatePolynomialManager) compositionLinear(x, y uint32, minus bool) (Polynomial, error) {
	s := &m.composition
	s.raw[0] = unumeral.Int{}
	s.rawMask |= 1
	if minus {
		s.raw[1] = unumeral.Int{}
		s.rawMask |= 2
		// Source explicitly uses modular SET(-1), not the unsafe GF2 literal -1.
		before := m.core.integer.PendingAllocationObject()
		raw, e := m.core.integer.SourceAllocator().PendingRoots()
		if e != nil {
			return Polynomial{}, e
		}
		e = m.core.modular.SetInt32(&s.raw[1], -1)
		if before == (umemory.ObjectRef{}) && m.core.integer.PendingAllocationObject() != (umemory.ObjectRef{}) {
			s.numericPending = m.core.integer.PendingAllocationObject()
		}
		if raw == ([3]umemory.Block{}) {
			after, x := m.core.integer.SourceAllocator().PendingRoots()
			if x != nil {
				return Polynomial{}, errors.Join(e, x)
			}
			if after != ([3]umemory.Block{}) {
				s.integerRaw = after
			}
		}
		if e != nil {
			return Polynomial{}, e
		}
		s.raw[2] = unumeral.NewInt(1)
		s.raw[3] = s.raw[1].Move()
		s.rawMask |= 12
	} else {
		s.raw[2], s.raw[3] = unumeral.NewInt(1), unumeral.NewInt(1)
		s.rawMask |= 12
	}
	p, e := m.core.MkLinearTake(s.raw[2:4], []uint32{x, y}, &s.raw[0])
	if e == nil {
		// RAW source numerals have no destructor. These reached source fields were
		// consumed by mutable mk_linear or are inline scalar remnants.
		for i := range s.raw {
			if s.raw[i].NativeCellToken() != 0 {
				return Polynomial{}, ErrPolynomialPending
			}
		}
		s.rawMask = 0
	}
	return p, e
}
func (m *MultivariatePolynomialManager) ComposeXMinusY(p Polynomial, y uint32, r *PolynomialRef) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.compositionOutput(r); e != nil {
		return e
	}
	if e := polynomialCompositionSourceAssert(4830, "is_valid(y)", func() (bool, error) {
		return y < m.core.monomials.nextVar, nil
	}); e != nil {
		return e
	}
	if e := m.compositionUnivariate(p, 4831); e != nil {
		return e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	if y == x {
		// This source early branch has no scoped_degree2pos or numeral
		// constructor. An older stopped lexical context is not read here.
		s := &m.composition
		if s.executing {
			return ErrPolynomialPending
		}
		m.core.compositionOwner = s
		s.executing = true
		defer func() {
			s.executing = false
			if cause != nil {
				s.active = true
				if s.bodyCause == nil {
					s.bodyCause = cause
				}
			}
		}()
		q, e := m.core.Coeff(p, x, 0)
		if e != nil {
			return e
		}
		return m.compositionAssign(r, q)
	}
	if e = m.beginComposition(); e != nil {
		return e
	}
	defer func() { cause = m.finishComposition(cause) }()
	m.composition.localCompleted = true
	q, e := m.compositionLinear(x, y, true)
	if e != nil {
		return e
	}
	if e = m.compositionAssignLocal(q); e != nil {
		return e
	}
	return m.composeBody(p, m.composition.local.value, r)
}
func (m *MultivariatePolynomialManager) ComposeXPlusY(p Polynomial, y uint32, r *PolynomialRef) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.compositionOutput(r); e != nil {
		return e
	}
	if e := polynomialCompositionSourceAssert(4850, "is_valid(y)", func() (bool, error) {
		return y < m.core.monomials.nextVar, nil
	}); e != nil {
		return e
	}
	if e := m.compositionUnivariate(p, 4851); e != nil {
		return e
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	if e = m.beginComposition(); e != nil {
		return e
	}
	defer func() { cause = m.finishComposition(cause) }()
	m.composition.localCompleted = true
	q, e := m.compositionLinear(x, y, false)
	if e != nil {
		return e
	}
	if e = m.compositionAssignLocal(q); e != nil {
		return e
	}
	return m.composeBody(p, m.composition.local.value, r)
}
func (m *MultivariatePolynomialManager) ComposeXMinusConstant(p Polynomial, a *unumeral.Int, r *PolynomialRef) (cause error) {
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.compositionOutput(r); e != nil {
		return e
	}
	if e := m.compositionUnivariate(p, 4872); e != nil {
		return e
	}
	zero, e := m.core.modular.IsZero(a)
	if e != nil {
		return e
	}
	if zero {
		return r.Assign(p)
	}
	x, e := m.core.MaxVariable(p)
	if e != nil {
		return e
	}
	if e = m.beginComposition(); e != nil {
		return e
	}
	defer func() { cause = m.finishComposition(cause) }()
	s := &m.composition
	s.localCompleted = true
	s.rawMask = 3
	if e = m.compositionNumeral(0, a, 0, &s.raw[0]); e != nil {
		return e
	}
	if e = m.compositionNumeral(3, nil, 1, &s.raw[1]); e != nil {
		return e
	}
	if e = m.compositionNumeral(1, nil, 0, &s.raw[0]); e != nil {
		return e
	}
	q, e := m.core.MkUnivariateTake(x, 1, s.raw[:2])
	if e != nil {
		return e
	}
	// Explicit RAW source Del actions occur before returning x-c to its ref.
	if _, e = upolynomialSourceNumeralDel(m.core.integer, &s.raw[0]); e != nil {
		s.stopped = true
		s.sourceCause = e
		return e
	}
	s.rawMask &^= 1
	if _, e = upolynomialSourceNumeralDel(m.core.integer, &s.raw[1]); e != nil {
		s.stopped = true
		s.sourceCause = e
		return e
	}
	s.rawMask &^= 2
	if e = m.compositionAssignLocal(q); e != nil {
		return e
	}
	return m.composeBody(p, s.local.value, r)
}

func (m *MultivariatePolynomialManager) compositionHasOrphans() bool {
	for _, a := range m.composition.orphans {
		if a != (Monomial{}) {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) CleanupComposition() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.composition
	c := m.core
	if s.executing {
		return ErrPolynomialPending
	}
	if e := c.cleanupArithmeticInvocation(&s.arithmetic, false); e != nil {
		return e
	}
	if s.constantGeneration != 0 {
		if c.constantGeneration == s.constantGeneration && c.constantScope.active {
			if e := c.cleanupConstantScope(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.constantGeneration = 0
	}
	// Owning abort never replays a stopped source leaf or asserts completion of
	// an incomplete scoped_degree2pos constructor.
	if s.aCompleted {
		if e := c.integer.Del(&s.a); e != nil {
			s.cleanupCause = e
			return e
		}
		s.aCompleted = false
	}
	for i := len(s.raw) - 1; i >= 0; i-- {
		mask := uint8(1) << uint(i)
		if s.rawMask&mask != 0 {
			if e := c.integer.Del(&s.raw[i]); e != nil {
				s.cleanupCause = e
				return e
			}
			s.rawMask &^= mask
		}
	}
	if s.degreeCompleted {
		if e := m.resetCompositionDegrees(s.degreeInput); e != nil {
			s.cleanupCause = e
			return e
		}
		s.degreeCompleted = false
	} else if s.degreeStored != 0 {
		for s.degreeStored > 0 {
			i := s.degreeStored - 1
			a, e := c.monomialCore(s.degreeInput, i)
			if e != nil {
				return e
			}
			d, e := c.monomials.degreeCore(a)
			if e != nil {
				return e
			}
			current, e := m.compositionDegreePosition(d)
			if e != nil {
				return e
			}
			if current != i {
				return ErrPolynomialPending
			}
			m.degree2pos[d] = ^uint32(0)
			s.degreeStored--
		}
	}
	if s.localCompleted {
		if e := s.local.close(c); e != nil {
			s.cleanupCause = e
			return e
		}
		s.localCompleted = false
	}
	if s.freshResult != (Polynomial{}) {
		if e := c.CleanupValue(&s.freshResult); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	if s.coreGenerated != (Monomial{}) {
		if c.generatedActive && c.generatedMonomial == s.coreGenerated {
			if e := c.cleanupGeneratedMonomial(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.coreGenerated = Monomial{}
	}
	if s.generated != (Monomial{}) {
		if e := c.monomials.CleanupValue(&s.generated); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	for s.drain < uint32(len(s.orphans)) {
		a := &s.orphans[s.drain]
		if *a != (Monomial{}) {
			if e := c.monomials.CleanupValue(a); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.drain++
	}
	if s.calleeGeneration != 0 {
		creation := &c.monomials.creation
		if creation.generation == s.calleeGeneration && creation.parent == c {
			if e := c.monomials.CleanupCreation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		if c.generatedCalleeGeneration == s.calleeGeneration {
			c.generatedCalleePending = false
		}
		s.calleeGeneration = 0
	}
	if s.factoryGeneration != 0 {
		if c.pending.generation == s.factoryGeneration && c.pending.compositionParent == s {
			if e := c.cleanupPendingCreation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.factoryGeneration = 0
	}
	if s.singleton != (Monomial{}) {
		if c.singletonReference && c.singletonMonomial == s.singleton && c.singletonGeneration == s.singletonGeneration {
			if e := c.monomials.DecRef(s.singleton); e != nil {
				s.cleanupCause = e
				return e
			}
			c.singletonReference = false
			c.singletonMonomial = Monomial{}
		}
		s.singleton = Monomial{}
	}
	if s.numericPending != (umemory.ObjectRef{}) {
		if c.integer.PendingAllocationObject() == s.numericPending {
			if e := c.integer.CleanupPendingAllocation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.numericPending = umemory.ObjectRef{}
	}
	for i := 0; i < 2; i++ {
		roots := &s.integerRaw
		allocator := c.integer.SourceAllocator()
		if i == 1 {
			roots = &s.monomialRaw
			allocator = c.monomials.allocator
		}
		if *roots != ([3]umemory.Block{}) {
			current, e := allocator.PendingRoots()
			if e != nil {
				return e
			}
			if current == *roots {
				e = allocator.CleanupPending()
				after, x := allocator.PendingRoots()
				*roots = after
				if e != nil {
					s.cleanupCause = e
					return errors.Join(e, x)
				}
				if x != nil {
					return x
				}
			}
			*roots = [3]umemory.Block{}
		}
	}
	if e := c.cleanupArithmeticInvocation(&s.arithmetic, true); e != nil {
		return e
	}
	s.active = false
	s.cleanupCause = nil
	return nil
}
func (m *MultivariatePolynomialManager) closeCompositionMetadata() error {
	if m.composition.active || m.compositionHasOrphans() {
		return ErrPolynomialPending
	}
	m.composition.orphans = nil
	m.composition.drain = 0
	return nil
}
func (m *MultivariatePolynomialManager) finishCompositionResult(p Polynomial, e error) (Polynomial, error) {
	e = m.finishComposition(e)
	s := &m.composition
	if e != nil {
		return Polynomial{}, e
	}
	if s.freshResult == p {
		s.freshResult = Polynomial{}
	}
	if s.generated == (Monomial{}) && s.factoryGeneration == 0 && s.calleeGeneration == 0 && s.numericPending == (umemory.ObjectRef{}) && s.integerRaw == ([3]umemory.Block{}) && s.monomialRaw == ([3]umemory.Block{}) && s.rawMask == 0 {
		s.active = false
	}
	return p, nil
}

// Observe only around an actually reached numeric manager call, without
// converting the allocator's retained directory into operation ownership.
type polynomialCompositionNumericSnapshot struct {
	factors       *PolynomialFactors
	factorsBefore polynomialNumericRoots
	scope         *polynomialCompositionScope
	object        umemory.ObjectRef
	raw           [3]umemory.Block
}

func (c *MultivariatePolynomialCore) compositionNumericBefore() (polynomialCompositionNumericSnapshot, error) {
	before := polynomialCompositionNumericSnapshot{}
	if f := c.factorsOwner; f != nil && f.executing && c.callbackDepth == 0 {
		var e error
		before.factors = f
		before.factorsBefore, e = polynomialNumericBefore(c.integer)
		if e != nil {
			return before, e
		}
	}
	s := c.compositionOwner
	if s == nil || !s.executing {
		return before, nil
	}
	raw, e := c.integer.SourceAllocator().PendingRoots()
	before.scope = s
	before.object = c.integer.PendingAllocationObject()
	before.raw = raw
	return before, e
}
func (c *MultivariatePolynomialCore) compositionNumericAfter(before polynomialCompositionNumericSnapshot, cause error) error {
	if f := before.factors; f != nil && f.executing && c.callbackDepth == 0 {
		cause = errors.Join(cause, f.numeric.after(c.integer, before.factorsBefore))
	}
	s := before.scope
	if s == nil || !s.executing {
		return cause
	}
	if before.object == (umemory.ObjectRef{}) && c.integer.PendingAllocationObject() != (umemory.ObjectRef{}) {
		s.numericPending = c.integer.PendingAllocationObject()
	}
	if before.raw == ([3]umemory.Block{}) {
		after, e := c.integer.SourceAllocator().PendingRoots()
		if e != nil {
			return errors.Join(cause, e)
		}
		if after != ([3]umemory.Block{}) {
			s.integerRaw = after
		}
	}
	return cause
}
func (c *MultivariatePolynomialCore) compositionNumericCall(op uint8, a, b, d, out *unumeral.Int) error {
	before, e := c.compositionNumericBefore()
	if e != nil {
		return e
	}
	switch op {
	case 0:
		e = c.modular.Copy(out, a)
	case 1:
		e = c.modular.Add(a, b, out)
	case 2:
		e = c.modular.Mul(a, b, out)
	case 3:
		e = c.modular.AddMul(a, b, d, out)
	case 4:
		e = c.modular.Div(a, b, out)
	default:
		return arena.ErrBounds
	}
	return c.compositionNumericAfter(before, e)
}
