package qe

// Source: src/qe/qe_datatype_plugin.cpp subst_rec equality branches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *DatatypeSolverPlugin) substituteEqualityBranch(target, original, constructor ast.ASTHandle, branch uint32, formula ast.ASTRef, definition *ast.ASTRef) (supported bool, err error) {
	entry := p.equalities[arithmeticPluginKey{variable: target, formula: original}]
	if entry == nil {
		return false, nil
	}
	if uint64(branch) > uint64(len(entry.positive)) {
		return false, ast.ErrASTDomain
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	for _, recognizer := range entry.recognizers {
		decl, e := p.manager.GetDecl(recognizer)
		if e != nil {
			return false, e
		}
		owner, matched, e := p.manager.DatatypeRecognizerConstructor(decl)
		if e != nil {
			return false, e
		}
		if !matched {
			return false, ast.ErrASTDomain
		}
		truth, e := p.manager.MkBoolVal(owner == constructor)
		if e != nil {
			return false, e
		}
		if e = replace.ApplySubstitution(recognizer, truth, formula); e != nil {
			return false, e
		}
	}
	falseValue, err := p.manager.MkFalse()
	if err != nil {
		return false, err
	}
	for _, atom := range entry.unsat {
		if err = replace.ApplySubstitution(atom, falseValue, formula); err != nil {
			return false, err
		}
	}
	if int(branch) < len(entry.positive) {
		selected := entry.positive[branch]
		value := selected.value
		if definition != nil {
			if err = p.manager.SetRef(*definition, value); err != nil {
				return false, err
			}
		}
		if err = replace.ApplySubstitution(target, value, formula); err != nil {
			return false, err
		}
		if selected.condition != (ast.ASTHandle{}) && !p.manager.IsTrue(selected.condition) {
			current, e := p.manager.RefGet(formula)
			if e != nil {
				return false, e
			}
			conjunction, e := p.manager.MkAnd(selected.condition, current)
			if e != nil {
				return false, e
			}
			if e = p.manager.SetRef(formula, conjunction); e != nil {
				return false, e
			}
		}
		return true, nil
	}
	for _, list := range [][]datatypeEqualityAtom{entry.positive, entry.negative} {
		for _, atom := range list {
			if err = replace.ApplySubstitution(atom.atom, falseValue, formula); err != nil {
				return false, err
			}
		}
	}
	if definition != nil {
		sort, e := p.manager.GetSort(target)
		if e != nil {
			return false, e
		}
		sorts, args := make([]ast.ASTHandle, len(entry.negative)), make([]ast.ASTHandle, len(entry.negative))
		for i, atom := range entry.negative {
			sorts[i], args[i] = sort, atom.value
		}
		name, e := p.manager.Symbols.Intern("diag")
		if e != nil {
			return false, e
		}
		decl, e := p.manager.MkFuncDecl(name, sorts, sort, ast.ASTFuncDeclInfo{})
		if e != nil {
			return false, e
		}
		declRef, e := p.manager.NewRef(decl)
		if e != nil {
			return false, e
		}
		defer func() { err = errors.Join(err, p.manager.DelRef(&declRef)) }()
		value, e := p.manager.MkApp(decl, args)
		if e != nil {
			return false, e
		}
		if e = p.manager.SetRef(*definition, value); e != nil {
			return false, e
		}
	}
	return true, nil
}
