package ast

// Source: src/ast/ast.h:1662,2033-2063; src/ast/ast.cpp:2503-2527.

func (m *ASTManager) mkEquality(lhs, rhs ASTHandle) (astNodeRef, error) {
	r, e := m.requireExpr(lhs)
	if e != nil {
		return astNodeRef{}, e
	}
	s, e := m.sortOf(r)
	if e != nil {
		return astNodeRef{}, e
	}
	k := int32(ASTOpEq)
	if s == m.basic.boolSort {
		k = ASTOpIff
	}
	args := [2]ASTHandle{lhs, rhs}
	return m.mkFamilyApp(0, k, nil, args[:])
}
func (m *ASTManager) mkDistinctNegatedPair(lhs, rhs ASTHandle) (astNodeRef, error) {
	eq, e := m.mkEquality(lhs, rhs)
	if e != nil {
		return astNodeRef{}, e
	}
	// Hold the equality across the fallible NOT factory. A successful NOT
	// acquires its own child reference before this temporary is released.
	if e = m.incNode(eq); e != nil {
		return astNodeRef{}, e
	}
	unary := [1]ASTHandle{m.external(eq)}
	neg, e := m.mkFamilyApp(0, ASTOpNot, nil, unary[:])
	_, release := m.decNode(eq)
	if e != nil {
		return astNodeRef{}, e
	}
	if release != nil {
		return astNodeRef{}, release
	}
	return neg, nil
}
func (m *ASTManager) MkEq(lhs, rhs ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.mkEquality(lhs, rhs)
	return m.external(r), e
}
func (m *ASTManager) MkIff(lhs, rhs ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{lhs, rhs}
	return m.MkFamilyApp(0, ASTOpIff, nil, args[:])
}
func (m *ASTManager) MkOEq(lhs, rhs ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{lhs, rhs}
	return m.MkFamilyApp(0, ASTOpOEq, nil, args[:])
}
func (m *ASTManager) MkXor(lhs, rhs ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{lhs, rhs}
	return m.MkFamilyApp(0, ASTOpXor, nil, args[:])
}
func (m *ASTManager) MkIte(c, t, e ASTHandle) (ASTHandle, error) {
	args := [3]ASTHandle{c, t, e}
	return m.MkFamilyApp(0, ASTOpIte, nil, args[:])
}
func (m *ASTManager) MkImplies(lhs, rhs ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{lhs, rhs}
	return m.MkFamilyApp(0, ASTOpImplies, nil, args[:])
}
func (m *ASTManager) MkInterp(a ASTHandle) (ASTHandle, error) {
	args := [1]ASTHandle{a}
	return m.MkFamilyApp(0, ASTOpInterp, nil, args[:])
}
func (m *ASTManager) MkAnd3(a, b, c ASTHandle) (ASTHandle, error) {
	args := [3]ASTHandle{a, b, c}
	return m.MkFamilyApp(0, ASTOpAnd, nil, args[:])
}
func (m *ASTManager) MkOr3(a, b, c ASTHandle) (ASTHandle, error) {
	args := [3]ASTHandle{a, b, c}
	return m.MkFamilyApp(0, ASTOpOr, nil, args[:])
}

// These ast_manager array overloads always construct an application. They
// differ from ast_util's separately ported singleton/empty MkOrArray shortcut.
func (m *ASTManager) MkXorArgs(args []ASTHandle) (ASTHandle, error) {
	return m.MkFamilyApp(0, ASTOpXor, nil, args)
}
func (m *ASTManager) MkAndArgs(args []ASTHandle) (ASTHandle, error) {
	return m.MkFamilyApp(0, ASTOpAnd, nil, args)
}
func (m *ASTManager) MkOrArgs(args []ASTHandle) (ASTHandle, error) {
	return m.MkFamilyApp(0, ASTOpOr, nil, args)
}
func (m *ASTManager) MkBoolVal(b bool) (ASTHandle, error) {
	if b {
		return m.MkTrue()
	}
	return m.MkFalse()
}
func (m *ASTManager) MkDistinct(args []ASTHandle) (ASTHandle, error) {
	return m.MkFamilyApp(0, ASTOpDistinct, nil, args)
}
func (m *ASTManager) MkDistinctExpanded(args []ASTHandle) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(args) < 2 {
		return m.external(m.trueApp), nil
	}
	if len(args) == 2 {
		r, e := m.mkDistinctNegatedPair(args[0], args[1])
		return m.external(r), e
	}
	// Source new_diseqs owns each completed NOT until the final AND has
	// acquired its child references. Release in insertion order on every exit.
	var expanded []ASTHandle
	defer func() {
		for _, h := range expanded {
			r, e := m.internal(h)
			if e == nil {
				_, e = m.decNode(r)
			}
			if e != nil && err == nil {
				result, err = ASTHandle{}, e
			}
		}
	}()
	for i := 0; i < len(args)-1; i++ {
		a1 := args[i]
		for j := i + 1; j < len(args); j++ {
			neg, e := m.mkDistinctNegatedPair(a1, args[j])
			if e != nil {
				return ASTHandle{}, e
			}
			if e = m.incNode(neg); e != nil {
				return ASTHandle{}, e
			}
			expanded = append(expanded, m.external(neg))
		}
	}
	r, e := m.mkFamilyApp(0, ASTOpAnd, nil, expanded)
	if e == nil && m.distinctTraceEnabled() {
		// The SMT2 printer takes this manager's lock itself. The source TRACE
		// runs after construction and before the temporary disequalities die.
		m.mu.Unlock()
		traceErr := m.traceDistinctExpanded(m.external(r))
		m.mu.Lock()
		if traceErr != nil {
			return ASTHandle{}, traceErr
		}
	}
	return m.external(r), e
}
func (m *ASTManager) basicBooleanDeclaration(k int32, n uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	domain := [2]ASTHandle{m.external(m.basic.boolSort), m.external(m.basic.boolSort)}
	r, e := m.basicDecl(k, nil, domain[:n])
	return m.external(r), e
}
func (m *ASTManager) MkAndDecl() (ASTHandle, error) { return m.basicBooleanDeclaration(ASTOpAnd, 2) }
func (m *ASTManager) MkOrDecl() (ASTHandle, error)  { return m.basicBooleanDeclaration(ASTOpOr, 2) }
func (m *ASTManager) MkNotDecl() (ASTHandle, error) { return m.basicBooleanDeclaration(ASTOpNot, 1) }
