package qe

// Source: src/qe/qe_mbp.cpp project_plugin::partition_values/partition_args.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func partitionProjectionValues(m *ast.ASTManager, eval *model.ModelEvaluator, values []ast.ASTHandle, roots *[]ast.ASTRef, lits *[]ast.ASTRef) error {
	groups := make(map[ast.ASTHandle]ast.ASTHandle)
	var representatives []ast.ASTHandle
	for _, h := range values {
		r, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		*roots = append(*roots, r)
		if err = eval.Evaluate(h, r); err != nil {
			return err
		}
		value, err := m.RefGet(r)
		if err != nil {
			return err
		}
		if representative, ok := groups[value]; ok {
			eq, err := m.MkEq(h, representative)
			if err != nil {
				return err
			}
			out, err := m.NewRef(eq)
			if err != nil {
				return err
			}
			*lits = append(*lits, out)
		} else {
			groups[value] = h
			representatives = append(representatives, h)
		}
	}
	if len(representatives) > 1 {
		distinct, err := m.MkDistinctSimplified(representatives)
		if err != nil {
			return err
		}
		r, err := m.NewRef(distinct)
		if err != nil {
			return err
		}
		*lits = append(*lits, r)
	}
	return nil
}
func partitionProjectionArguments(m *ast.ASTManager, eval *model.ModelEvaluator, selects []ast.ASTHandle, roots *[]ast.ASTRef, lits *[]ast.ASTRef) error {
	if len(selects) == 0 {
		return nil
	}
	d, err := m.GetDecl(selects[0])
	if err != nil {
		return err
	}
	n, err := m.GetArity(d)
	if err != nil {
		return err
	}
	for j := uint32(1); j < n; j++ {
		args := make([]ast.ASTHandle, len(selects))
		for i, h := range selects {
			args[i], err = m.GetArg(h, j)
			if err != nil {
				return err
			}
		}
		if err = partitionProjectionValues(m, eval, args, roots, lits); err != nil {
			return err
		}
	}
	return nil
}
