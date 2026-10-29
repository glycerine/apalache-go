package ast

// Source: src/ast/ast_util.cpp:94-158; src/ast/ast_util.h:78-93.

func (m *ASTManager) astUtilOperator(r astNodeRef, kind int32) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	family, op, e := m.declIdentity(m.appAt(r.slot).decl)
	return family == 0 && op == kind, e
}

func (m *ASTManager) astUtilIsAtom(r astNodeRef) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind == ASTQuantifier {
		return false, nil
	}
	sort, e := m.sortOf(r)
	if e != nil {
		return false, e
	}
	if sort != m.basic.boolSort {
		return false, nil
	}
	if m.nodeAt(r.slot).kind == ASTVar {
		return true, nil
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, ErrASTDomain
	}
	a := m.appAt(r.slot)
	family, op, e := m.declIdentity(a.decl)
	if e != nil {
		return false, e
	}
	if family != 0 {
		return true, nil
	}
	if op == ASTOpTrue || op == ASTOpFalse {
		return true, nil
	}
	if op != ASTOpEq || a.count == 0 {
		return false, nil
	}
	arg, e := m.readInternal(a.args, 0)
	if e != nil {
		return false, e
	}
	argSort, e := m.sortOf(arg)
	return argSort != m.basic.boolSort, e
}

func (m *ASTManager) astUtilIsLiteral(r astNodeRef) (bool, error) {
	atom, e := m.astUtilIsAtom(r)
	if e != nil || atom {
		return atom, e
	}
	negated, e := m.astUtilOperator(r, ASTOpNot)
	if e != nil || !negated {
		return false, e
	}
	a := m.appAt(r.slot)
	if a.count != 1 {
		return false, ErrASTDomain
	}
	child, e := m.readInternal(a.args, 0)
	if e != nil {
		return false, e
	}
	return m.astUtilIsAtom(child)
}

func (m *ASTManager) astUtilLiteralAtomSign(r astNodeRef) (astNodeRef, bool, error) {
	literal, e := m.astUtilIsLiteral(r)
	if e != nil {
		return astNodeRef{}, false, e
	}
	if !literal {
		return astNodeRef{}, false, ErrASTDomain
	}
	atom, e := m.astUtilIsAtom(r)
	if e != nil || atom {
		return r, false, e
	}
	child, e := m.readInternal(m.appAt(r.slot).args, 0)
	return child, true, e
}

func (m *ASTManager) astUtilIsClause(r astNodeRef) (bool, error) {
	literal, e := m.astUtilIsLiteral(r)
	if e != nil || literal {
		return literal, e
	}
	disjunction, e := m.astUtilOperator(r, ASTOpOr)
	if e != nil || !disjunction {
		return false, e
	}
	a := m.appAt(r.slot)
	for i := uint32(0); i < a.count; i++ {
		child, e := m.readInternal(a.args, i)
		if e != nil {
			return false, e
		}
		literal, e = m.astUtilIsLiteral(child)
		if e != nil || !literal {
			return false, e
		}
	}
	return true, nil
}

func (m *ASTManager) IsAtom(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.astUtilIsAtom(r)
}

func (m *ASTManager) IsLiteral(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.astUtilIsLiteral(r)
}

// LiteralAtomSign returns the literal's atom and whether the literal is
// negated. A nonliteral is rejected as the source assertion precondition.
func (m *ASTManager) LiteralAtomSign(h ASTHandle) (ASTHandle, bool, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return ASTHandle{}, false, e
	}
	atom, sign, e := m.astUtilLiteralAtomSign(r)
	return m.external(atom), sign, e
}

func (m *ASTManager) IsClause(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.astUtilIsClause(r)
}

func (m *ASTManager) ClauseNumLiterals(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return 0, e
	}
	clause, e := m.astUtilIsClause(r)
	if e != nil {
		return 0, e
	}
	if !clause {
		return 0, ErrASTDomain
	}
	literal, e := m.astUtilIsLiteral(r)
	if e != nil || literal {
		return 1, e
	}
	return m.appAt(r.slot).count, nil
}

func (m *ASTManager) ClauseLiteral(h ASTHandle, index uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return ASTHandle{}, e
	}
	clause, e := m.astUtilIsClause(r)
	if e != nil {
		return ASTHandle{}, e
	}
	if !clause {
		return ASTHandle{}, ErrASTDomain
	}
	literal, e := m.astUtilIsLiteral(r)
	if e != nil {
		return ASTHandle{}, e
	}
	if literal {
		if index != 0 {
			return ASTHandle{}, ErrASTDomain
		}
		return h, nil
	}
	a := m.appAt(r.slot)
	if index >= a.count {
		return ASTHandle{}, ErrASTDomain
	}
	child, e := m.readInternal(a.args, index)
	return m.external(child), e
}
