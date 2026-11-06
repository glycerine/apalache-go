package ast

// Source: src/ast/ast_util.cpp:160-252; src/ast/ast_util.h:99-140.

func (m *ASTManager) astUtilMkAnd(args []ASTHandle) (astNodeRef, error) {
	switch len(args) {
	case 0:
		return m.trueApp, nil
	case 1:
		return m.requireExpr(args[0])
	default:
		return m.mkFamilyApp(0, ASTOpAnd, nil, args)
	}
}

func (m *ASTManager) astUtilMkOr(args []ASTHandle) (astNodeRef, error) {
	switch len(args) {
	case 0:
		return m.falseApp, nil
	case 1:
		return m.requireExpr(args[0])
	default:
		return m.mkFamilyApp(0, ASTOpOr, nil, args)
	}
}

func (m *ASTManager) astUtilMkNot(r astNodeRef) (astNodeRef, error) {
	negated, e := m.astUtilOperator(r, ASTOpNot)
	if e != nil {
		return astNodeRef{}, e
	}
	if negated {
		a := m.appAt(r.slot)
		if a.count != 1 {
			return astNodeRef{}, ErrASTDomain
		}
		return m.readInternal(a.args, 0)
	}
	if r == m.trueApp {
		return m.falseApp, nil
	}
	if r == m.falseApp {
		return m.trueApp, nil
	}
	args := [1]ASTHandle{m.external(r)}
	return m.mkFamilyApp(0, ASTOpNot, nil, args[:])
}

func (m *ASTManager) astUtilPushNot(r astNodeRef) (astNodeRef, error) {
	if e := m.validNode(r); e != nil {
		return astNodeRef{}, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		args := [1]ASTHandle{m.external(r)}
		return m.mkFamilyApp(0, ASTOpNot, nil, args[:])
	}
	a := m.appAt(r.slot)
	and, e := m.astUtilOperator(r, ASTOpAnd)
	if e != nil {
		return astNodeRef{}, e
	}
	or, e := m.astUtilOperator(r, ASTOpOr)
	if e != nil {
		return astNodeRef{}, e
	}
	if !and && !or {
		return m.astUtilMkNot(r)
	}
	children := make([]ASTHandle, 0, a.count)
	for i := uint32(0); i < a.count; i++ {
		child, e := m.readInternal(a.args, i)
		if e != nil {
			return astNodeRef{}, e
		}
		negated, e := m.astUtilPushNot(child)
		if e != nil {
			return astNodeRef{}, e
		}
		children = append(children, m.external(negated))
	}
	if and {
		return m.astUtilMkOr(children)
	}
	return m.astUtilMkAnd(children)
}

// MkAndArray is ast_util's zero/singleton/multiple overload.
func (m *ASTManager) MkAndArray(args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.astUtilMkAnd(args)
	return m.external(r), e
}

// MkNotSimplified removes a leading not and folds Boolean constants.
func (m *ASTManager) MkNotSimplified(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e = m.astUtilMkNot(r)
	return m.external(r), e
}

// PushNot negates h and applies De Morgan's laws through conjunctions and
// disjunctions, matching ast_util's recursive helper.
func (m *ASTManager) PushNot(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e = m.astUtilPushNot(r)
	return m.external(r), e
}

// MkDistinctSimplified folds arities zero and one to true and arity two to a
// disequality; larger inputs retain the basic distinct application.
func (m *ASTManager) MkDistinctSimplified(args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	switch len(args) {
	case 0, 1:
		return m.external(m.trueApp), nil
	case 2:
		r, e := m.mkDistinctNegatedPair(args[0], args[1])
		return m.external(r), e
	default:
		r, e := m.mkFamilyApp(0, ASTOpDistinct, nil, args)
		return m.external(r), e
	}
}
