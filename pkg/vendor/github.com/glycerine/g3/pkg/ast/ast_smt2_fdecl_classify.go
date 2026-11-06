package ast

// Source: src/ast/ast_smt2_pp.cpp:75-99.

// SMT2IsIndexedFdecl follows the SMT2 printer's source parameter walk. An
// indexed declaration has a non-null family and at least one parameter, each
// of which is an integer, rational, or function declaration reference.
func (m *ASTManager) SMT2IsIndexedFdecl(decl ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return false, err
	}
	infoToken := m.declAt(r.slot).info
	if infoToken == (ASTFuncDeclInfo{}) {
		return false, nil
	}
	info, err := m.funcInfo(infoToken)
	if err != nil {
		return false, err
	}
	if info.family == NullFamilyID || info.count == 0 {
		return false, nil
	}
	for i := uint32(0); i < info.count; i++ {
		p, err := m.infoParameter(info, i)
		if err != nil {
			return false, err
		}
		v, err := m.parameter(p)
		if err != nil {
			return false, err
		}
		switch v.Kind {
		case ASTParameterInt, ASTParameterRational:
			continue
		case ASTParameterAST:
			if err := m.validNode(v.ast); err != nil {
				return false, err
			}
			if m.nodeAt(v.ast.slot).kind == ASTFuncDecl {
				continue
			}
		}
		return false, nil
	}
	return true, nil
}

// SMT2IsSortParam identifies the source "as" annotation: a sole sort
// parameter identical to the function declaration's range.
func (m *ASTManager) SMT2IsSortParam(decl ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return false, err
	}
	d := m.declAt(r.slot)
	if d.info == (ASTFuncDeclInfo{}) {
		return false, nil
	}
	info, err := m.funcInfo(d.info)
	if err != nil {
		return false, err
	}
	if info.family == NullFamilyID || info.count != 1 {
		return false, nil
	}
	p, err := m.infoParameter(info, 0)
	if err != nil {
		return false, err
	}
	v, err := m.parameter(p)
	if err != nil {
		return false, err
	}
	if v.Kind != ASTParameterAST {
		return false, nil
	}
	if err := m.validNode(v.ast); err != nil {
		return false, err
	}
	return m.nodeAt(v.ast.slot).kind == ASTSort && d.rangeSort == v.ast, nil
}
