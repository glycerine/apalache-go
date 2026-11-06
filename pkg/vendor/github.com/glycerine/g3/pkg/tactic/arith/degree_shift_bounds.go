package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, even-degree constraints.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func degreeShiftAssertBounds(plan *degreeShiftPlan, goal *tactic.Goal, proofs bool) (err error) {
	if plan == nil || plan.manager == nil || goal == nil || goal.Manager() != plan.manager {
		return ast.ErrASTHandle
	}
	m := plan.manager
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	zero, err := m.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	zeroRef, err := m.NewRef(zero)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&zeroRef)) }()
	for _, entry := range plan.entries {
		if entry.degree.Bit(0) != 0 {
			continue
		}
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return e
		}
		bound, e := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{fresh, zero})
		if e != nil {
			return e
		}
		boundRef, e := m.NewRef(bound)
		if e != nil {
			return e
		}
		if proofs {
			premise, x := m.RefGet(entry.proof)
			if x == nil {
				var proof ast.ASTHandle
				proof, x = m.MkProofTheoryLemma(family, bound, []ast.ASTHandle{premise}, nil)
				if x == nil {
					x = goal.AssertWithProof(bound, proof, ast.ASTDependency{})
				}
			}
			e = x
		} else {
			e = goal.Assert(bound)
		}
		e = errors.Join(e, m.DelRef(&boundRef))
		if e != nil {
			return e
		}
	}
	return nil
}
