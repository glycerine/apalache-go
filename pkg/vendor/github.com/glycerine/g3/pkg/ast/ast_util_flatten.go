package ast

// Source: src/ast/ast_util.cpp:254-366; src/ast/ast_util.h:146-153.

func (m *ASTManager) astUtilRawNot(r astNodeRef) (astNodeRef, error) {
	args := [1]ASTHandle{m.external(r)}
	return m.mkFamilyApp(0, ASTOpNot, nil, args[:])
}

func (m *ASTManager) astUtilUnary(r astNodeRef, op int32) (astNodeRef, bool, error) {
	yes, e := m.astUtilOperator(r, op)
	if e != nil || !yes {
		return astNodeRef{}, false, e
	}
	a := m.appAt(r.slot)
	if a.count != 1 {
		return astNodeRef{}, false, ErrASTDomain
	}
	child, e := m.readInternal(a.args, 0)
	return child, true, e
}

func (m *ASTManager) astUtilBinary(r astNodeRef, op int32) (astNodeRef, astNodeRef, bool, error) {
	yes, e := m.astUtilOperator(r, op)
	if e != nil || !yes {
		return astNodeRef{}, astNodeRef{}, false, e
	}
	a := m.appAt(r.slot)
	if a.count != 2 {
		return astNodeRef{}, astNodeRef{}, false, ErrASTDomain
	}
	left, e := m.readInternal(a.args, 0)
	if e != nil {
		return astNodeRef{}, astNodeRef{}, false, e
	}
	right, e := m.readInternal(a.args, 1)
	return left, right, true, e
}

func (m *ASTManager) astUtilFlattenAnd(result []ASTHandle) ([]ASTHandle, error) {
	for i := 0; i < len(result); {
		r, e := m.requireExpr(result[i])
		if e != nil {
			return result, e
		}
		if yes, e := m.astUtilOperator(r, ASTOpAnd); e != nil {
			return result, e
		} else if yes {
			a := m.appAt(r.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.readInternal(a.args, j)
				if e != nil {
					return result, e
				}
				result = append(result, m.external(child))
			}
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		inner, isNot, e := m.astUtilUnary(r, ASTOpNot)
		if e != nil {
			return result, e
		}
		if isNot {
			if twice, yes, e := m.astUtilUnary(inner, ASTOpNot); e != nil {
				return result, e
			} else if yes {
				result[i] = m.external(twice)
				continue
			}
			if yes, e := m.astUtilOperator(inner, ASTOpOr); e != nil {
				return result, e
			} else if yes {
				a := m.appAt(inner.slot)
				for j := uint32(0); j < a.count; j++ {
					child, e := m.readInternal(a.args, j)
					if e != nil {
						return result, e
					}
					negated, e := m.astUtilRawNot(child)
					if e != nil {
						return result, e
					}
					result = append(result, m.external(negated))
				}
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if left, right, yes, e := m.astUtilBinary(inner, ASTOpImplies); e != nil {
				return result, e
			} else if yes {
				result = append(result, m.external(left))
				negated, e := m.astUtilRawNot(right)
				if e != nil {
					return result, e
				}
				result[i] = m.external(negated)
				continue
			}
			if inner == m.falseApp {
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if inner == m.trueApp {
				return []ASTHandle{m.external(m.falseApp)}, nil
			}
		}
		if r == m.trueApp {
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		if r == m.falseApp {
			return []ASTHandle{m.external(m.falseApp)}, nil
		}
		i++
	}
	return result, nil
}

func (m *ASTManager) astUtilFlattenOr(result []ASTHandle) ([]ASTHandle, error) {
	for i := 0; i < len(result); {
		r, e := m.requireExpr(result[i])
		if e != nil {
			return result, e
		}
		if yes, e := m.astUtilOperator(r, ASTOpOr); e != nil {
			return result, e
		} else if yes {
			a := m.appAt(r.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.readInternal(a.args, j)
				if e != nil {
					return result, e
				}
				result = append(result, m.external(child))
			}
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		inner, isNot, e := m.astUtilUnary(r, ASTOpNot)
		if e != nil {
			return result, e
		}
		if isNot {
			if twice, yes, e := m.astUtilUnary(inner, ASTOpNot); e != nil {
				return result, e
			} else if yes {
				result[i] = m.external(twice)
				continue
			}
			if yes, e := m.astUtilOperator(inner, ASTOpAnd); e != nil {
				return result, e
			} else if yes {
				a := m.appAt(inner.slot)
				for j := uint32(0); j < a.count; j++ {
					child, e := m.readInternal(a.args, j)
					if e != nil {
						return result, e
					}
					negated, e := m.astUtilRawNot(child)
					if e != nil {
						return result, e
					}
					result = append(result, m.external(negated))
				}
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if inner == m.trueApp {
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if inner == m.falseApp {
				return []ASTHandle{m.external(m.trueApp)}, nil
			}
		}
		if left, right, yes, e := m.astUtilBinary(r, ASTOpImplies); e != nil {
			return result, e
		} else if yes {
			result = append(result, m.external(right))
			negated, e := m.astUtilRawNot(left)
			if e != nil {
				return result, e
			}
			result[i] = m.external(negated)
			continue
		}
		if r == m.falseApp {
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		if r == m.trueApp {
			return []ASTHandle{m.external(m.trueApp)}, nil
		}
		i++
	}
	return result, nil
}

// FlattenAnd rewrites a caller-owned conjunction vector in the source order.
func (m *ASTManager) FlattenAnd(result []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	return m.astUtilFlattenAnd(result)
}

// FlattenOr rewrites a caller-owned disjunction vector in the source order.
func (m *ASTManager) FlattenOr(result []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	return m.astUtilFlattenOr(result)
}
