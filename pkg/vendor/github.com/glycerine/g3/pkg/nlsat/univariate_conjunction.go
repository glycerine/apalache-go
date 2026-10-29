package nlsat

// Univariate conjunction specialization of the retained evaluator/domain/witness
// path in nlsat_solver.cpp. This is not the multivariate search constructor.

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type UnivariateConjunction struct {
	am              *polynomial.AlgebraicManager
	pm              *polynomial.MultivariatePolynomialManager
	assignment      *Assignment
	allocator       *umemory.SmallAllocator
	evaluator       *Evaluator
	domain, current evaluatorIntervalRef
	union           intervalUnionReceipt
	variable        uint32
	nextReason      uint32
	closed          bool
}

// Managers are borrowed. On a numerical failure their owner acknowledges the
// numerical call before Close resumes the owned evaluator/interval cleanup.
func NewUnivariateConjunction(am *polynomial.AlgebraicManager, pm *polynomial.MultivariatePolynomialManager, variable uint32) (u *UnivariateConjunction, err error) {
	u = &UnivariateConjunction{am: am, pm: pm, variable: variable, nextReason: 1}
	if am == nil || pm == nil {
		return u, ErrSearchOwner
	}
	q, err := am.RationalManager()
	if err != nil {
		return u, err
	}
	u.assignment, err = NewAssignment(am)
	if err != nil {
		return u, err
	}
	u.allocator, err = umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: q.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: diagnostic.DebugBuild}})
	if err != nil {
		return u, err
	}
	u.evaluator, err = NewEvaluator(u.assignment, pm, u.allocator)
	if err != nil {
		return u, err
	}
	u.domain.manager = u.evaluator.ism
	u.current.manager = u.evaluator.ism
	return u, nil
}
func (u *UnivariateConjunction) Add(p polynomial.Polynomial, kind AtomKind, negative bool) error {
	if u == nil || u.closed || u.evaluator == nil || (kind != AtomEQ && kind != AtomLT && kind != AtomGT) {
		return ErrSearchOwner
	}
	c, err := u.pm.ArithmeticCore()
	if err != nil {
		return err
	}
	x, err := c.MaxVariable(p)
	if err != nil {
		return err
	}
	if x == NullVariable {
		sign, err := u.am.EvalSign(u.pm, p, u.assignment)
		if err != nil {
			return err
		}
		if satisfied(sign, kind, negative) {
			return nil
		}
		full, err := u.evaluator.ism.Mk(true, true, nil, true, true, nil, sat.MakeLiteral(sat.BoolVar(u.nextReason), negative))
		if err != nil {
			return err
		}
		return u.domain.assign(full)
	}
	univariate, err := c.IsUnivariate(p)
	if err != nil {
		return err
	}
	if !univariate || x != u.variable {
		return ErrSearchOwner
	}
	atom := InequalityAtom{}
	if err = atom.initialize(kind, []polynomial.Polynomial{p}, []bool{false}, x, make([]InequalityFactor, 1)); err != nil {
		return err
	}
	atom.boolVar = sat.BoolVar(u.nextReason)
	u.nextReason++
	u.current.value, err = u.evaluator.InfeasibleIntervals(&atom, negative)
	if err != nil {
		return err
	}
	combined, err := u.evaluator.ism.mkUnionWithReceipt(u.domain.value, u.current.value, &u.union)
	if err != nil {
		return err
	}
	if err = u.domain.assign(combined); err != nil {
		return err
	}
	return u.current.reset()
}
func (u *UnivariateConjunction) Check(witness *polynomial.Algebraic) (bool, error) {
	if u == nil || u.closed || u.evaluator == nil || witness == nil {
		return false, ErrSearchOwner
	}
	full, err := u.evaluator.ism.IsFull(u.domain.value)
	if err != nil || full {
		return false, err
	}
	return true, u.evaluator.ism.PeekInComplement(u.domain.value, false, witness, false)
}
func (u *UnivariateConjunction) Close() error {
	if u == nil || u.closed {
		return nil
	}
	if u.evaluator != nil {
		if err := u.evaluator.Cleanup(); err != nil {
			return err
		}
		if err := u.current.reset(); err != nil {
			return err
		}
		if err := u.domain.reset(); err != nil {
			return err
		}
		if err := u.evaluator.ism.cleanupUnion(&u.union); err != nil {
			return err
		}
		if err := u.evaluator.ism.Cleanup(); err != nil {
			return err
		}
		if err := u.evaluator.Close(); err != nil {
			return err
		}
		u.evaluator = nil
	}
	if u.assignment != nil {
		if err := u.assignment.Close(); err != nil {
			return err
		}
		u.assignment = nil
	}
	if u.allocator != nil {
		if err := u.allocator.Close(); err != nil {
			return err
		}
		u.allocator = nil
	}
	u.closed = true
	return nil
}
