package qe

// Source: src/qe/qe.cpp, search_tree definitions and guarded leaves.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *exprQETree) addDefinition(variable, definition ast.ASTHandle) error {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	if variable.IsNull() || definition.IsNull() {
		return nil
	}
	decl, err := s.manager.GetDecl(variable)
	if err != nil {
		return err
	}
	return s.definitions.PushBack(decl, definition)
}

func (s *exprQETree) guardedLeaves(config Config) (result *GuardedDefinitions, err error) {
	if s == nil || s.manager == nil || s.parent != nil {
		return nil, ast.ErrASTHandle
	}
	result, err = NewGuardedDefinitions(s.manager, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	defs, err := NewDefinitionVector(s.manager, config)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, defs.Close()) }()
	var visit func(*exprQETree) error
	visit = func(node *exprQETree) (e error) {
		size, e := defs.Size()
		if e != nil {
			return e
		}
		if e = defs.Append(node.definitions); e != nil {
			return e
		}
		defer func() { e = errors.Join(e, defs.Shrink(size)) }()
		var formula ast.ASTHandle
		if node.formula != (ast.ASTRef{}) {
			formula, e = s.manager.RefGet(node.formula)
			if e != nil {
				return e
			}
		}
		if len(node.children) == 0 && !formula.IsNull() && !s.manager.IsFalse(formula) && len(node.variables) == 0 && node.selected == (ast.ASTRef{}) {
			return result.Add(formula, defs)
		}
		for _, child := range node.children {
			if e = visit(child); e != nil {
				return e
			}
		}
		return nil
	}
	if err = visit(s); err != nil {
		return result, err
	}
	return result, nil
}
