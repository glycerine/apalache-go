package ast

// Source: src/ast/fpa_decl_plugin.cpp:980-1037.

// ContainsFloatingPoint reports whether an AST contains a FloatingPoint or
// RoundingMode sort. It follows fpa_util::contains_floats through expression
// children, binder sorts, declaration signatures, and AST-valued parameters.
func (m *ASTManager) ContainsFloatingPoint(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.containsFloatingPoint(r)
}

func (m *ASTManager) containsFloatingPoint(root astNodeRef) (bool, error) {
	stack := []astNodeRef{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		r := stack[last]
		stack = stack[:last]
		if e := m.validNode(r); e != nil {
			return false, e
		}
		switch m.nodeAt(r.slot).kind {
		case ASTApp:
			v := m.appAt(r.slot)
			for i := v.count; i != 0; i-- {
				child, e := m.readInternal(v.args, i-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
			stack = append(stack, v.decl)
		case ASTVar:
			stack = append(stack, m.varAt(r.slot).sort)
		case ASTQuantifier:
			v := m.quantifierAt(r.slot)
			// Source visits children (body, patterns, no-patterns), declaration
			// sorts, then the body once more. Push in reverse for the same order.
			stack = append(stack, v.body)
			for i := v.count; i != 0; i-- {
				child, e := m.readInternal(v.sorts, i-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
			for i := v.noPatternCount; i != 0; i-- {
				child, e := m.readInternal(v.noPatterns, i-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
			for i := v.patternCount; i != 0; i-- {
				child, e := m.readInternal(v.patterns, i-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
			stack = append(stack, v.body)
		case ASTSort:
			fid, kind, e := m.declIdentity(r)
			if e != nil {
				return false, e
			}
			if p := m.floatingPointPlugin; p != nil && fid == p.family && (kind == 0 || kind == 1) {
				return true, nil
			}
			infoRef := m.sortAt(r.slot).info
			if infoRef == (ASTSortInfo{}) {
				continue
			}
			info, e := m.sortInfo(infoRef)
			if e != nil {
				return false, e
			}
			for i := info.count; i != 0; i-- {
				parameter, e := m.infoParameter(info, i-1)
				if e != nil {
					return false, e
				}
				value, e := m.parameter(parameter)
				if e != nil {
					return false, e
				}
				if value.Kind == ASTParameterAST {
					stack = append(stack, value.ast)
				}
			}
		case ASTFuncDecl:
			v := m.declAt(r.slot)
			if v.info != (ASTFuncDeclInfo{}) {
				info, e := m.funcInfo(v.info)
				if e != nil {
					return false, e
				}
				for i := info.count; i != 0; i-- {
					parameter, e := m.infoParameter(info, i-1)
					if e != nil {
						return false, e
					}
					value, e := m.parameter(parameter)
					if e != nil {
						return false, e
					}
					if value.Kind == ASTParameterAST {
						stack = append(stack, value.ast)
					}
				}
			}
			stack = append(stack, v.rangeSort)
			for i := v.arity; i != 0; i-- {
				child, e := m.readInternal(v.domain, i-1)
				if e != nil {
					return false, e
				}
				stack = append(stack, child)
			}
		default:
			return false, ErrASTDomain
		}
	}
	return false, nil
}
