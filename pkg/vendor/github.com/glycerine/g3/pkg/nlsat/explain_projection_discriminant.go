package nlsat

// Source: src/nlsat/nlsat_explain.cpp:670-683.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type explanationDiscriminantScope struct {
	refs              [2]*polynomial.PolynomialRef // actual p, then p_prime
	derivative        polynomial.PolynomialDerivativeCall
	active, executing bool
	cursor            int
	sourceCause       error
}

func (n *explanationAssumptions) finishProjectionDiscriminant(owning, abort bool) (err error) {
	s := &n.discriminant
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
	if owning {
		err = s.derivative.FinishRetainedLocals()
	} else {
		err = s.derivative.FinishSource()
	}
	if err != nil {
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
		if err = s.derivative.CleanupRetained(); err != nil {
			return err
		}
		*s = explanationDiscriminantScope{}
	}
	return nil
}
func (n *explanationAssumptions) projectDiscriminants(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	s := &n.discriminant
	if s.active {
		return ErrSearchPending
	}
	*s = explanationDiscriminantScope{active: true, executing: true}
	defer func() {
		s.executing = false
		err = errors.Join(err, n.finishProjectionDiscriminant(false, false))
		if err == nil {
			err = n.finishProjectionDiscriminant(true, true)
		}
	}()
	f := n.search.store.atoms.ineq
	for i := range s.refs {
		s.refs[i], err = polynomial.NewPolynomialRef(f.core)
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
		degree, e := f.core.Degree(p, x)
		if e != nil {
			return e
		}
		if degree < 2 {
			continue
		}
		if err = f.pm.DerivativeInto(p, x, s.refs[1], &s.derivative); err != nil {
			return err
		}
		if err = n.projectPSC(s.refs[0], s.refs[1], x); err != nil {
			return err
		}
	}
	return nil
}
func (n *explanationAssumptions) collectProjectionDiscriminants(ps *polynomial.PolynomialRefVector, x uint32) (err error) {
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	defer func() { err = n.finish(err) }()
	return n.projectDiscriminants(ps, x)
}
