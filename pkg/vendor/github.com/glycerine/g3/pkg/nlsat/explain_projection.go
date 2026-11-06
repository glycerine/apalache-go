package nlsat

// Source: src/nlsat/nlsat_explain.cpp:953-991.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (n *explanationAssumptions) allUnivariate(ps *polynomial.PolynomialRefVector, x uint32) (bool, error) {
	count, err := ps.Size()
	if err != nil {
		return false, err
	}
	core := n.search.store.atoms.ineq.core
	for i := uint32(0); i < count; i++ {
		p, err := ps.Get(i)
		if err != nil {
			return false, err
		}
		max, err := core.MaxVariable(p)
		if err != nil {
			return false, err
		}
		if max != x {
			return false, nil
		}
		univariate, err := core.IsUnivariate(p)
		if err != nil || !univariate {
			return univariate, err
		}
	}
	return true, nil
}

// project applies the source model-based projection loop to the bound
// worklist. ps is both the initial polynomial set and the current variable
// stage, exactly as in explain::imp::project.
func (n *explanationAssumptions) project(ps *polynomial.PolynomialRefVector, maxX uint32) error {
	count, err := ps.Size()
	if err != nil || count == 0 {
		return err
	}
	todo := n.projectionTodo
	if err = todo.resetFromExplanation(n); err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		p, err := ps.Get(i)
		if err != nil {
			return err
		}
		if err = todo.insertFromExplanation(n, p); err != nil {
			return err
		}
	}
	x, err := todo.removeMaxPolynomialsFromExplanation(n, ps)
	if err != nil {
		return err
	}
	if x < maxX {
		if err = n.addCellLiterals(ps, x); err != nil {
			return err
		}
	}
	for {
		univariate, err := n.allUnivariate(ps, x)
		if err != nil {
			return err
		}
		empty, err := todo.emptyFromExplanation(n)
		if err != nil {
			return err
		}
		if univariate && empty {
			return todo.resetFromExplanation(n)
		}
		if err = n.addLeadingCoefficients(ps, x); err != nil {
			return err
		}
		if err = n.projectDiscriminants(ps, x); err != nil {
			return err
		}
		if err = n.projectResultants(ps, x); err != nil {
			return err
		}
		empty, err = todo.emptyFromExplanation(n)
		if err != nil || empty {
			return err
		}
		x, err = todo.removeMaxPolynomialsFromExplanation(n, ps)
		if err != nil {
			return err
		}
		if err = n.addCellLiterals(ps, x); err != nil {
			return err
		}
	}
}

func (n *explanationAssumptions) collectProjection(ps *polynomial.PolynomialRefVector, maxX uint32, fullDimensional bool) (err error) {
	if ps == nil {
		return arena.ErrConfig
	}
	if err = n.projectionReady(); err != nil {
		return err
	}
	if err = n.start(); err != nil {
		return err
	}
	n.fullDimensional = fullDimensional
	defer func() { err = n.finish(err) }()
	return n.project(ps, maxX)
}
