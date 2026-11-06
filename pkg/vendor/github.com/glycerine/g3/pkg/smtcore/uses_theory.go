package smtcore

// Source: src/smt/uses_theory.h and src/smt/uses_theory.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtTheoryVisitFrame struct {
	node  ast.ASTHandle
	index uint32
}

func SMTUsesTheory(m *ast.ASTManager, expression ast.ASTHandle, family ast.FamilyID) (found bool, result error) {
	if m == nil {
		return false, ast.ErrASTDomain
	}
	visited, err := ast.NewASTExprMark(m)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, visited.Close()) }()
	return SMTUsesTheoryWithMark(m, expression, family, visited)
}

// SMTUsesTheoryWithMark checks only nodes absent from visited and adds every
// newly reached node to it, matching the source expr_mark overload.
func SMTUsesTheoryWithMark(m *ast.ASTManager, expression ast.ASTHandle, family ast.FamilyID, visited *ast.ASTExprMark) (bool, error) {
	if m == nil || visited == nil {
		return false, ast.ErrASTDomain
	}
	marked, err := visited.IsMarked(expression)
	if err != nil || marked {
		return false, err
	}
	if err = visited.Mark(expression, true); err != nil {
		return false, err
	}
	stack := []smtTheoryVisitFrame{{node: expression}}
	for len(stack) != 0 {
		frame := &stack[len(stack)-1]
		kind, err := m.GetKind(frame.node)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTVar:
			stack = stack[:len(stack)-1]
		case ast.ASTApp:
			count, err := m.GetNumArgs(frame.node)
			if err != nil {
				return false, err
			}
			if frame.index < count {
				child, err := m.GetArg(frame.node, frame.index)
				if err != nil {
					return false, err
				}
				frame.index++
				if pushed, err := smtPushUnvisited(child, visited, &stack); err != nil {
					return false, err
				} else if pushed {
					continue
				}
				continue
			}
			stack = stack[:len(stack)-1]
			actual, err := m.GetFamilyID(frame.node)
			if err != nil {
				return false, err
			}
			if actual == family {
				return true, nil
			}
		case ast.ASTQuantifier:
			bodyCount := uint32(1)
			patterns, err := m.GetNumPatterns(frame.node)
			if err != nil {
				return false, err
			}
			noPatterns, err := m.GetNumNoPatterns(frame.node)
			if err != nil {
				return false, err
			}
			count := bodyCount + patterns + noPatterns
			if frame.index >= count {
				stack = stack[:len(stack)-1]
				continue
			}
			index := frame.index
			frame.index++
			var child ast.ASTHandle
			switch {
			case index == 0:
				child, err = m.GetQuantifierBody(frame.node)
			case index <= patterns:
				child, err = m.GetPattern(frame.node, index-1)
			default:
				child, err = m.GetNoPattern(frame.node, index-patterns-1)
			}
			if err != nil {
				return false, err
			}
			if _, err = smtPushUnvisited(child, visited, &stack); err != nil {
				return false, err
			}
		default:
			return false, ast.ErrASTDomain
		}
	}
	return false, nil
}

func smtPushUnvisited(child ast.ASTHandle, visited *ast.ASTExprMark, stack *[]smtTheoryVisitFrame) (bool, error) {
	marked, err := visited.IsMarked(child)
	if err != nil || marked {
		return false, err
	}
	if err = visited.Mark(child, true); err != nil {
		return false, err
	}
	*stack = append(*stack, smtTheoryVisitFrame{node: child})
	return true, nil
}
