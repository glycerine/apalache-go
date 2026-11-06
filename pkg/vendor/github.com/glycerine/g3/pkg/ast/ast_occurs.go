package ast

// Source: src/ast/occurs.cpp:26-76; src/ast/occurs.h:22-33.

type astOccursFrame struct {
	node        astNodeRef
	next, count uint32
}

func (m *ASTManager) occursChildren(r astNodeRef) uint32 {
	switch m.nodeAt(r.slot).kind {
	case ASTApp:
		return m.appAt(r.slot).count
	case ASTQuantifier:
		q := m.quantifierAt(r.slot)
		return 1 + q.patternCount + q.noPatternCount
	default:
		return 0
	}
}

func (m *ASTManager) occursChild(r astNodeRef, i uint32) (astNodeRef, error) {
	switch m.nodeAt(r.slot).kind {
	case ASTApp:
		return m.readInternal(m.appAt(r.slot).args, i)
	case ASTQuantifier:
		q := m.quantifierAt(r.slot)
		switch {
		case i == 0:
			return q.body, nil
		case i <= q.patternCount:
			return m.readInternal(q.patterns, i-1)
		default:
			return m.readInternal(q.noPatterns, i-1-q.patternCount)
		}
	default:
		return astNodeRef{}, ErrASTDomain
	}
}

func (m *ASTManager) occurs(root astNodeRef, expression *astNodeRef, declaration *astNodeRef) (bool, error) {
	visited := make(map[astNodeRef]bool)
	visited[root] = true
	stack := []astOccursFrame{{node: root, count: m.occursChildren(root)}}
	for len(stack) != 0 {
		frame := &stack[len(stack)-1]
		if frame.next < frame.count {
			child, e := m.occursChild(frame.node, frame.next)
			frame.next++
			if e != nil {
				return false, e
			}
			if !visited[child] {
				if e = m.validNode(child); e != nil {
					return false, e
				}
				visited[child] = true
				stack = append(stack, astOccursFrame{node: child, count: m.occursChildren(child)})
			}
			continue
		}
		r := frame.node
		stack = stack[:len(stack)-1]
		if expression != nil && r == *expression {
			return true, nil
		}
		if declaration != nil && m.nodeAt(r.slot).kind == ASTApp && m.appAt(r.slot).decl == *declaration {
			return true, nil
		}
	}
	return false, nil
}

// OccursExpr reports whether needle occurs in root.
func (m *ASTManager) OccursExpr(needle, root ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	n, e := m.requireExpr(needle)
	if e != nil {
		return false, e
	}
	r, e := m.requireExpr(root)
	if e != nil {
		return false, e
	}
	return m.occurs(r, &n, nil)
}

// OccursDecl reports whether an application of declaration occurs in root.
func (m *ASTManager) OccursDecl(declaration, root ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	d, e := m.internal(declaration)
	if e != nil {
		return false, e
	}
	if m.nodeAt(d.slot).kind != ASTFuncDecl {
		return false, ErrASTDomain
	}
	r, e := m.requireExpr(root)
	if e != nil {
		return false, e
	}
	return m.occurs(r, nil, &d)
}
