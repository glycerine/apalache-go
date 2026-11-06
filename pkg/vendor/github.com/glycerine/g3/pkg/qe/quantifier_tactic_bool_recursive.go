package qe

// Source: src/qe/qe.cpp, expr_quant_elim's recursive expression traversal.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// rewriteQuantifierScalar recursively eliminates supported Boolean, arithmetic,
// bit-vector, datatype, array, and finite-domain quantifiers.
// Free de Bruijn variables are temporarily represented by fresh constants while
// eliminating each quantifier, then abstracted back into their original scope.
func rewriteQuantifierScalar(m *ast.ASTManager, expression ast.ASTHandle, config Config) (result ast.ASTRef, complete bool, err error) {
	if m == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	canceled, err := m.Canceled()
	if err != nil {
		return result, false, err
	}
	if canceled {
		return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	if err = config.Cooperation.Cooperate("qe"); err != nil {
		return result, false, err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return result, false, err
	}
	switch kind {
	case ast.ASTQuantifier:
		if config.eliminateVariablesAsBlockSet && !config.eliminateVariablesAsBlock {
			count, e := m.GetNumDeclarations(expression)
			if e != nil {
				return result, false, e
			}
			if count > 1 {
				return rewriteQuantifierVariablesIndividually(m, expression, config)
			}
		}
		body, e := m.GetQuantifierBody(expression)
		if e != nil {
			return result, false, e
		}
		rewritten, done, e := rewriteQuantifierScalar(m, body, config)
		if e != nil || !done {
			return result, false, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&rewritten)) }()
		body, e = m.RefGet(rewritten)
		if e != nil {
			return result, false, e
		}
		updated, e := m.UpdateQuantifierBody(expression, body)
		if e != nil {
			return result, false, e
		}
		updatedRef, e := m.NewRef(updated)
		if e != nil {
			return result, false, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&updatedRef)) }()
		instantiated, e := instantiateExprFreeVars(m, updated)
		if e != nil {
			return result, false, e
		}
		defer func() { err = errors.Join(err, instantiated.Close()) }()
		q, e := m.RefGet(instantiated.formula)
		if e != nil {
			return result, false, e
		}
		eliminated, done, e := eliminateQuantifierBoolUnits(m, q, config)
		if e == nil && !done && !config.nonlinearQuantifierElimination {
			eliminated, done, e = eliminateQuantifierArithmeticEquations(m, q, config)
		}
		if e == nil && !done && !config.nonlinearQuantifierElimination {
			eliminated, done, e = eliminateQuantifierArithmeticClosed(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierArithmeticSearch(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierBitVectorSearch(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierBitVectorBlock(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierDatatypeSearch(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierDatatypeBlock(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierDatalogSearch(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierDatalogBlock(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierArrayEquations(m, q, config)
		}
		if e == nil && !done {
			eliminated, done, e = eliminateQuantifierMixed(m, q, config)
		}
		if e != nil || !done {
			return result, false, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&eliminated)) }()
		value, e := m.RefGet(eliminated)
		if e != nil {
			return result, false, e
		}
		result, err = instantiated.abstract(value)
		return result, err == nil, err
	case ast.ASTApp:
		count, e := m.GetNumArgs(expression)
		if e != nil {
			return result, false, e
		}
		args := make([]ast.ASTHandle, count)
		refs := make([]ast.ASTRef, 0, count)
		defer func() {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, m.DelRef(&refs[i]))
			}
		}()
		changed := false
		for i := range args {
			old, x := m.GetArg(expression, uint32(i))
			if x != nil {
				return result, false, x
			}
			ref, done, x := rewriteQuantifierScalar(m, old, config)
			if x != nil || !done {
				return result, false, x
			}
			refs = append(refs, ref)
			args[i], x = m.RefGet(ref)
			if x != nil {
				return result, false, x
			}
			changed = changed || args[i] != old
		}
		if changed {
			decl, x := m.GetDecl(expression)
			if x != nil {
				return result, false, x
			}
			expression, x = m.MkApp(decl, args)
			if x != nil {
				return result, false, x
			}
		}
		result, err = m.NewRef(expression)
		return result, err == nil, err
	default:
		result, err = m.NewRef(expression)
		return result, err == nil, err
	}
}
