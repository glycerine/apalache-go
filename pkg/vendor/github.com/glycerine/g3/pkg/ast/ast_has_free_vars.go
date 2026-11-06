package ast

// Source: src/ast/has_free_vars.cpp:20-94; src/ast/has_free_vars.h:22-24.

import "math"

type astExprDelta struct {
	node  astNodeRef
	delta uint32
}

// HasFreeVars reports whether n contains a de Bruijn variable that is free at
// its occurrence. Quantifier annotations are deliberately excluded, matching
// contains_vars::visit_children.
func (m *ASTManager) HasFreeVars(n ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	root, e := m.requireExpr(n)
	if e != nil {
		return false, e
	}
	cache := make(map[astExprDelta]struct{})
	todo := []astExprDelta{{node: root}}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		visited := true
		switch m.nodeAt(current.node.slot).kind {
		case ASTVar:
			if m.varAt(current.node.slot).index >= current.delta {
				return true, nil
			}
		case ASTApp:
			a := m.appAt(current.node.slot)
			for j := a.count; j > 0; j-- {
				child, err := m.readInternal(a.args, j-1)
				if err != nil {
					return false, err
				}
				pair := astExprDelta{node: child, delta: current.delta}
				if _, found := cache[pair]; !found {
					todo = append(todo, pair)
					visited = false
				}
			}
		case ASTQuantifier:
			q := m.quantifierAt(current.node.slot)
			if current.delta <= math.MaxUint32-q.count {
				pair := astExprDelta{node: q.body, delta: current.delta + q.count}
				if _, found := cache[pair]; !found {
					todo = append(todo, pair)
					visited = false
				}
			}
		default:
			return false, ErrASTDomain
		}
		if visited {
			cache[current] = struct{}{}
			todo = todo[:len(todo)-1]
		}
	}
	return false, nil
}
