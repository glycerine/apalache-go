package qe

// Source: src/qe/qe.cpp and qe_array_plugin.cpp, equation elimination.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// eliminateQuantifierArrayEquations repeatedly applies the array plugin to
// conjunctive equations and eliminates auxiliary arrays that become unused.
func eliminateQuantifierArrayEquations(m *ast.ASTManager, quantifier ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		return result, false, err
	}
	free, err := rewriter.NewExprFreeVars(m)
	if err != nil {
		return result, false, err
	}
	if err = free.Collect(quantifier); err != nil || !free.Empty() {
		return result, false, err
	}
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return result, false, err
	}
	family, err := m.RegisterArrayPlugin()
	if err != nil {
		return result, false, err
	}
	count, err := m.GetNumDeclarations(quantifier)
	if err != nil {
		return result, false, err
	}
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDeclarationSort(quantifier, i)
		if e != nil {
			return result, false, e
		}
		id, e := m.GetFamilyID(sort)
		if e != nil || id != family {
			return result, false, e
		}
	}
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, false, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	variables := make([]ast.ASTHandle, len(extracted.variables))
	for i, ref := range extracted.variables {
		variables[i], err = m.RefGet(ref)
		if err != nil {
			return result, false, err
		}
	}
	body, err = m.RefGet(extracted.body)
	if err != nil {
		return result, false, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	context, err := NewSolverContext(m, variables)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	plugin, err := NewArrayPlugin(context)
	if err != nil {
		return result, false, err
	}
	if err = context.AddPlugin(plugin); err != nil {
		return result, false, errors.Join(err, plugin.Close())
	}
	params, err := config.Context.NewValue()
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, config.Context.Del(&params)) }()
	key := "expand_select_store"
	if err = config.Context.SetBoolName(&params, &key, true); err != nil {
		return result, false, err
	}
	key = "expand_store_eq"
	if err = config.Context.SetBoolName(&params, &key, true); err != nil {
		return result, false, err
	}
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, &params)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return result, false, err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	for len(context.variables) > 0 {
		canceled, e := m.Canceled()
		if e != nil {
			return result, false, e
		}
		if canceled {
			return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		if e = config.Cooperation.Cooperate("qe"); e != nil {
			return result, false, e
		}
		body, e = m.RefGet(formula)
		if e != nil {
			return result, false, e
		}
		if e = rw.Rewrite(body, formula); e != nil {
			return result, false, e
		}
		body, e = m.RefGet(formula)
		if e != nil {
			return result, false, e
		}
		removed := false
		for i, variable := range context.variables {
			contains, cause := ast.NewContainsApp(m, variable)
			if cause != nil {
				return result, false, cause
			}
			found, cause := contains.Check(body)
			cause = errors.Join(cause, contains.Close())
			if cause != nil {
				return result, false, cause
			}
			if !found {
				if cause = context.EliminateVariable(uint32(i), body, ast.ASTHandle{}); cause != nil {
					return result, false, cause
				}
				removed = true
				break
			}
		}
		if removed {
			continue
		}
		conjs, e := NewConjunctionEnum(m, body)
		if e != nil {
			return result, false, e
		}
		solved, e := plugin.Solve(conjs, body)
		e = errors.Join(e, conjs.Close())
		if e != nil {
			return result, false, e
		}
		if !solved {
			changed, cause := projectArrayAuxiliary(m, context, formula, config)
			if cause != nil {
				return result, false, cause
			}
			if changed {
				continue
			}
			projected, done, cause := projectArrayConditional(m, body, context, config)
			if cause == nil && !done {
				projected, done, cause = projectArrayDisjunction(m, body, context, config)
			}
			if cause != nil || !done {
				return result, false, cause
			}
			value, cause := m.RefGet(projected)
			if cause == nil {
				cause = m.SetRef(formula, value)
			}
			cause = errors.Join(cause, m.DelRef(&projected))
			if cause != nil {
				return result, false, cause
			}
			break
		}
		next, found, e := context.CurrentFormula()
		if e != nil {
			return result, false, e
		}
		if !found {
			return result, false, ast.ErrASTDomain
		}
		if e = m.SetRef(formula, next); e != nil {
			return result, false, e
		}
	}
	body, err = m.RefGet(formula)
	if err != nil {
		return result, false, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err != nil {
			return result, false, err
		}
	}
	result, err = m.NewRef(body)
	if err != nil {
		return result, false, err
	}
	if err = rw.Rewrite(body, result); err != nil {
		return result, false, err
	}
	return result, true, nil
}
