package ast

// Source: src/ast/ast.h:1984,2008-2018,2031-2060; src/ast/ast_util.cpp:173-180.

func (m *ASTManager) MkFalse() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.falseApp), nil
}
func (m *ASTManager) MkTrue() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.trueApp), nil
}
func (m *ASTManager) MkNot(a ASTHandle) (ASTHandle, error) {
	args := [1]ASTHandle{a}
	return m.MkFamilyApp(0, ASTOpNot, nil, args[:])
}
func (m *ASTManager) MkOr(a, b ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{a, b}
	return m.MkFamilyApp(0, ASTOpOr, nil, args[:])
}
func (m *ASTManager) MkAnd(a, b ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{a, b}
	return m.MkFamilyApp(0, ASTOpAnd, nil, args[:])
}

// MkOrArray is ast_util's source overload: singleton returns its raw pointer
// without constructing a new node, and zero uses the pinned false constant.
func (m *ASTManager) MkOrArray(args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	switch len(args) {
	case 0:
		return m.external(m.falseApp), nil
	case 1:
		return args[0], nil
	}
	r, e := m.mkFamilyApp(0, ASTOpOr, nil, args)
	return m.external(r), e
}

// IsNot includes the source argument-output overload used by ast_ext2.
func (m *ASTManager) IsNot(a ASTHandle) (bool, ASTHandle, error) {
	if e := m.lock(); e != nil {
		return false, ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(a)
	if e != nil {
		return false, ASTHandle{}, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, ASTHandle{}, nil
	}
	app := m.appAt(r.slot)
	decl := m.declAt(app.decl.slot)
	if decl.info == (ASTFuncDeclInfo{}) {
		return false, ASTHandle{}, nil
	}
	info, e := m.funcInfo(decl.info)
	if e != nil {
		return false, ASTHandle{}, e
	}
	if info.family != 0 || info.kind != ASTOpNot {
		return false, ASTHandle{}, nil
	}
	child, e := m.readInternal(app.args, 0)
	return true, m.external(child), e
}
