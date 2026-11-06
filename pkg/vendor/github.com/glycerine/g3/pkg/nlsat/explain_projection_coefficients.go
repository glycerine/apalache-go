package nlsat

// Source: src/nlsat/nlsat_explain.cpp:570-621.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type explanationProjectionFactorScope struct {
	f                 *polynomial.PolynomialRef
	active, executing bool
	sourceCause       error
}
type explanationLeadingScope struct {
	refs              [2]*polynomial.PolynomialRef // source p, then lc
	coefficient       polynomial.PolynomialCoefficientCall
	predicate         polynomial.PolynomialNonzeroConstantCall
	active, executing bool
	cursor            int
	sourceCause       error
}

// Bind the actual existing worklist and factor parameter; no full explanation
// constructor or fake solver is implied by composing these retained subobjects.
func (n *explanationAssumptions) bindProjection(todo *projectionWorklist, factor bool) error {
	if err := n.owner(); err != nil {
		return err
	}
	if err := n.search.open(); err != nil {
		return err
	}
	if todo == nil || todo.search != n.search {
		return ErrSearchOwner
	}
	if err := todo.open(); err != nil {
		return err
	}
	if n.projectionTodo != nil && n.projectionTodo != todo {
		return ErrSearchOwner
	}
	n.projectionTodo, n.factorProjection = todo, factor
	if n.pscTmp == nil {
		var err error
		n.pscTmp, err = polynomial.NewPolynomialRefVector(n.search.store.atoms.ineq.core)
		n.failed = err != nil
		return err
	}
	return nil
}
func (n *explanationAssumptions) projectionReady() error {
	if err := n.owner(); err != nil {
		return err
	}
	if n.projectionTodo == nil {
		return ErrSearchOwner
	}
	return n.projectionTodo.open()
}
func (n *explanationAssumptions) finishProjectionFactors(owning, abort bool) (err error) {
	s := &n.projectionFactors
	if s.executing {
		return ErrSearchPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	defer func() {
		if err != nil && s.sourceCause == nil {
			s.sourceCause = err
		}
	}()
	if err = n.finishVanishingPhase(owning, false); err != nil {
		return err
	}
	if s.f != nil {
		if owning {
			err = s.f.Close()
		} else {
			_, err = s.f.SourceFinalize()
		}
		if err != nil {
			return err
		}
		if owning {
			s.f = nil
		}
	}
	if abort {
		if err = n.finishVanishingPhase(true, true); err != nil {
			return err
		}
		*s = explanationProjectionFactorScope{}
	}
	return nil
}
func (n *explanationAssumptions) addProjectionFactors(p *polynomial.PolynomialRef) (err error) {
	s := &n.projectionFactors
	if s.active {
		return ErrSearchPending
	}
	*s = explanationProjectionFactorScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishProjectionFactors(false, false))
		if err == nil {
			err = n.finishProjectionFactors(true, true)
		}
	}()
	c := n.search.store.atoms.ineq.core
	value, err := p.Get()
	if err != nil {
		return err
	}
	constant, err := c.IsConstant(value)
	if err != nil || constant {
		return err
	}
	if err = n.elimVanishing(p); err != nil {
		return err
	}
	value, err = p.Get()
	if err != nil {
		return err
	}
	constant, err = c.IsConstant(value)
	if err != nil || constant {
		return err
	}
	if !n.factorProjection {
		return n.projectionTodo.insertFromExplanation(n, value)
	}
	if err = n.factor(value); err != nil {
		return err
	}
	s.f, err = polynomial.NewPolynomialRef(c)
	if err != nil {
		return err
	}
	for i := uint32(0); ; i++ {
		// The source rereads the shared vector extent on every iteration.
		size, e := n.factors.Size()
		if e != nil {
			return e
		}
		if i >= size {
			break
		}
		factor, e := n.factors.Get(i)
		if e != nil {
			return e
		}
		if err = s.f.Assign(factor); err != nil {
			return err
		}
		if err = n.elimVanishing(s.f); err != nil {
			return err
		}
		factor, err = s.f.Get()
		if err != nil {
			return err
		}
		constant, e := c.IsConstant(factor)
		if e != nil {
			return e
		}
		if !constant {
			if err = n.projectionTodo.insertFromExplanation(n, factor); err != nil {
				return err
			}
		}
	}
	return nil
}
func (n *explanationAssumptions) finishProjectionLeading(owning, abort bool) (err error) {
	s := &n.leading
	if s.executing {
		return ErrSearchPending
	}
	if !owning && s.sourceCause != nil {
		return s.sourceCause
	}
	defer func() {
		if err != nil && s.sourceCause == nil {
			s.sourceCause = err
		}
	}()
	if err = n.finishProjectionFactors(owning, false); err != nil {
		return err
	}
	if s.active {
		if err = s.coefficient.FinishSource(); err != nil {
			return err
		}
		for s.cursor < 2 {
			i := 1 - s.cursor
			if s.refs[i] != nil {
				if owning {
					err = s.refs[i].Close()
				} else {
					_, err = s.refs[i].SourceFinalize()
				}
				if err != nil {
					return err
				}
				if owning {
					s.refs[i] = nil
				}
			}
			s.cursor++
		}
		if owning {
			for i := 1; i >= 0; i-- {
				if s.refs[i] != nil {
					if err = s.refs[i].Close(); err != nil {
						return err
					}
					s.refs[i] = nil
				}
			}
		}
	}
	if abort {
		if err = n.finishProjectionFactors(true, true); err != nil {
			return err
		}
		if err = s.predicate.CleanupRetained(); err != nil {
			return err
		}
		if err = s.coefficient.CleanupRetained(); err != nil {
			return err
		}
		*s = explanationLeadingScope{}
	}
	return nil
}
func (n *explanationAssumptions) assertProjectionLeadingSign(p polynomial.Polynomial) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	sign, err := n.sign(p)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_explain.cpp", 616, "sign(lc) != 0", sign != 0)
}
func (n *explanationAssumptions) addLeadingCoefficients(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	s := &n.leading
	if s.active {
		return ErrSearchPending
	}
	*s = explanationLeadingScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishProjectionLeading(false, false))
		if err == nil {
			err = n.finishProjectionLeading(true, true)
		}
	}()
	c := n.search.store.atoms.ineq.core
	for i := range s.refs {
		s.refs[i], err = polynomial.NewPolynomialRef(c)
		if err != nil {
			return err
		}
	}
	size, err := ps.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		p, e := ps.Get(i)
		if e != nil {
			return e
		}
		if err = s.refs[0].Assign(p); err != nil {
			return err
		}
		k, e := c.Degree(p, x)
		if e != nil {
			return e
		}
		if err = atomAssert("nlsat_explain.cpp", 609, "k > 0", k > 0); err != nil {
			return err
		}
		constant, e := c.NonzeroConstantCoefficientWithCall(p, x, k, &s.predicate)
		if e != nil {
			return e
		}
		if constant {
			continue
		}
		if err = c.CoeffInto(p, x, k, s.refs[1], &s.coefficient); err != nil {
			return err
		}
		lc, e := s.refs[1].Get()
		if e != nil {
			return e
		}
		if err = n.assertProjectionLeadingSign(lc); err != nil {
			return err
		}
		if err = n.vanishingAssert(617, "!is_const(lc)", lc); err != nil {
			return err
		}
		if err = n.addProjectionFactors(s.refs[1]); err != nil {
			return err
		}
	}
	return nil
}
func (n *explanationAssumptions) collectProjectionFactors(p *polynomial.PolynomialRef) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.addProjectionFactors(p)
}
func (n *explanationAssumptions) collectLeadingCoefficients(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.addLeadingCoefficients(ps, x)
}
