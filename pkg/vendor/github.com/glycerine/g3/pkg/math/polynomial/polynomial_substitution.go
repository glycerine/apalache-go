package polynomial

// Source: src/math/polynomial/polynomial.cpp:6237-6425,7364-7377;
// src/math/polynomial/polynomial.h:942-966,1126-1130.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (a *polynomialSubstitutionArray) Contains(x uint32) (bool, error) {
	p, e := a.positions.Position(x)
	return p != ^uint32(0), e
}
func (a *polynomialSubstitutionArray) Rational(x uint32) (*unumeral.Rational, error) {
	p, e := a.positions.Position(x)
	if e != nil {
		return nil, e
	}
	if uint64(p) >= uint64(len(a.values)) {
		return nil, arena.ErrBounds
	}
	return &a.values[p], nil
}

func (m *MultivariatePolynomialCore) initSubstitutionMembers() {
	d := &m.substitution
	d.max.core = m
	d.max.runtime = m.memory
	d.max.self = &d.max
	d.max.constructed = true
	variableEmbedded(&d.max.maxDegree, m.memory, true)
	variableEmbedded(&d.max.variables, m.memory, true)
	d.positions.runtime = m.memory
	d.positions.self = &d.positions
	d.positions.constructed = true
	variableEmbedded(&d.positions.positions, m.memory, true)
	d.array.positions = &d.positions
}
func (m *MultivariatePolynomialCore) beginSubstitution(kind uint8) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.substitutionNumericPending() || m.substitution.scope.invocation == ^uint64(0) || m.substitution.scope.active || m.pending.substitutionOwned || m.powerLocal.substitutionOwned || m.substitution.borrowedReceipt != 0 || m.substitution.generatedCalleeReceipt != 0 || m.substitution.generatedReceipt != (Monomial{}) {
		return ErrPolynomialPending
	}
	m.substitution.bodyCause, m.substitution.cleanupCause = nil, nil
	m.substitution.scope = polynomialSubstitutionScope{invocation: m.substitution.scope.invocation + 1, active: true, executing: true, kind: kind}
	return nil
}
func (m *MultivariatePolynomialCore) finishSubstitution(cause error) error {
	s := &m.substitution.scope
	// A retained stopped source destructor is an owning continuation, not a
	// still executing body. Later unrelated calls cannot inherit its receipts.
	s.executing = false
	s.bodyCause = cause
	m.substitution.bodyCause = cause
	cleanup := m.substitutionCleanup(false)
	if cleanup != nil {
		if s.sourceCause == nil {
			s.sourceCause = cleanup
		}
		s.cleanupCause = s.sourceCause
		if m.substitution.cleanupCause == nil {
			m.substitution.cleanupCause = s.sourceCause
		}
	}
	return errors.Join(cause, cleanup)
}

