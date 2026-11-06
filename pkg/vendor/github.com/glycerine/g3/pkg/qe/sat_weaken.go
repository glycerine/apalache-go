package qe

// Source: src/qe/qe_sat_tactic.cpp, NNF weakening extrapolation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func satWeaken(m *ast.ASTManager, normalizer *QENNFNormalizer, solver *smtcore.SMTSolver, rw *rewriter.TheoryRewriter, a, b ast.ASTHandle, checkpoint func() error) (result ast.ASTRef, err error) {
	if m == nil || normalizer == nil || solver == nil || rw == nil {
		return result, ast.ErrASTHandle
	}
	result, err = m.NewRef(b)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	atoms, err := normalizer.Normalize(result)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, atoms.Close()) }()
	pos, err := satExclusiveAtoms(m, atoms.positiveAtoms(), atoms.negativeAtoms())
	if err != nil {
		return result, err
	}
	neg, err := satExclusiveAtoms(m, atoms.negativeAtoms(), atoms.positiveAtoms())
	if err != nil {
		return result, err
	}
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	replace, err := rewriter.NewDefaultExprReplacer(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	if err = solver.Push(); err != nil {
		return result, err
	}
	scoped := true
	defer func() {
		if scoped {
			err = errors.Join(err, solver.Pop(1))
		}
	}()
	if err = solver.AssertFormula(a); err != nil {
		return result, err
	}
	notB, err := m.MkNot(b)
	if err != nil {
		return result, err
	}
	if err = solver.AssertFormula(notB); err != nil {
		return result, err
	}
	for polarity, set := range [][]ast.ASTHandle{pos, neg} {
		var value ast.ASTHandle
		if polarity == 0 {
			value, err = m.MkTrue()
		} else {
			value, err = m.MkFalse()
		}
		if err != nil {
			return result, err
		}
		for _, atom := range set {
			current, e := m.RefGet(result)
			if e != nil {
				return result, e
			}
			candidate, e := m.NewRef(current)
			if e != nil {
				return result, e
			}
			e = replace.ApplySubstitution(atom, value, ast.ASTHandle{}, candidate)
			if e == nil {
				e = solver.Push()
				if e == nil {
					var h ast.ASTHandle
					h, e = m.RefGet(candidate)
					if e == nil {
						e = solver.AssertFormula(h)
					}
					var status lbool.LBool
					if e == nil {
						status, e = solver.CheckSat(nil)
					}
					e = errors.Join(e, solver.Pop(1))
					if e == nil && status == lbool.False {
						if e = m.ExprSubstitutionInsert(sub, atom, value, ast.ASTHandle{}, ast.ASTDependency{}); e == nil {
							e = m.AssignRef(result, candidate)
						}
					}
				}
			}
			e = errors.Join(e, m.DelRef(&candidate))
			if e != nil {
				return result, e
			}
			if checkpoint != nil {
				if e = checkpoint(); e != nil {
					return result, e
				}
			}
		}
	}
	if err = replace.SetSubstitution(&sub); err != nil {
		return result, err
	}
	h, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = replace.Rewrite(h, result); err != nil {
		return result, err
	}
	h, err = m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = rw.Rewrite(h, result); err != nil {
		return result, err
	}
	if err = solver.Pop(1); err != nil {
		return result, err
	}
	scoped = false
	return result, nil
}
