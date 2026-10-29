package nlsat

// Source: src/nlsat/nlsat_explain.cpp:237-242,625-658,692-707.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type explanationPSCScope struct {
	s                 *polynomial.PolynomialRef
	active, executing bool
	sourceCause       error
}
type explanationResultantScope struct {
	refs              [2]*polynomial.PolynomialRef // actual p, then q
	active, executing bool
	cursor            int
	sourceCause       error
}

func (n *explanationAssumptions) finishProjectionPSC(owning, abort bool) (err error) {
	s := &n.psc
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
	if err = n.finishProjectionLeading(owning, false); err != nil {
		return err
	}
	if s.s != nil {
		if owning {
			err = s.s.Close()
		} else {
			_, err = s.s.SourceFinalize()
		}
		if err != nil {
			return err
		}
		if owning {
			s.s = nil
		}
	}
	if abort {
		if err = n.finishProjectionLeading(true, true); err != nil {
			return err
		}
		*s = explanationPSCScope{}
	}
	return nil
}
func (n *explanationAssumptions) assertPSCVariables(p, q polynomial.Polynomial, x uint32) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	c := n.search.store.atoms.ineq.core
	a, err := c.MaxVariable(p)
	if err != nil {
		return err
	}
	b, err := c.MaxVariable(q)
	if err != nil {
		return err
	}
	if err = atomAssert("nlsat_explain.cpp", 239, "max_var(p) == max_var(q)", a == b); err != nil {
		return err
	}
	a, err = c.MaxVariable(p)
	if err != nil {
		return err
	}
	return atomAssert("nlsat_explain.cpp", 240, "max_var(p) == x", a == x)
}
func (n *explanationAssumptions) projectPSC(p, q *polynomial.PolynomialRef, x uint32) (err error) {
	s := &n.psc
	if s.active {
		return ErrSearchPending
	}
	*s = explanationPSCScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishProjectionPSC(false, false))
		if err == nil {
			err = n.finishProjectionPSC(true, true)
		}
	}()
	c := n.search.store.atoms.ineq.core
	s.s, err = polynomial.NewPolynomialRef(c)
	if err != nil {
		return err
	}
	a, err := p.Get()
	if err != nil {
		return err
	}
	b, err := q.Get()
	if err != nil {
		return err
	}
	if err = n.assertPSCVariables(a, b, x); err != nil {
		return err
	}
	if err = n.search.store.atoms.ineq.polynomials.PSCChain(a, b, x, n.pscTmp); err != nil {
		return err
	}
	size, err := n.pscTmp.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		value, e := n.pscTmp.Get(i)
		if e != nil {
			return e
		}
		if err = s.s.Assign(value); err != nil {
			return err
		}
		zero, e := c.IsZero(value)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		constant, e := c.IsConstant(value)
		if e != nil {
			return e
		}
		if constant {
			return nil
		}
		sign, e := n.sign(value)
		if e != nil {
			return e
		}
		if sign == 0 {
			if err = n.addZeroAssumption(value); err != nil {
				return err
			}
			continue
		}
		return n.addProjectionFactors(s.s)
	}
	return nil
}
func (n *explanationAssumptions) finishProjectionResultant(owning, abort bool) (err error) {
	s := &n.resultant
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
	if err = n.finishProjectionPSC(owning, false); err != nil {
		return err
	}
	if s.active {
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
		if err = n.finishProjectionPSC(true, true); err != nil {
			return err
		}
		*s = explanationResultantScope{}
	}
	return nil
}
func (n *explanationAssumptions) projectResultants(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	s := &n.resultant
	if s.active {
		return ErrSearchPending
	}
	*s = explanationResultantScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishProjectionResultant(false, false))
		if err == nil {
			err = n.finishProjectionResultant(true, true)
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
	// Retained source callers pass a nonempty stage. Avoid native unsigned
	// underflow for a rejected/empty caller without synthesizing a polynomial.
	for i := uint32(0); i+1 < size; i++ {
		p, e := ps.Get(i)
		if e != nil {
			return e
		}
		if err = s.refs[0].Assign(p); err != nil {
			return err
		}
		for j := i + 1; j < size; j++ {
			q, e := ps.Get(j)
			if e != nil {
				return e
			}
			if err = s.refs[1].Assign(q); err != nil {
				return err
			}
			if err = n.projectPSC(s.refs[0], s.refs[1], x); err != nil {
				return err
			}
		}
	}
	return nil
}
func (n *explanationAssumptions) collectProjectionPSC(p, q *polynomial.PolynomialRef, x uint32) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.projectPSC(p, q, x)
}
func (n *explanationAssumptions) collectProjectionResultants(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.projectResultants(ps, x)
}
