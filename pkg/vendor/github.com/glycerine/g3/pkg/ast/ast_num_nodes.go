package ast

// Source: src/ast/for_each_ast.cpp:20-33; src/ast/for_each_ast.h:42-134.

type astNodeCountFrame struct {
	node     astNodeRef
	children []astNodeRef
	next     int
}

// GetNumNodes returns the number of distinct AST nodes reached by the source
// default for_each_ast traversal. AST-valued declaration/sort parameters are
// excluded because get_num_nodes does not enable visit_parameters.
func (m *ASTManager) GetNumNodes(root ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(root)
	if e != nil {
		return 0, e
	}
	children := func(n astNodeRef) ([]astNodeRef, error) {
		var out []astNodeRef
		switch m.nodeAt(n.slot).kind {
		case ASTSort, ASTVar:
		case ASTFuncDecl:
			d := m.declAt(n.slot)
			for i := uint32(0); i < d.arity; i++ {
				child, err := m.readInternal(d.domain, i)
				if err != nil {
					return nil, err
				}
				out = append(out, child)
			}
			out = append(out, d.rangeSort)
		case ASTApp:
			a := m.appAt(n.slot)
			out = append(out, a.decl)
			for i := uint32(0); i < a.count; i++ {
				child, err := m.readInternal(a.args, i)
				if err != nil {
					return nil, err
				}
				out = append(out, child)
			}
		case ASTQuantifier:
			q := m.quantifierAt(n.slot)
			for i := uint32(0); i < q.patternCount; i++ {
				child, err := m.readInternal(q.patterns, i)
				if err != nil {
					return nil, err
				}
				out = append(out, child)
			}
			for i := uint32(0); i < q.noPatternCount; i++ {
				child, err := m.readInternal(q.noPatterns, i)
				if err != nil {
					return nil, err
				}
				out = append(out, child)
			}
			out = append(out, q.body)
		default:
			return nil, ErrASTDomain
		}
		return out, nil
	}
	first, e := children(r)
	if e != nil {
		return 0, e
	}
	marked := map[astNodeRef]bool{r: true}
	stack := []astNodeCountFrame{{node: r, children: first}}
	var count uint32
	for len(stack) != 0 {
		f := &stack[len(stack)-1]
		if f.next < len(f.children) {
			child := f.children[f.next]
			f.next++
			if marked[child] {
				continue
			}
			if e = m.validNode(child); e != nil {
				return 0, e
			}
			marked[child] = true
			cs, err := children(child)
			if err != nil {
				return 0, err
			}
			stack = append(stack, astNodeCountFrame{node: child, children: cs})
			continue
		}
		count++
		stack = stack[:len(stack)-1]
	}
	return count, nil
}
