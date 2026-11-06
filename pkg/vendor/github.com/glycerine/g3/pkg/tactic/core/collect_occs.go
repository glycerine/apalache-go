package core

// Source: src/tactic/core/collect_occs.cpp/.h. The retained consumer creates a
// fresh collector and an empty result for each elim-uncnstr round.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// collectSingleOccurrenceVariables returns borrowed constants in discovery
// order. The goal owns their lifetime. Maps are lookup-only; the ordered slice
// supplies traversal order to the consumer without depending on Go map order.
func collectSingleOccurrenceVariables(goal *tactic.Goal) ([]ast.ASTHandle, error) {
	if goal == nil || goal.Manager() == nil {
		return nil, ast.ErrASTHandle
	}
	m := goal.Manager()
	visited := make(map[ast.ASTHandle]bool)
	repeated := make(map[ast.ASTHandle]bool)
	var variables []ast.ASTHandle
	type frame struct {
		term ast.ASTHandle
		next uint32
	}
	var stack []frame
	visit := func(term ast.ASTHandle) (bool, error) {
		kind, err := m.GetKind(term)
		if err != nil {
			return false, err
		}
		constant := false
		var count uint32
		if kind == ast.ASTApp {
			count, err = m.GetNumArgs(term)
			if err != nil {
				return false, err
			}
			if count == 0 {
				family, e := m.GetFamilyID(term)
				if e != nil {
					return false, e
				}
				constant = family == ast.NullFamilyID
			}
		}
		if visited[term] {
			if constant {
				repeated[term] = true
			}
			return true, nil
		}
		visited[term] = true
		if constant {
			variables = append(variables, term)
			return true, nil
		}
		if kind == ast.ASTVar || kind == ast.ASTApp && count == 0 {
			return true, nil
		}
		if kind != ast.ASTApp && kind != ast.ASTQuantifier {
			return false, ast.ErrASTDomain
		}
		stack = append(stack, frame{term: term})
		return false, nil
	}
	size, err := goal.Size()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < size; i++ {
		term, err := goal.Form(i)
		if err != nil {
			return nil, err
		}
		if _, err = visit(term); err != nil {
			return nil, err
		}
		for len(stack) != 0 {
			top := len(stack) - 1
			term = stack[top].term
			kind, err := m.GetKind(term)
			if err != nil {
				return nil, err
			}
			if kind == ast.ASTQuantifier {
				// Source intentionally revisits the body after returning from
				// a compound child. visit sees its mark and retires this frame.
				body, err := m.GetQuantifierBody(term)
				if err != nil {
					return nil, err
				}
				stack[top].next++
				done, err := visit(body)
				if err != nil {
					return nil, err
				}
				if done {
					stack = stack[:top]
				}
				continue
			}
			count, err := m.GetNumArgs(term)
			if err != nil {
				return nil, err
			}
			if stack[top].next == count {
				stack = stack[:top]
				continue
			}
			child, err := m.GetArg(term, stack[top].next)
			if err != nil {
				return nil, err
			}
			stack[top].next++
			if _, err = visit(child); err != nil {
				return nil, err
			}
		}
	}
	result := variables[:0]
	for _, variable := range variables {
		if !repeated[variable] {
			result = append(result, variable)
		}
	}
	return result, nil
}
