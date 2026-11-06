package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, merge_models.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

// mergeNLModels translates SMT function graphs from their current real values
// to nonlinear values, then copies nonlinear constant interpretations.
func mergeNLModels(m *ast.ASTManager, nonlinear, smt *model.ModelCore) (resultErr error) {
	if m == nil || nonlinear == nil || smt == nil || nonlinear.Manager() != m || smt.Manager() != m {
		return ast.ErrASTDomain
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return err
	}
	replace, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	count, err := nonlinear.NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		decl, e := nonlinear.Constant(i)
		if e != nil {
			return e
		}
		rangeSort, e := m.GetRange(decl)
		if e != nil {
			return e
		}
		if rangeSort != real {
			continue
		}
		nlValue, e := nonlinear.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		smtValue, e := smt.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		if !smtValue.IsNull() && nlValue != smtValue {
			if e = replace.Insert(smtValue, nlValue); e != nil {
				return e
			}
		}
	}
	functionCount := smt.NumFunctionDeclarations()
	for i := uint32(0); i < functionCount; i++ {
		decl, e := smt.Function(i)
		if e != nil {
			return e
		}
		arity, e := m.GetArity(decl)
		if e != nil {
			return e
		}
		resultSort, e := m.GetRange(decl)
		if e != nil {
			return e
		}
		hasReal := resultSort == real
		for j := uint32(0); j < arity && !hasReal; j++ {
			sort, e := m.GetDomain(decl, j)
			if e != nil {
				return e
			}
			hasReal = sort == real
		}
		if !hasReal {
			continue
		}
		old, e := smt.FunctionInterpretation(decl)
		if e != nil {
			return e
		}
		if old == nil {
			return ast.ErrASTHandle
		}
		fresh, e := model.NewFunctionInterpretation(m, arity)
		if e != nil {
			return e
		}
		e = translateNLFunction(m, replace, old, fresh)
		if e == nil {
			e = smt.RegisterFunction(decl, fresh)
		}
		if e != nil {
			return errors.Join(e, fresh.Close())
		}
	}
	return smt.CopyConstantInterpretations(nonlinear)
}

func translateNLFunction(m *ast.ASTManager, replace *rewriter.ExprSafeReplace, old, fresh *model.FunctionInterpretation) (resultErr error) {
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	translate := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if h.IsNull() {
			return h, nil
		}
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		if err = replace.Apply(h, ref); err != nil {
			return ast.ASTHandle{}, err
		}
		return m.RefGet(ref)
	}
	for i := uint32(0); i < old.NumEntries(); i++ {
		entry, err := old.Entry(i)
		if err != nil {
			return err
		}
		args := make([]ast.ASTHandle, old.Arity())
		for j := range args {
			arg, e := entry.Argument(uint32(j))
			if e != nil {
				return e
			}
			args[j], e = translate(arg)
			if e != nil {
				return e
			}
		}
		value, err := entry.Result()
		if err != nil {
			return err
		}
		value, err = translate(value)
		if err != nil {
			return err
		}
		if err = fresh.InsertEntry(args, value); err != nil {
			return err
		}
	}
	fallback, err := old.Else()
	if err != nil {
		return err
	}
	fallback, err = translate(fallback)
	if err != nil {
		return err
	}
	return fresh.SetElse(fallback)
}
