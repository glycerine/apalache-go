package qe

// Source: src/qe/qe.cpp, simplify_rewriter_cfg::reduce_quantifier/pre_visit.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// SimplifyQuantifiers performs the source QE simplification used by QE-SAT
// and Datalog rule normalization. The caller owns the returned reference.
func SimplifyQuantifiers(m *ast.ASTManager, expression ast.ASTHandle) (ast.ASTRef, error) {
	return simplifyQEExpression(m, expression)
}

// simplifyQEExpression visits unannotated quantifiers bottom-up and simplifies
// their bodies without requiring every quantified variable to be eliminated.
func simplifyQEExpression(m *ast.ASTManager, expression ast.ASTHandle) (result ast.ASTRef, err error) {
	if m == nil {
		return result, ast.ErrASTHandle
	}
	canceled, err := m.Canceled()
	if err != nil {
		return result, err
	}
	if canceled {
		return result, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(expression)
	if err != nil {
		return result, err
	}
	switch kind {
	case ast.ASTQuantifier:
		patterns, e := m.GetNumPatterns(expression)
		if e != nil {
			return result, e
		}
		noPatterns, e := m.GetNumNoPatterns(expression)
		if e != nil {
			return result, e
		}
		if patterns != 0 || noPatterns != 0 {
			return m.NewRef(expression)
		}
		body, e := m.GetQuantifierBody(expression)
		if e != nil {
			return result, e
		}
		child, e := simplifyQEExpression(m, body)
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&child)) }()
		body, e = m.RefGet(child)
		if e != nil {
			return result, e
		}
		updated, e := m.UpdateQuantifierBody(expression, body)
		if e != nil {
			return result, e
		}
		updatedRef, e := m.NewRef(updated)
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&updatedRef)) }()
		return simplifyQEQuantifier(m, updated)
	case ast.ASTApp:
		n, e := m.GetNumArgs(expression)
		if e != nil {
			return result, e
		}
		args := make([]ast.ASTHandle, n)
		refs := make([]ast.ASTRef, 0, n)
		defer func() {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, m.DelRef(&refs[i]))
			}
		}()
		changed := false
		for i := range args {
			old, x := m.GetArg(expression, uint32(i))
			if x != nil {
				return result, x
			}
			ref, x := simplifyQEExpression(m, old)
			if x != nil {
				return result, x
			}
			refs = append(refs, ref)
			args[i], x = m.RefGet(ref)
			if x != nil {
				return result, x
			}
			changed = changed || args[i] != old
		}
		if changed {
			decl, x := m.GetDecl(expression)
			if x != nil {
				return result, x
			}
			expression, x = m.MkApp(decl, args)
			if x != nil {
				return result, x
			}
		}
		return m.NewRef(expression)
	default:
		return m.NewRef(expression)
	}
}

func simplifyQEQuantifier(m *ast.ASTManager, quantifier ast.ASTHandle) (result ast.ASTRef, err error) {
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	vars := make([]ast.ASTHandle, len(extracted.variables))
	for i, ref := range extracted.variables {
		vars[i], err = m.RefGet(ref)
		if err != nil {
			return result, err
		}
	}
	body, err := m.RefGet(extracted.body)
	if err != nil {
		return result, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, err
		}
	}
	bodyRef, err := m.NewRef(body)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&bodyRef)) }()
	simplified, err := simplifyExists(m, vars, body)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, simplified.Close()) }()
	body, err = m.RefGet(simplified.formula)
	if err != nil {
		return result, err
	}
	if forall {
		var isNot bool
		var arg ast.ASTHandle
		isNot, arg, err = m.IsNot(body)
		if err != nil {
			return result, err
		}
		if isNot {
			body = arg
		} else {
			body, err = m.MkNot(body)
			if err != nil {
				return result, err
			}
		}
	}
	result, err = m.NewRef(body)
	if err != nil {
		return result, err
	}
	if len(simplified.variables) == 0 {
		return result, nil
	}
	shifter, err := rewriter.NewVarShifter(m)
	if shifter != nil {
		defer func() { err = errors.Join(err, shifter.Close()) }()
	}
	if err != nil {
		return result, err
	}
	if err = shifter.ShiftBy(body, uint32(len(simplified.variables)), result); err != nil {
		return result, err
	}
	body, err = m.RefGet(result)
	if err != nil {
		return result, err
	}
	bound := make([]ast.ASTHandle, len(simplified.variables))
	sorts := make([]ast.ASTHandle, len(bound))
	names := make([]symbol.Symbol, len(bound))
	for i, ref := range simplified.variables {
		bound[i], err = m.RefGet(ref)
		if err != nil {
			return result, err
		}
		sorts[i], err = m.GetSort(bound[i])
		if err != nil {
			return result, err
		}
		decl, e := m.GetDecl(bound[i])
		if e != nil {
			return result, e
		}
		names[i], err = m.GetASTName(decl)
		if err != nil {
			return result, err
		}
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	if err = abstractor.Abstract(0, bound, body, result); err != nil {
		return result, err
	}
	body, err = m.RefGet(result)
	if err != nil {
		return result, err
	}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body, Weight: 1})
	if err != nil {
		return result, err
	}
	if err = m.SetRef(result, q); err != nil {
		return result, err
	}
	return result, nil
}
