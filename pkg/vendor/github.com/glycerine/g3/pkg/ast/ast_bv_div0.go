package ast

// Source: src/ast/bv_decl_plugin.h:107-126.

// BitVectorDivZeroKind selects the unary unspecified operation for an ordinary
// division, remainder or modulus. Other kinds violate the native precondition.
func BitVectorDivZeroKind(kind int32) (int32, error) {
	switch kind {
	case 7:
		return 12, nil
	case 8:
		return 13, nil
	case 9:
		return 14, nil
	case 10:
		return 15, nil
	case 11:
		return 16, nil
	default:
		return 61, ErrASTDomain
	}
}

// BitVectorDivZeroDeclaration preserves the input family and uses its first
// domain sort to construct the unary declaration. Parameters are discarded.
func (m *ASTManager) BitVectorDivZeroDeclaration(decl ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(decl, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	fid, kind, e := m.declIdentity(r)
	if e != nil {
		return ASTHandle{}, e
	}
	zeroKind, e := BitVectorDivZeroKind(kind)
	if e != nil {
		return ASTHandle{}, e
	}
	d := m.declAt(r.slot)
	if d.arity == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	first, e := m.readInternal(d.domain, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.mkFamilyFuncDecl(fid, zeroKind, nil, []ASTHandle{m.external(first)}, ASTHandle{})
}
