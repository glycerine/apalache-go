package ast

// Source: src/ast/dl_decl_plugin.cpp:683-725;
// src/ast/dl_decl_plugin.h:131-134,176-190.

func (m *ASTManager) datalogArithmeticSort(h ASTHandle) (integer, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(h, ASTSort)
	if err != nil {
		return false, false, err
	}
	p := m.arithmeticPlugin
	if p == nil {
		return false, false, nil
	}
	if r == p.fields[1] {
		return true, true, nil
	}
	return false, r == p.fields[0], nil
}

func (m *ASTManager) isDatalogOperation(h ASTHandle, expected int32) (bool, error) {
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		r = m.appAt(r.slot).decl
	}
	if m.nodeAt(r.slot).kind != ASTFuncDecl {
		return false, nil
	}
	family, kind, e := m.declIdentity(r)
	p := m.auxiliaryPlugin(astDatalogPlugin)
	return e == nil && p != nil && family == p.family && kind == expected, e
}

func (m *ASTManager) IsDatalogLT(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.isDatalogOperation(h, DatalogOpLT)
}

func (m *ASTManager) IsDatalogNumeral(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return m.isDatalogOperation(h, astDatalogConstantKind)
}

// GetDatalogOpNames and GetDatalogSortNames preserve the source plugin's empty
// builtin-name publications for every logic.
func (m *ASTManager) GetDatalogOpNames(names *ASTBuiltinNames) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.auxiliaryPlugin(astDatalogPlugin) == nil {
		return ErrASTHandle
	}
	return nil
}

func (m *ASTManager) GetDatalogSortNames(names *ASTBuiltinNames) error {
	return m.GetDatalogOpNames(names)
}