// Ordinary cleanup visits only completed constructors, in reverse source
// lexical order. Failed copy constructor roots/unjoined assignment INC remain
// named until a distinct owning abort; no destructor is invented for them.
func (m *MultivariatePolynomialCore) substitutionCleanup(owning bool) error {
	return m.substitutionCleanupPhase(owning, owning)
}
func (m *MultivariatePolynomialCore) substitutionCleanupPhase(owning, abort bool) error {
	s := &m.substitution.scope
	if !s.active {
		return nil
	}
	if !owning {
		if cause := s.traceInsertion.SourceCause(); cause != nil {
			return cause
		}
	} else if s.traceInsertion.Active() {
		if e := m.modular.CleanupSourceString(&s.traceInsertion); e != nil {
			return e
		}
	}
	if s.cleanupCause != nil && !owning {
		return s.cleanupCause
	}
	b := &m.som
	if m.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active {
		if owning {
			if e := b.finishNormalizeNumeralPhase(true, false); e != nil {
				return e
			}
		} else if b.gcdCleanupCause != nil {
			return b.gcdCleanupCause
		}
	}
	phase := &s.stage
	resetStopped := &s.resetStopped
	if owning {
		if !s.ownStarted {
			s.ownStage = s.stage
			s.ownResetStopped = s.resetStopped
			s.ownStarted = true
		}
		phase = &s.ownStage
		resetStopped = &s.ownResetStopped
	}
	if s.kind == 3 {
		if *phase == 0 {
			if s.productConstructed {
				if e := s.product.closeValue(m, owning); e != nil {
					return e
				}
			}
			*phase = 1
		}
		if *phase == 1 {
			if s.monomialConstructed {
				if e := s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, owning); e != nil {
					return e
				}
			}
			*phase = 2
		}
		// The completed empty polynomial_ref_buffer has no pushed elements and
		// no dynamic backing; its genuine destructor reaches no callback.
		if *phase == 2 {
			if s.q1Constructed {
				if e := s.q1.closeValue(m, owning); e != nil {
					return e
				}
			}
			*phase = 3
		}
		if *phase == 3 {
			if s.p1Constructed {
				if e := s.p1.closeValue(m, owning); e != nil {
					return e
				}
			}
			*phase = 4
		}
		if !owning && (s.product.assigning || s.monomial.assigning || s.q1.assigning || s.p1.assigning) {
			return nil
		}
		if owning {
			if e := s.product.abort(m); e != nil {
				return e
			}
			if e := s.monomial.drop(m, &s.monomial.incoming, &s.monomial.incomingStarted, true); e != nil {
				return e
			}
			s.monomial.assigning = false
			if e := s.q1.abort(m); e != nil {
				return e
			}
			if e := s.p1.abort(m); e != nil {
				return e
			}
		}
	} else {
		if *phase == 0 {
			if s.tmpConstructed {
				var e error
				if owning {
					e = m.integer.Del(&s.tmp)
				} else {
					e = unumeral.SourceIntegerDel(m.integer, &s.tmp)
				}
				if e != nil {
					return e
				}
			}
			*phase = 1
		}
		if *phase == 1 {
			if s.newAConstructed {
				var e error
				if owning {
					e = m.integer.Del(&s.newA)
				} else {
					e = unumeral.SourceIntegerDel(m.integer, &s.newA)
				}
				if e != nil {
					return e
				}
			}
			*phase = 2
		}
		if *phase == 2 {
			if s.maxConstructed && !*resetStopped {
				if e := m.substitution.max.Reset(); e != nil {
					*resetStopped = true
					return e
				}
			}
			*phase = 3
		}
		if *phase == 3 {
			if s.posConstructed && !*resetStopped {
				if e := m.resetSubstitutionPositions(s); e != nil {
					*resetStopped = true
					return e
				}
			}
			*phase = 4
		}
	}
	if s.returned != (Polynomial{}) && !owning {
		return nil
	}
	if owning && !abort {
		return nil
	}
	if owning && s.returned != (Polynomial{}) {
		if e := m.CleanupValue(&s.returned); e != nil {
			return e
		}
	}
	if owning && m.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active {
		if e := b.finishNormalizeNumeralPhase(true, true); e != nil {
			return e
		}
	}
	if !owning && m.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active {
		return nil
	}
	if owning && abort && s.traceMonomial != (Monomial{}) {
		if e := m.monomials.CleanupValue(&s.traceMonomial); e != nil {
			return e
		}
	}
	if s.traceInsertion.Active() || s.traceMonomial != (Monomial{}) {
		return nil
	}
	s.active = false
	s.executing = false
	m.substitution.array.values = nil
	return nil
}
func (m *MultivariatePolynomialCore) CleanupSubstitution() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.substitution.scope.executing {
		return ErrPolynomialPending
	}
	if m.substitution.scope.modularPowerPending {
		if e := m.modular.CleanupPowerTemporary(); e != nil {
			return e
		}
		m.substitution.scope.modularPowerPending = false
	}
	if e := m.substitution.scope.powerNumeric.roots.Cleanup(m.integer); e != nil {
		return e
	}
	if e := m.substitutionCleanup(true); e != nil {
		if m.substitution.scope.ownCause == nil {
			m.substitution.scope.ownCause = e
		}
		if m.substitution.scope.cleanupCause == nil {
			m.substitution.scope.cleanupCause = e
		}
		if m.substitution.cleanupCause == nil {
			m.substitution.cleanupCause = e
		}
		return e
	}
	// The actual pending objects themselves carry the receipt; unrelated older
	// callee state never acquires this invocation's owning authority.
	if m.powerLocal.substitutionOwned {
		if e := m.CleanupPolynomialPower(); e != nil {
			return e
		}
	}
	if m.pending.substitutionOwned {
		if e := m.cleanupCreation(); e != nil {
			return e
		}
	}
	d := &m.substitution
	if d.generatedReceipt != (Monomial{}) {
		if m.generatedActive && m.generatedMonomial == d.generatedReceipt {
			if e := m.cleanupGeneratedMonomial(); e != nil {
				return e
			}
		}
		d.generatedReceipt = Monomial{}
	}
	if d.generatedCalleeReceipt != 0 {
		if m.monomials.creation.generation == d.generatedCalleeReceipt && m.monomials.creation.parent == m {
			if e := m.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		d.generatedCalleeReceipt = 0
	}
	if d.borrowedReceipt != 0 {
		if m.monomials.borrowedTemp == &m.tmp1 && m.monomials.borrowedGeneration == d.borrowedReceipt && m.monomials.borrowedParent == m {
			if e := m.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		d.borrowedReceipt = 0
	}
	m.substitution.bodyCause, m.substitution.cleanupCause = nil, nil
	return nil
}
func (m *MultivariatePolynomialCore) substitutionModularPower(a *unumeral.Int, k uint32, out *unumeral.Int) error {
	before := m.modular.PowerPending
	numeric := &m.substitution.scope.powerNumeric
	numeric.owner = m
	numeric.hooks.Generation = m.substitution.scope.invocation
	numeric.hooks.Policy = numeric
	context := unumeral.NumericAllocationContext{}.WithCaller(&numeric.hooks)
	e := m.modular.WithAllocationContext(context).Power(a, k, out)
	if !before && m.modular.PowerPending {
		m.substitution.scope.modularPowerPending = true
	}
	return e
}
func (m *MultivariatePolynomialCore) generatedSubstitutionMonomial() (Monomial, error) {
	if m.generatedActive {
		return Monomial{}, ErrPolynomialPending
	}
	outcome := monomialFactoryOutcome{}
	freshBinding := m.monomials.borrowedTemp == nil && m.monomials.creation.stage == 0
	a, e := m.monomials.internBorrowedTempCreated(&m.tmp1, &outcome)
	if e != nil && freshBinding && m.monomials.borrowedTemp == &m.tmp1 {
		m.substitution.borrowedReceipt = m.monomials.borrowedGeneration
		m.monomials.borrowedParent = m
	}
	if e != nil && freshBinding && outcome.started {
		m.recordGeneratedCallee(outcome)
	}
	if e == nil && outcome.created {
		m.generatedMonomial = a
		m.generatedActive = true
		m.substitution.generatedReceipt = a
	}
	return a, e
}
func (m *MultivariatePolynomialCore) addSubstitutionTerm(a *unumeral.Int, term Monomial) error {
	consumed := false
	e := m.som.addConsumed(a, term, false, &consumed)
	if consumed && m.generatedActive && m.generatedMonomial == term {
		m.transferGeneratedMonomial()
		m.substitution.generatedReceipt = Monomial{}
	}
	if e == nil && !consumed && m.generatedActive && m.generatedMonomial == term {
		if e := m.retainUnreferencedSubstitutionMonomial(term); e != nil {
			return e
		}
		m.transferGeneratedMonomial()
		m.substitution.generatedReceipt = Monomial{}
	}
	if m.substitution.scope.traceMonomial == term {
		if consumed {
			m.substitution.scope.traceMonomial = Monomial{}
		} else if e == nil {
			if err := m.retainUnreferencedSubstitutionMonomial(term); err != nil {
				return err
			}
			m.substitution.scope.traceMonomial = Monomial{}
		}
	}

	return e
}

func (m *MultivariatePolynomialCore) retainUnreferencedSubstitutionMonomial(term Monomial) error {
	d := &m.substitution
	l, guard := m.monomials.location(term)
	if guard != nil {
		return guard
	}
	// Native storage retains the actual unconsumed root after publication.
	index := uint32(len(d.orphans))
	old := d.orphans
	d.orphans = append(d.orphans, term)
	if len(old) != 0 && &old[0] != &d.orphans[0] {
		clear(old)
	}
	l.unreferencedCohort, l.unreferencedSlot = d, index
	return nil
}

// Source integer-array and single-value overloads. Raw arrays remain borrowed;
// there is no copied-value staging or global component preflight.
func (m *MultivariatePolynomialCore) SubstituteInteger(p Polynomial, x uint32, v *unumeral.Int) (Polynomial, error) {
	return m.substituteIntegerWithReceipt(p, x, v, nil)
}
func (m *MultivariatePolynomialCore) substituteIntegerWithReceipt(p Polynomial, x uint32, v *unumeral.Int, receipt *polynomialSubstitutionInvocationReceipt) (Polynomial, error) {
	if e := m.check(); e != nil {
		return Polynomial{}, e
	}
	if v == nil {
		return Polynomial{}, arena.ErrConfig
	}
	// The scalar overload is the same engine with one actual source pointer.
	return m.substituteIntegersWithReceipt(p, 1, PolynomialVariablesView{}, nil, &x, v, receipt)
}
func (m *MultivariatePolynomialCore) SubstituteIntegers(p Polynomial, n uint32, xs PolynomialVariablesView, values []unumeral.Int) (Polynomial, error) {
	return m.substituteIntegers(p, n, xs, values, nil, nil)
}
func (m *MultivariatePolynomialCore) substituteIntegers(p Polynomial, n uint32, xs PolynomialVariablesView, values []unumeral.Int, oneX *uint32, oneValue *unumeral.Int) (result Polynomial, err error) {
	return m.substituteIntegersWithReceipt(p, n, xs, values, oneX, oneValue, nil)
}
func (m *MultivariatePolynomialCore) substituteIntegersWithReceipt(p Polynomial, n uint32, xs PolynomialVariablesView, values []unumeral.Int, oneX *uint32, oneValue *unumeral.Int, receipt *polynomialSubstitutionInvocationReceipt) (result Polynomial, err error) {
	if e := m.beginSubstitution(1); e != nil {
		return Polynomial{}, e
	}
	s := &m.substitution.scope
	if receipt != nil {
		*receipt = polynomialSubstitutionInvocationReceipt{m, s, s.invocation}
	}
	if e := m.traceIntegerSubstitutionEntry(n, xs, values, oneX, oneValue); e != nil {
		return Polynomial{}, m.finishSubstitution(e)
	}
	// The one-scalar header overload has one call-local var address. It cannot
	// escape this frame. The shared array algorithm reads that pointer directly.
	if oneX != nil {
		// A separate concrete source var pointer adapter avoids allocating a fake
		// Runtime array for a source automatic unsigned scalar.
		if e := m.substitution.positions.initSingle(*oneX); e != nil {
			return Polynomial{}, m.finishSubstitution(e)
		}
	} else if e := m.substitution.positions.Init(n, xs); e != nil {
		return Polynomial{}, m.finishSubstitution(e)
	}
	s.posConstructed = true
	s.xs = xs
	s.xsCount = n
	if oneX != nil {
		s.xsCount = 0
		s.single = true
		s.singleX = *oneX
	}
	s.newAConstructed = true
	s.tmpConstructed = true
	defer func() {
		if err == nil {
			s.returned = result
		}
		err = m.finishSubstitution(err)
		if err == nil {
			m.substitution.array.values = nil
			*s = polynomialSubstitutionScope{invocation: s.invocation}
		}
	}()
	if e := m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	size, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < size; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		info, e := m.monomials.sizeInfoCore(term)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.modular.Copy(&s.newA, &a); e != nil {
			return Polynomial{}, e
		}
		newSize := uint32(0)
		for j := uint32(0); j < info.Size; j++ {
			x, e := m.monomials.varCore(term, j)
			if e != nil {
				return Polynomial{}, e
			}
			k, e := m.monomials.degreeAtCore(term, j)
			if e != nil {
				return Polynomial{}, e
			}
			pos, e := m.substitution.positions.Position(x)
			if e != nil {
				return Polynomial{}, e
			}
			if pos != ^uint32(0) {
				var v *unumeral.Int
				if oneX != nil {
					if pos != 0 {
						return Polynomial{}, arena.ErrBounds
					}
					v = oneValue
				} else {
					if uint64(pos) >= uint64(len(values)) {
						return Polynomial{}, arena.ErrBounds
					}
					v = &values[pos]
				}
				if e = m.substitutionModularPower(v, k, &s.tmp); e != nil {
					return Polynomial{}, e
				}
				if e = m.modular.Mul(&s.tmp, &s.newA, &s.newA); e != nil {
					return Polynomial{}, e
				}
			} else {
				if e = polynomialSubstitutionSourceAssert(6388, "var2pos(x) == UINT_MAX", func() (bool, error) {
					again, e := m.substitution.positions.Position(x)
					if e != nil {
						return false, e
					}
					return again == ^uint32(0), nil
				}); e != nil {
					return Polynomial{}, e
				}
				power, e := m.monomials.powerCore(term, j)
				if e != nil {
					return Polynomial{}, e
				}
				if e = m.monomials.tempSetPower(&m.tmp1, newSize, power); e != nil {
					return Polynomial{}, e
				}
				newSize++
			}
		}
		if e = m.monomials.tempSetSize(&m.tmp1, newSize); e != nil {
			return Polynomial{}, e
		}
		if e := m.traceIntegerSubstitutionTerm(p, i, term); e != nil {
			return Polynomial{}, e
		}
		mono, e := m.generatedSubstitutionMonomial()
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.addSubstitutionTerm(&s.newA, mono); e != nil {
			return Polynomial{}, e
		}
	}
	return m.som.mk(false)
}

