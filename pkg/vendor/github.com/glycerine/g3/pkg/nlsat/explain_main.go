package nlsat

// Source: src/nlsat/nlsat_explain.cpp:247-261,503-540,1339-1351.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (n *explanationAssumptions) collectPolynomials(literals []sat.Literal, ps *polynomial.PolynomialRefVector) error {
	if ps == nil {
		return arena.ErrConfig
	}
	if err := ps.Reset(); err != nil {
		return err
	}
	for _, literal := range literals {
		a, err := n.search.store.literalAtom(literal)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_explain.cpp", 251, "a != 0", a != nil); err != nil {
			return err
		}
		if a == nil {
			return arena.ErrConfig
		}
		switch a := a.(type) {
		case *InequalityAtom:
			for i := uint32(0); i < a.Size(); i++ {
				p, err := a.Polynomial(i)
				if err != nil {
					return err
				}
				if err = ps.Push(p); err != nil {
					return err
				}
			}
		case *RootAtom:
			if err = ps.Push(a.Polynomial()); err != nil {
				return err
			}
		default:
			return arena.ErrConfig
		}
	}
	return nil
}

func (n *explanationAssumptions) maxPolynomialVariable(ps *polynomial.PolynomialRefVector) (uint32, error) {
	count, err := ps.Size()
	if err != nil || count == 0 {
		return NullVariable, err
	}
	core := n.search.store.atoms.ineq.core
	p, err := ps.Get(0)
	if err != nil {
		return NullVariable, err
	}
	max, err := core.MaxVariable(p)
	if err != nil {
		return NullVariable, err
	}
	if err = atomAssert("nlsat_explain.cpp", 511, "max != null_var", max != NullVariable); err != nil {
		return NullVariable, err
	}
	if max == NullVariable {
		return NullVariable, arena.ErrConfig
	}
	for i := uint32(1); i < count; i++ {
		p, err = ps.Get(i)
		if err != nil {
			return NullVariable, err
		}
		current, err := core.MaxVariable(p)
		if err != nil {
			return NullVariable, err
		}
		if err = atomAssert("nlsat_explain.cpp", 515, "curr != null_var", current != NullVariable); err != nil {
			return NullVariable, err
		}
		if current == NullVariable {
			return NullVariable, arena.ErrConfig
		}
		if current > max {
			max = current
		}
	}
	return max, nil
}

func (n *explanationAssumptions) explainMain(literals []sat.Literal) (err error) {
	if len(literals) == 0 {
		return nil
	}
	ps, err := polynomial.NewPolynomialRefVector(n.search.store.atoms.ineq.core)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ps.Close()) }()
	if err = n.collectPolynomials(literals, ps); err != nil {
		return err
	}
	max, err := n.maxPolynomialVariable(ps)
	if err != nil {
		return err
	}
	if err = n.elimVanishingVector(ps); err != nil {
		return err
	}
	return n.project(ps, max)
}

func (n *explanationAssumptions) collectExplanation(literals []sat.Literal, fullDimensional bool) (err error) {
	if len(literals) == 0 {
		return nil
	}
	if err = n.projectionReady(); err != nil {
		return err
	}
	if n.search.minimizer != nil && len(literals) > 1 {
		minimized, minimizeErr := n.search.minimizer.minimize(literals)
		if minimizeErr != nil {
			return minimizeErr
		}
		defer func() { err = errors.Join(err, n.search.minimizer.resetResult()) }()
		literals = minimized
	}
	if err = n.start(); err != nil {
		return err
	}
	n.fullDimensional = fullDimensional
	defer func() { err = n.finish(err) }()
	if n.search.config.SimplifyCores {
		for _, literal := range literals {
			if err = n.simplifiedCore.push(literal); err != nil {
				return err
			}
		}
		defer func() { err = errors.Join(err, n.simplifiedCore.reset()) }()
		max, maxErr := n.search.store.maxVar(n.simplifiedCore.result)
		if maxErr != nil {
			return maxErr
		}
		if err = atomAssert("nlsat_explain.cpp", 1357, "max != null_var", max != NullVariable); err != nil {
			return err
		}
		if err = n.normalizeCore(&n.simplifiedCore, max); err != nil {
			return err
		}
		if err = n.simplifyCoreEqualities(&n.simplifiedCore, max); err != nil {
			return err
		}
		return n.explainMain(n.simplifiedCore.result)
	}
	return n.explainMain(literals)
}
