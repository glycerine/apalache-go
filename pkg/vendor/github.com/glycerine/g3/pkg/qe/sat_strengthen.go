package qe

// Source: src/qe/qe_sat_tactic.cpp, NNF strengthening extrapolation.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func satExclusiveAtoms(m *ast.ASTManager, set, other map[ast.ASTHandle]ast.ASTRef) ([]ast.ASTHandle, error) {
	type item struct {
		h  ast.ASTHandle
		id uint32
	}
	items := make([]item, 0, len(set))
	for h := range set {
		if _, both := other[h]; both {
			continue
		}
		id, err := m.GetASTID(h)
		if err != nil {
			return nil, err
		}
		items = append(items, item{h, id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })
	out := make([]ast.ASTHandle, len(items))
	for i := range items {
		out[i] = items[i].h
	}
	return out, nil
}

// satStrengthen expects B to be asserted in solver, as in the source tactic.
// The returned expression is owned; all temporary solver scopes are popped.
func satStrengthen(m *ast.ASTManager, normalizer *QENNFNormalizer, solver *smtcore.SMTSolver, rw *rewriter.TheoryRewriter, a, b ast.ASTHandle, checkpoint func() error) (result ast.ASTRef, err error) {
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
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	positive, negative := atoms.positiveAtoms(), atoms.negativeAtoms()
	pos, err := satExclusiveAtoms(m, positive, negative)
	if err != nil {
		return result, err
	}
	neg, err := satExclusiveAtoms(m, negative, positive)
	if err != nil {
		return result, err
	}
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
	for polarity, set := range [][]ast.ASTHandle{pos, neg} {
		var value ast.ASTHandle
		if polarity == 0 {
			value, err = m.MkFalse()
		} else {
			value, err = m.MkTrue()
		}
		if err != nil {
			return result, err
		}
		for _, atom := range set {
			equal, e := m.MkIff(atom, value)
			if e != nil {
				return result, e
			}
			ref, e := m.NewRef(equal)
			if e != nil {
				return result, e
			}
			if e = solver.Push(); e != nil {
				return result, errors.Join(e, m.DelRef(&ref))
			}
			if e = solver.AssertFormula(equal); e == nil {
				var status lbool.LBool
				status, e = solver.CheckSat(nil)
				if e == nil {
					e = solver.Pop(1)
					if e == nil && status == lbool.True {
						if e = m.ExprSubstitutionInsert(sub, atom, value, ast.ASTHandle{}, ast.ASTDependency{}); e == nil {
							e = solver.AssertFormula(equal)
						}
					}
				} else {
					e = errors.Join(e, solver.Pop(1))
				}
			} else {
				e = errors.Join(e, solver.Pop(1))
			}
			e = errors.Join(e, m.DelRef(&ref))
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
	if err = solver.Pop(1); err != nil {
		return result, err
	}
	scoped = false
	replace, err := rewriter.NewDefaultExprReplacer(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	if err = replace.SetSubstitution(&sub); err != nil {
		return result, err
	}
	old, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = replace.Rewrite(old, result); err != nil {
		return result, err
	}
	old, err = m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = rw.Rewrite(old, result); err != nil {
		return result, err
	}
	return result, nil
}