func (d *PolynomialVariablePositions) initSingle(x uint32) error {
	if e := d.open(); e != nil {
		return e
	}
	if e := polynomialSubstitutionSourceAssert(6313, "m_pos.get(xs[i], UINT_MAX) == UINT_MAX", func() (bool, error) {
		p, e := variableGet(&d.positions, x, ^uint32(0))
		if e != nil {
			return false, e
		}
		return p == ^uint32(0), nil
	}); e != nil {
		return e
	}
	return variableSetFilled(&d.positions, x, 0, ^uint32(0))
}
func (m *MultivariatePolynomialCore) resetSubstitutionPositions(s *polynomialSubstitutionScope) error {
	if !s.single {
		return m.substitution.positions.Reset(s.xsCount, s.xs)
	}
	if e := polynomialSubstitutionSourceAssert(6320, "m_pos.get(xs[i], UINT_MAX) != UINT_MAX", func() (bool, error) {
		p, e := m.substitution.positions.Position(s.singleX)
		if e != nil {
			return false, e
		}
		return p != ^uint32(0), nil
	}); e != nil {
		return e
	}
	return variablePut(&m.substitution.positions.positions, s.singleX, ^uint32(0))
}
func (m *MultivariatePolynomialCore) closeSubstitutionMembers() error {
	if e := m.CleanupUnreferencedSubstitutionMonomials(); e != nil {
		return e
	}
	m.substitution.orphans = nil
	if e := m.substitution.positions.CloseChecked(); e != nil {
		return e
	}
	if e := m.substitution.max.CloseChecked(); e != nil {
		return e
	}
	if e := m.substitution.found.close(); e != nil {
		return e
	}
	return m.monomials.tempCleanup(&m.tmp1)
}

