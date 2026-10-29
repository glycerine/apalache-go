package polynomial

// Source: src/math/polynomial/polynomial.cpp:3282-3291,6865-6902,7392-7398;
// src/math/polynomial/polynomial.h:991-1000.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// One actual monomial_ref local, constructed only after the coefficient loops.
// Persistent coefficient buffers are source members, not per-call scratch.
type polynomialTranslationScope struct {
	active, executing, constructed bool
	xk                             polynomialDivisionReference
	generated                      Monomial
	factoryGeneration              uint64
	freshResult                    Polynomial
	bodyCause, cleanupCause        error
}

func (m *MultivariatePolynomialCore) initTranslationMember() {
	v := &m.translateBuffers
	v.self = v
	v.storage = m
}
func (m *MultivariatePolynomialCore) beginTranslation() error {
	if e := m.check(); e != nil {
		return e
	}
	if m.translation.active {
		return ErrPolynomialPending
	}
	m.translation = polynomialTranslationScope{active: true, executing: true}
	return nil
}
func (m *MultivariatePolynomialCore) finishTranslation(cause error) error {
	s := &m.translation
	s.bodyCause = cause
	s.executing = false
	if s.constructed {
		if e := s.xk.drop(m, &s.xk.value, &s.xk.decrementStarted, false); e != nil {
			s.cleanupCause = e
			return errors.Join(cause, e)
		}
	}
	ownsGenerated := s.generated != (Monomial{}) && m.generatedActive && m.generatedMonomial == s.generated
	ownsFactory := s.factoryGeneration != 0 && m.generatedCalleeGeneration == s.factoryGeneration
	ownsCell := m.pending.translationOwned && (m.pending.raw != (umemory.ObjectRef{}) || m.pending.value != (Polynomial{}))
	if !ownsGenerated && !ownsFactory && !ownsCell && !s.xk.assigning {
		// A normal return transfers the completed raw pointer to the caller. A
		// body error retains a genuinely constructed return object for owning abort.
		if cause == nil || s.freshResult == (Polynomial{}) {
			m.translation = polynomialTranslationScope{}
		}
	}
	return cause
}

