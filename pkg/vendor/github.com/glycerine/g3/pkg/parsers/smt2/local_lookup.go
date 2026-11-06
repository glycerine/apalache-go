package smt2

// Source: src/parsers/smt2/smt2parser.cpp:1699-1721,1723-1729,1735-1754.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var errLocalBindingLevel = errors.New("smt2 parser: local binding level exceeds current bindings")
var errLocalShifter = errors.New("smt2 parser: variable shifter is unavailable")
var errLocalQualifiedSort = errors.New("invalid qualified identifier, sort mismatch")
var errLocalIndexed = errors.New("invalid indexed identifier, symbol is a local declaration")

// resolveLocal is the local-identifier branch of parse_expr_name. A found
// result is an owning expression-stack reference; the parser releases it when
// its frame pops. A missing name is resolved by the command context.
func (e *smt2LocalEnv) resolveLocal(m *ast.ASTManager, shifter *rewriter.VarShifter, name usymbol.Symbol, numBindings uint32) (ast.ASTRef, bool, error) {
	l, found := e.find(name)
	if !found {
		return ast.ASTRef{}, false, nil
	}
	ground, err := m.IsGround(l.term)
	if err != nil {
		return ast.ASTRef{}, true, err
	}
	if ground || l.level == numBindings {
		ref, err := m.NewRef(l.term)
		return ref, true, err
	}
	if l.level > numBindings {
		return ast.ASTRef{}, true, errLocalBindingLevel
	}
	if shifter == nil {
		return ast.ASTRef{}, true, errLocalShifter
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, true, err
	}
	if err = shifter.ShiftBy(l.term, numBindings-l.level, ref); err != nil {
		return ast.ASTRef{}, true, errors.Join(err, m.DelRef(&ref))
	}
	return ref, true, nil
}

// pushLocal is the retained parse_expr_name local branch's expression-stack
// action. The stack takes the reference produced by push_local.
func (e *smt2LocalEnv) pushLocal(m *ast.ASTManager, shifter *rewriter.VarShifter, name usymbol.Symbol, numBindings uint32, exprs *localExprStack) (bool, error) {
	ref, found, err := e.resolveLocal(m, shifter, name, numBindings)
	if err != nil || !found {
		return found, err
	}
	exprs.pushOwned(ref)
	return true, nil
}

// resolveQualifiedLocal is the local branch of parse_qualified_name. Source
// order is push_local, optional sort check, then rejection of index parameters.
// A refused branch releases the newly pushed expression-stack reference.
func (e *smt2LocalEnv) resolveQualifiedLocal(m *ast.ASTManager, shifter *rewriter.VarShifter, name usymbol.Symbol, numBindings uint32, sorts *localSortStack, hasSort bool, numIndices uint32) (ast.ASTRef, bool, error) {
	ref, found, err := e.resolveLocal(m, shifter, name, numBindings)
	if err != nil || !found {
		return ref, found, err
	}
	if hasSort {
		h, e := m.RefGet(ref)
		if e != nil {
			return ast.ASTRef{}, true, errors.Join(e, m.DelRef(&ref))
		}
		expectedSort, e := sorts.top(m)
		if e != nil {
			return ast.ASTRef{}, true, errors.Join(e, m.DelRef(&ref))
		}
		sort, e := m.GetSort(h)
		if e != nil {
			return ast.ASTRef{}, true, errors.Join(e, m.DelRef(&ref))
		}
		if sort != expectedSort {
			return ast.ASTRef{}, true, errors.Join(errLocalQualifiedSort, m.DelRef(&ref))
		}
		if e := sorts.pop(m); e != nil {
			return ast.ASTRef{}, true, errors.Join(e, m.DelRef(&ref))
		}
	}
	if numIndices != 0 {
		return ast.ASTRef{}, true, errors.Join(errLocalIndexed, m.DelRef(&ref))
	}
	return ref, true, nil
}

// pushQualifiedLocal is the corresponding parse_qualified_name local branch.
// A missing local remains available to the caller's nonlocal resolution path.
func (e *smt2LocalEnv) pushQualifiedLocal(m *ast.ASTManager, shifter *rewriter.VarShifter, name usymbol.Symbol, numBindings uint32, sorts *localSortStack, hasSort bool, numIndices uint32, exprs *localExprStack) (bool, error) {
	ref, found, err := e.resolveQualifiedLocal(m, shifter, name, numBindings, sorts, hasSort, numIndices)
	if err != nil || !found {
		return found, err
	}
	exprs.pushOwned(ref)
	return true, nil
}