// The array wrapper constructs scoped_var_pos before entering the same real
// rational-map algorithm. No rational snapshots or numeric prevalidation.
func (m *MultivariatePolynomialCore) SubstituteRationals(p Polynomial, n uint32, xs PolynomialVariablesView, values []unumeral.Rational) (Polynomial, error) {
	if e := m.beginSubstitution(2); e != nil {
		return Polynomial{}, e
	}
	s := &m.substitution.scope
	s.xs = xs
	s.xsCount = n
	if e := m.substitution.positions.Init(n, xs); e != nil {
		return Polynomial{}, m.finishSubstitution(e)
	}
	s.posConstructed = true
	m.substitution.array.values = values
	return m.substituteRationalMap(p, &m.substitution.array)
}
func (m *MultivariatePolynomialCore) SubstituteRationalMap(p Polynomial, x2v PolynomialRationalSubstitution) (Polynomial, error) {
	return m.substituteRationalForAlgebraic(p, x2v, nil)
}
func (m *MultivariatePolynomialCore) substituteRationalForAlgebraic(p Polynomial, x2v PolynomialRationalSubstitution, caller *algebraicSignSubstitutionScope) (Polynomial, error) {
	if e := m.beginSubstitution(2); e != nil {
		return Polynomial{}, e
	}
	m.substitution.scope.algebraicCaller = caller
	if caller != nil {
		caller.core = m
	}
	return m.substituteRationalMap(p, x2v)
}
func (m *MultivariatePolynomialCore) substituteRationalMap(p Polynomial, x2v PolynomialRationalSubstitution) (result Polynomial, err error) {
	s := &m.substitution.scope
	defer func() {
		err = m.finishSubstitution(err)
		if err == nil {
			m.substitution.array.values = nil
			*s = polynomialSubstitutionScope{invocation: s.invocation}
		}
	}()
	if e := m.substitution.max.Init(p); e != nil {
		return Polynomial{}, e
	}
	s.maxConstructed = true
	count, e := m.substitution.max.NumVariables()
	if e != nil {
		return Polynomial{}, e
	}
	xs, e := m.substitution.max.Vars()
	if e != nil {
		return Polynomial{}, e
	}
	found := false
	for i := uint32(0); i < count; i++ {
		x, e := xs.At(i)
		if e != nil {
			return Polynomial{}, e
		}
		has, e := x2v.Contains(x)
		if e != nil {
			return Polynomial{}, e
		}
		if has {
			k, e := m.substitution.max.Degree(x)
			if e != nil {
				return Polynomial{}, e
			}
			if k > 0 {
				found = true
				break
			}
		}
	}
	if !found {
		return p, nil
	}
	s.newAConstructed = true
	s.tmpConstructed = true
	vars, e := m.monomials.NumVars()
	if e != nil {
		return Polynomial{}, e
	}
	if e = m.substitution.found.reserve(vars); e != nil {
		return Polynomial{}, e
	}
	if e = m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	size, e := m.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	for i := uint32(0); i < size; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		info, e := m.monomials.sizeInfoCore(term)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.modular.Copy(&s.newA, &a); e != nil {
			return Polynomial{}, e
		}
		newSize := uint32(0)
		for j := uint32(0); j < info.Size; j++ {
			x, e := m.monomials.varCore(term, j)
			if e != nil {
				return Polynomial{}, e
			}
			k, e := m.monomials.degreeAtCore(term, j)
			if e != nil {
				return Polynomial{}, e
			}
			has, e := x2v.Contains(x)
			if e != nil {
				return Polynomial{}, e
			}
			if has {
				max, e := m.substitution.max.Degree(x)
				if e != nil {
					return Polynomial{}, e
				}
				if e = m.substitution.found.put(x, true); e != nil {
					return Polynomial{}, e
				}
				value, e := x2v.Rational(x)
				if e != nil {
					return Polynomial{}, e
				}
				if value == nil {
					return Polynomial{}, arena.ErrConfig
				}
				if e = m.substitutionModularPower(&(*value.BorrowNumerator()), k, &s.tmp); e != nil {
					return Polynomial{}, e
				}
				if e = m.modular.Mul(&s.tmp, &s.newA, &s.newA); e != nil {
					return Polynomial{}, e
				}
				if k < max {
					if e = m.substitutionModularPower(&(*value.BorrowDenominator()), max-k, &s.tmp); e != nil {
						return Polynomial{}, e
					}
					if e = m.modular.Mul(&s.tmp, &s.newA, &s.newA); e != nil {
						return Polynomial{}, e
					}
				}
			} else {
				power, e := m.monomials.powerCore(term, j)
				if e != nil {
					return Polynomial{}, e
				}
				if e = m.monomials.tempSetPower(&m.tmp1, newSize, power); e != nil {
					return Polynomial{}, e
				}
				newSize++
			}
		}
		for j := uint32(0); j < count; j++ {
			x, e := xs.At(j)
			if e != nil {
				return Polynomial{}, e
			}
			seen, e := m.substitution.found.at(x)
			if e != nil {
				return Polynomial{}, e
			}
			if seen {
				continue
			}
			has, e := x2v.Contains(x)
			if e != nil {
				return Polynomial{}, e
			}
			if has {
				value, e := x2v.Rational(x)
				if e != nil {
					return Polynomial{}, e
				}
				if value == nil {
					return Polynomial{}, arena.ErrConfig
				}
				k, e := m.substitution.max.Degree(x)
				if e != nil {
					return Polynomial{}, e
				}
				// Selected C++11 profile evaluates map(x) before degree(x), matching the
				// expression's actual retained reference; neither value is snapshotted.
				if e = m.substitutionModularPower(&(*value.BorrowDenominator()), k, &s.tmp); e != nil {
					return Polynomial{}, e
				}
				if e = m.modular.Mul(&s.tmp, &s.newA, &s.newA); e != nil {
					return Polynomial{}, e
				}
			}
		}
		for j := uint32(0); j < info.Size; j++ {
			x, e := m.monomials.varCore(term, j)
			if e != nil {
				return Polynomial{}, e
			}
			if e = m.substitution.found.put(x, false); e != nil {
				return Polynomial{}, e
			}
		}
		zero, e := m.modular.IsZero(&s.newA)
		if e != nil {
			return Polynomial{}, e
		}
		if !zero {
			if e = m.monomials.tempSetSize(&m.tmp1, newSize); e != nil {
				return Polynomial{}, e
			}
			mono, e := m.generatedSubstitutionMonomial()
			if e != nil {
				return Polynomial{}, e
			}
			if e = m.addSubstitutionTerm(&s.newA, mono); e != nil {
				return Polynomial{}, e
			}
		}
	}
	result, err = m.som.mk(true)
	if err == nil {
		s.returned = result
	}
	return
}