// CleanupTranslation acknowledges only the actual source locals/callees this
// call started. Persistent source coefficient buffers keep their completed
// prefixes, including real pending children, until their owner's destruction.
func (m *MultivariatePolynomialCore) CleanupTranslation() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	s := &m.translation
	if s.constructed {
		if e := s.xk.drop(m, &s.xk.value, &s.xk.decrementStarted, true); e != nil {
			s.cleanupCause = e
			return e
		}
		if e := s.xk.drop(m, &s.xk.incoming, &s.xk.incomingStarted, true); e != nil {
			s.cleanupCause = e
			return e
		}
		s.xk.assigning = false
	}
	if s.generated != (Monomial{}) && m.generatedActive && m.generatedMonomial == s.generated {
		if e := m.cleanupGeneratedMonomial(); e != nil {
			return e
		}
	}
	if s.factoryGeneration != 0 && m.generatedCalleeGeneration == s.factoryGeneration && m.monomials.creation.generation == s.factoryGeneration && m.monomials.creation.parent == m {
		if e := m.monomials.CleanupCreation(); e != nil {
			return e
		}
	}
	if m.pending.translationOwned {
		if e := m.cleanupCreation(); e != nil {
			return e
		}
	}
	if s.freshResult != (Polynomial{}) {
		if e := m.CleanupValue(&s.freshResult); e != nil {
			return e
		}
	}
	m.translation = polynomialTranslationScope{}
	return nil
}
func (m *MultivariatePolynomialCore) Coefficients(p Polynomial, x uint32, cs *PolynomialSOMVector) (err error) {
	if e := m.beginTranslation(); e != nil {
		return e
	}
	defer func() { err = m.finishTranslation(err) }()
	return m.coefficientsCore(p, x, cs)
}
func (m *MultivariatePolynomialCore) coefficientsCore(p Polynomial, x uint32, cs *PolynomialSOMVector) error {
	if cs == nil || cs.storage != m {
		return ErrPolynomialOwner
	}
	if e := cs.SetOwner(m); e != nil {
		return e
	}
	sz, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < sz; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		d, e := m.monomials.DegreeOf(term, x)
		if e != nil {
			return e
		}
		c, e := cs.At(d)
		if e != nil {
			return e
		}
		// The source argument evaluation profile reads the coefficient before
		// div_x. Both argument expressions precede the actual c->add call.
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return e
		}
		divided, e := m.generatedRemoveVariable(term, x)
		if e != nil {
			return e
		}
		if e = c.Add(&a, divided); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) Translate(p Polynomial, x uint32, v *unumeral.Int) (result Polynomial, err error) {
	if e := m.beginTranslation(); e != nil {
		return Polynomial{}, e
	}
	defer func() {
		err = m.finishTranslation(err)
		if err != nil {
			result = Polynomial{}
		}
	}()
	deg, e := m.Degree(p, x)
	if e != nil {
		return Polynomial{}, e
	}
	if deg == 0 {
		return p, nil
	}
	if v == nil {
		return Polynomial{}, arena.ErrConfig
	}
	zero, e := m.modular.IsZero(v)
	if e != nil {
		return Polynomial{}, e
	}
	if zero {
		return p, nil
	}
	as := &m.translateBuffers
	if e = as.ResetN(deg + 1); e != nil {
		return Polynomial{}, e
	}
	if e = m.coefficientsCore(p, x, as); e != nil {
		return Polynomial{}, e
	}
	for i := uint32(1); i <= deg; i++ {
		if e = m.checkpoint(); e != nil {
			return Polynomial{}, e
		}
		for k := deg - i; k <= deg-1; k++ {
			a, e := as.At(k)
			if e != nil {
				return Polynomial{}, e
			}
			b, e := as.At(k + 1)
			if e != nil {
				return Polynomial{}, e
			}
			if e = a.AddScaledBuffer(v, b); e != nil {
				return Polynomial{}, e
			}
		}
	}
	s := &m.translation
	s.constructed = true
	if e = m.som.reset(); e != nil {
		return Polynomial{}, e
	}
	for k := uint32(0); k <= deg; k++ {
		term, e := m.generatedVariable(x, k)
		if e != nil {
			return Polynomial{}, e
		}
		if e = s.xk.assign(m, term); e != nil {
			return Polynomial{}, e
		}
		a, e := as.At(k)
		if e != nil {
			return Polynomial{}, e
		}
		one := unumeral.NewInt(1)
		if e = m.som.addSourceInput(&one, s.xk.value, polynomialSOMInput{buffer: a, fromBuffer: true}, true); e != nil {
			return Polynomial{}, e
		}
	}
	if e = as.ResetN(deg + 1); e != nil {
		return Polynomial{}, e
	}
	return m.som.mk(false)
}
func (m *MultivariatePolynomialCore) TranslateMany(p Polynomial, n uint32, xs PolynomialVariablesView, vs []unumeral.Int, r *PolynomialRef) error {
	if e := m.check(); e != nil {
		return e
	}
	if r == nil || r.owner != m {
		return ErrPolynomialOwner
	}
	if e := r.Assign(p); e != nil {
		return e
	}
	if n == 0 {
		return nil
	}
	constant, e := m.IsConstant(p)
	if e != nil {
		return e
	}
	if constant {
		return nil
	}
	for i := uint32(0); i < n; i++ {
		x, e := xs.At(i)
		if e != nil {
			return e
		}
		if uint64(i) >= uint64(len(vs)) {
			return arena.ErrBounds
		}
		value, e := m.Translate(r.root.value, x, &vs[i])
		if e != nil {
			return e
		}
		if e = r.Assign(value); e != nil {
			return e
		}
	}
	return nil
}

// The concrete vector overload carries actual borrowed variable bytes and the
// source sizes. Its DEBUG assertion precedes delegation to the raw array body.
func (m *MultivariatePolynomialCore) TranslateVariables(p Polynomial, xs *PolynomialVariableDegrees, vs []unumeral.Int, r *PolynomialRef) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := xs.open(); e != nil {
		return e
	}
	n, e := xs.size()
	if e != nil {
		return e
	}
	if e = polynomialTranslateSourceAssert(999, "xs.size() == vs.size()", func() (bool, error) {
		return uint64(n) == uint64(len(vs)), nil
	}); e != nil {
		return e
	}
	return m.TranslateMany(p, n, variableView(xs), vs, r)
}

// The public source manager forwards to its actual imp, retaining the full
// owner lifetime guard before exposing the core operations.
func (m *MultivariatePolynomialManager) Translate(p Polynomial, x uint32, v *unumeral.Int) (Polynomial, error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	return m.core.Translate(p, x, v)
}
func (m *MultivariatePolynomialManager) TranslateMany(p Polynomial, n uint32, xs PolynomialVariablesView, vs []unumeral.Int, r *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.TranslateMany(p, n, xs, vs, r)
}
func (m *MultivariatePolynomialManager) TranslateVariables(p Polynomial, xs *PolynomialVariableDegrees, vs []unumeral.Int, r *PolynomialRef) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.TranslateVariables(p, xs, vs, r)
}
