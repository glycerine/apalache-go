package ast

// Source: src/ast/bv_decl_plugin.cpp:571-630; src/ast/bv_decl_plugin.h:406;
// src/ast/ast.cpp:1863-1881,2064-2211.

// MkBitVectorOrArguments borrows the actual operand storage through the
// original cached declaration and application factories, without a clone.
func (m *ASTManager) MkBitVectorOrArguments(args AppArguments) (ASTHandle, error) {
	return m.mkBitVectorArguments(31, args)
}

// Source: src/ast/bv_decl_plugin.h:408.
func (m *ASTManager) MkBitVectorXorArguments(args AppArguments) (ASTHandle, error) {
	return m.mkBitVectorArguments(33, args)
}

// Source: src/ast/bv_decl_plugin.h:404.
// MkBitVectorConcatArguments borrows the original argument array, including
// suffix ranges used when extract propagation removes a leading segment.
func (m *ASTManager) MkBitVectorConcatArguments(args AppArguments) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.mkFamilyAppDomainArguments(m.bitVectorPlugin.family, 37, nil, args)
	return m.external(r), e
}

func (m *ASTManager) mkBitVectorArguments(kind int32, args AppArguments) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.bitVectorArgumentsCore(kind, args)
}

func (m *ASTManager) bitVectorArgumentsCore(kind int32, args AppArguments) (ASTHandle, error) {
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	d, handled, e := m.bitVectorExprDeclArguments(kind, nil, args)
	if e != nil {
		return ASTHandle{}, e
	}
	if !handled {
		return ASTHandle{}, ErrASTDomain
	}
	if d.slot == 0 {
		return ASTHandle{}, nil
	}
	r, e := m.mkAppArguments(d, args)
	return m.external(r), e
}

// Source: src/ast/ast.cpp:1568-1638; src/ast/bv_decl_plugin.cpp:571-630.
// MkBitVectorAddArguments borrows the original ADD operand storage, including
// the common suffix selected by bv_rewriter::are_eq_upto_num.
func (m *ASTManager) MkBitVectorAddArguments(args AppArguments) (ASTHandle, error) {
	return m.mkBitVectorArguments(4, args)
}