func (m *MultivariatePolynomialCore) SubstitutePolynomialFraction(r Polynomial, x uint32, p, q Polynomial, result *PolynomialRef) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if result == nil || result.owner != m {
		return ErrPolynomialOwner
	}
	if e := result.open(); e != nil {
		return e
	}
	md, e := m.Degree(r, x)
	if e != nil {
		return e
	}
	if md == 0 {
		return result.root.assign(m, r)
	}
	if e = m.beginSubstitution(3); e != nil {
		return e
	}
	s := &m.substitution.scope
	defer func() {
		err = m.finishSubstitution(err)
		if err == nil {
			m.substitution.array.values = nil
			*s = polynomialSubstitutionScope{invocation: s.invocation}
		}
	}()
	if e = result.root.assign(m, Polynomial{}); e != nil {
		return e
	}
	s.p1Constructed = true
	s.q1Constructed = true
	size, e := m.readWord(r, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		term, e := m.monomialCore(r, i)
		if e != nil {
			return e
		}
		dm, e := m.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		if e = polynomialSubstitutionSourceAssert(6414, "md >= dm", func() (bool, error) {
			return md >= dm, nil
		}); e != nil {
			return e
		}
		priorGenerated, priorCallee := m.generatedActive, m.generatedCalleePending
		mono, e := m.generatedRemoveVariable(term, x)
		if !priorGenerated && m.generatedActive && m.generatedMonomial == mono {
			m.substitution.generatedReceipt = mono
		}
		if !priorCallee && m.generatedCalleePending {
			m.substitution.generatedCalleeReceipt = m.generatedCalleeGeneration
		}
		if e != nil {
			return e
		}
		if e = s.monomial.assign(m, mono); e != nil {
			return e
		}
		s.monomialConstructed = true
		m.substitution.generatedReceipt = Monomial{}
		if e = m.powerInto(p, dm, &s.p1); e != nil {
			return e
		}
		if e = m.powerInto(q, md-dm, &s.q1); e != nil {
			return e
		}
		// h1126 is an owning COPY constructor in this full expression. It is not
		// a raw polynomial product discarded on an arithmetic or assignment error.
		product, e := m.Mul(s.p1.value, s.q1.value)
		if e != nil {
			return e
		}
		if e = s.product.assign(m, product); e != nil {
			return e
		}
		s.productConstructed = true
		a, e := m.coefficientCore(r, i)
		if e != nil {
			return e
		}
		scaled, e := m.Scale(&a, s.monomial.value, s.product.value)
		if e != nil {
			return e
		}
		if e = s.p1.assign(m, scaled); e != nil {
			return e
		}
		if e = s.product.closeValue(m, false); e != nil {
			s.cleanupCause = e
			return e
		}
		s.productConstructed = false
		if result.root.value != (Polynomial{}) {
			sum, e := m.Add(result.root.value, s.p1.value)
			if e != nil {
				return e
			}
			if e = result.root.assign(m, sum); e != nil {
				return e
			}
		} else if e = result.root.assign(m, s.p1.value); e != nil {
			return e
		}
		if e = s.monomial.drop(m, &s.monomial.value, &s.monomial.decrementStarted, false); e != nil {
			s.cleanupCause = e
			return e
		}
		s.monomialConstructed = false
	}
	return nil
}

func (m *MultivariatePolynomialCore) CleanupUnreferencedSubstitutionMonomials() error {
	if m == nil || m.self != m || m.closed {
		return ErrPolynomialOwner
	}
	d := &m.substitution
	for uint64(d.orphanDrain) < uint64(len(d.orphans)) {
		slot := &d.orphans[d.orphanDrain]
		if *slot != (Monomial{}) {
			l, e := m.monomials.location(*slot)
			if e != nil {
				return e
			}
			if e := m.monomials.CleanupValue(slot); e != nil {
				return e
			}
			l.unreferencedCohort = nil
			l.unreferencedSlot = 0
		}
		d.orphanDrain++
	}
	d.orphans = d.orphans[:0]
	d.orphanDrain = 0
	return nil
}
