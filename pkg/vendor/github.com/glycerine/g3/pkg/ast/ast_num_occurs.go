package ast

// Source: src/ast/num_occurs.cpp; src/ast/num_occurs.h.

// NumOccurs counts incoming occurrences while traversing an expression DAG.
// It borrows every handle and retains no AST references. Each node's children
// are traversed once per Count/CountAll call, but every incoming edge is
// counted, matching num_occurs' VISIT-before-mark ordering.
type NumOccurs struct {
	manager           *ASTManager
	counts            map[ASTHandle]uint32
	ignoreRefCountOne bool
	ignoreQuantifiers bool
}

func NewNumOccurs(m *ASTManager, ignoreRefCountOne, ignoreQuantifiers bool) (*NumOccurs, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	return &NumOccurs{
		manager: m, counts: make(map[ASTHandle]uint32),
		ignoreRefCountOne: ignoreRefCountOne, ignoreQuantifiers: ignoreQuantifiers,
	}, nil
}

// Reset forgets all accumulated occurrence counts.
func (n *NumOccurs) Reset() {
	if n != nil {
		clear(n.counts)
	}
}

func (n *NumOccurs) visit(h ASTHandle, visited map[ASTHandle]struct{}, stack *[]ASTHandle) error {
	if n.ignoreRefCountOne {
		refs, e := n.manager.GetRefCount(h)
		if e != nil {
			return e
		}
		if refs > 1 {
			n.counts[h]++
		}
	} else {
		n.counts[h]++
	}
	if _, found := visited[h]; !found {
		visited[h] = struct{}{}
		*stack = append(*stack, h)
	}
	return nil
}

func (n *NumOccurs) process(root ASTHandle, visited map[ASTHandle]struct{}) error {
	if n == nil || n.manager == nil || n.counts == nil {
		return ErrASTHandle
	}
	stack := make([]ASTHandle, 0, 128)
	if e := n.visit(root, visited, &stack); e != nil {
		return e
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		kind, e := n.manager.GetKind(current)
		if e != nil {
			return e
		}
		switch kind {
		case ASTApp:
			count, e := n.manager.GetNumArgs(current)
			if e != nil {
				return e
			}
			for i := count; i != 0; i-- {
				child, e := n.manager.GetArg(current, i-1)
				if e != nil {
					return e
				}
				if e = n.visit(child, visited, &stack); e != nil {
					return e
				}
			}
		case ASTQuantifier:
			if !n.ignoreQuantifiers {
				body, e := n.manager.GetQuantifierBody(current)
				if e != nil {
					return e
				}
				if e = n.visit(body, visited, &stack); e != nil {
					return e
				}
			}
		case ASTVar:
		default:
			return ErrASTDomain
		}
	}
	return nil
}

// Count adds occurrences reachable from root to the current counts.
func (n *NumOccurs) Count(root ASTHandle) error {
	return n.process(root, make(map[ASTHandle]struct{}))
}

// CountAll adds occurrences from each root while sharing one visited set.
func (n *NumOccurs) CountAll(roots []ASTHandle) error {
	if n == nil || n.manager == nil || n.counts == nil {
		return ErrASTHandle
	}
	visited := make(map[ASTHandle]struct{})
	for _, root := range roots {
		if e := n.process(root, visited); e != nil {
			return e
		}
	}
	return nil
}

func (n *NumOccurs) GetNumOccurrences(h ASTHandle) uint32 {
	if n == nil {
		return 0
	}
	return n.counts[h]
}
