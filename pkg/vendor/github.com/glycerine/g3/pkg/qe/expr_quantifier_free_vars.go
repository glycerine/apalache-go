package qe

// Source: src/qe/qe.cpp, expr_quant_elim::instantiate_expr/abstract_expr.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type exprQuantifierInstantiation struct {
	manager *ast.ASTManager
	formula ast.ASTRef
	bound   []ast.ASTRef
}

func instantiateExprFreeVars(m *ast.ASTManager, formula ast.ASTHandle) (result *exprQuantifierInstantiation, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	result = &exprQuantifierInstantiation{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	result.formula, err = m.NewRef(formula)
	if err != nil {
		return result, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, err
	}
	if err = free.Collect(formula); err != nil {
		return result, err
	}
	if free.Empty() {
		return result, nil
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	if err = free.SetDefaultSort(boolSort); err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, 0, free.Size())
	for i := free.Size(); i > 0; i-- {
		sort, e := free.Get(i - 1)
		if e != nil {
			return result, e
		}
		fresh, e := m.MkFreshConst("bound", sort)
		if e != nil {
			return result, e
		}
		ref, e := m.NewRef(fresh)
		if e != nil {
			return result, e
		}
		result.bound = append(result.bound, ref)
		args = append(args, fresh)
	}
	sub, err := rewriter.NewVarSubstitution(m, true)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return result, err
	}
	defer func() { err = errors.Join(err, sub.Close()) }()
	if err = sub.Substitute(formula, args, result.formula); err != nil {
		return result, err
	}
	return result, nil
}

// abstract restores free de Bruijn variables in an arbitrary transformed root.
func (s *exprQuantifierInstantiation) abstract(root ast.ASTHandle) (out ast.ASTRef, err error) {
	if s == nil || s.manager == nil {
		return out, ast.ErrASTHandle
	}
	m := s.manager
	out, err = m.NewRef(root)
	if err != nil {
		return out, err
	}
	defer func() {
		if err != nil && out != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&out))
			out = ast.ASTRef{}
		}
	}()
	if len(s.bound) == 0 {
		return out, nil
	}
	args := make([]ast.ASTHandle, len(s.bound))
	for i, ref := range s.bound {
		args[i], err = m.RefGet(ref)
		if err != nil {
			return out, err
		}
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return out, err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	if err = abstractor.Abstract(0, args, root, out); err != nil {
		return out, err
	}
	return out, nil
}

func (s *exprQuantifierInstantiation) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	if s.formula != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.formula); err != nil {
			return err
		}
	}
	for len(s.bound) > 0 {
		i := len(s.bound) - 1
		if err := s.manager.DelRef(&s.bound[i]); err != nil {
			return err
		}
		s.bound = s.bound[:i]
	}
	s.manager = nil
	return nil
}
