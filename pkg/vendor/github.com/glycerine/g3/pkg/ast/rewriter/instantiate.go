package rewriter

// Source: src/ast/rewriter/var_subst.cpp:160-174.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// InstantiateQuantifier substitutes the quantified declarations in standard
// order, then removes their de Bruijn level from free variables in the result.
func InstantiateQuantifier(m *ast.ASTManager, q ast.ASTHandle, values []ast.ASTHandle, out ast.ASTRef) (result error) {
	return InstantiateQuantifierWithOrder(m, q, values, true, out)
}

// InstantiateQuantifierWithOrder exposes the source var_subst order flag used
// by Spacer lemma bindings. Standard order binds variable zero to the last
// value; false binds it to the first value.
func InstantiateQuantifierWithOrder(m *ast.ASTManager, q ast.ASTHandle, values []ast.ASTHandle, standardOrder bool, out ast.ASTRef) (result error) {
	if m == nil {
		return ast.ErrASTHandle
	}
	n, e := m.GetNumDeclarations(q)
	if e != nil {
		return e
	}
	if uint64(len(values)) < uint64(n) {
		return ast.ErrASTDomain
	}
	body, e := m.GetQuantifierBody(q)
	if e != nil {
		return e
	}
	subst, e := NewVarSubstitution(m, standardOrder)
	if e != nil {
		if subst != nil {
			return errors.Join(e, subst.Close())
		}
		return e
	}
	defer func() { result = errors.Join(result, subst.Close()) }()
	intermediate, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, m.DelRef(&intermediate)) }()
	if e = subst.Substitute(body, values[:n], intermediate); e != nil {
		return e
	}
	rewritten, e := m.RefGet(intermediate)
	if e != nil {
		return e
	}
	if e = traceInstantiationSubstitution(m, q, rewritten); e != nil {
		return e
	}
	shifter, e := newInvVarShifter(m)
	if e != nil {
		if shifter != nil {
			return errors.Join(e, shifter.Close())
		}
		return e
	}
	defer func() { result = errors.Join(result, shifter.Close()) }()
	if e = shifter.InverseShift(rewritten, n, out); e != nil {
		return e
	}
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(debug)
	if e != nil {
		return e
	}
	published, e := m.RefGet(out)
	if e != nil {
		return e
	}
	if enabled {
		good, e := m.IsWellSorted(published, nil)
		if e != nil {
			return e
		}
		if !good {
			return diagnostic.DebugAssertionFailure(debug, "src/ast/rewriter/var_subst.cpp", 167, "is_well_sorted(m, result)")
		}
	}
	return traceInstantiationResult(m, q, values[:n], published)
}
