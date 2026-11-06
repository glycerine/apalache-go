package arith

// Source: src/ast/for_each_expr.h:27-105 and
// src/tactic/goal.h:161-166, concrete MarkAll goal specialization.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// postorder marks every expression across every goal root and visits arguments,
// quantifier bodies, patterns, and no-patterns before their parent.
func (c *nla2BVInvocation) postorder(g *tactic.Goal) ([]ast.ASTHandle, error) {
	type frame struct {
		h    ast.ASTHandle
		next uint32
	}
	m := c.t.manager
	seen := map[ast.ASTHandle]bool{}
	var result []ast.ASTHandle
	n, err := g.Size()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < n; i++ {
		root, err := g.Form(i)
		if err != nil {
			return nil, err
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		stack := []frame{{h: root}}
		for len(stack) > 0 {
			last := len(stack) - 1
			f := &stack[last]
			kind, err := m.GetKind(f.h)
			if err != nil {
				return nil, err
			}
			var count, patterns uint32
			switch kind {
			case ast.ASTApp:
				count, err = m.GetNumArgs(f.h)
			case ast.ASTQuantifier:
				patterns, err = m.GetNumPatterns(f.h)
				if err == nil {
					count, err = m.GetNumNoPatterns(f.h)
				}
				count += patterns + 1
			case ast.ASTVar:
			default:
				return nil, ast.ErrASTDomain
			}
			if err != nil {
				return nil, err
			}
			if f.next < count {
				index := f.next
				f.next++
				var child ast.ASTHandle
				if kind == ast.ASTApp {
					child, err = m.GetArg(f.h, index)
				} else if index == 0 {
					child, err = m.GetQuantifierBody(f.h)
				} else if index <= patterns {
					child, err = m.GetPattern(f.h, index-1)
				} else {
					child, err = m.GetNoPattern(f.h, index-1-patterns)
				}
				if err != nil {
					return nil, err
				}
				if !seen[child] {
					seen[child] = true
					stack = append(stack, frame{h: child})
				}
				continue
			}
			result = append(result, f.h)
			stack = stack[:last]
		}
	}
	return result, nil
}
