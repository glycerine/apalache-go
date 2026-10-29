package ast

// Source: src/ast/array_decl_plugin.cpp:566-569.

// ArrayFamilyID follows array_util construction: register the family name,
// independently of registering its declaration plugin.
func (m *ASTManager) ArrayFamilyID() (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return NullFamilyID, e
	}
	return m.families.MkID(name)
}

// Source: src/ast/ast.cpp:1568-1638; src/ast/array_decl_plugin.cpp:446-465.
// MkArrayAppArguments borrows an existing argument view. Declaration inference
// uses native sort scratch without allocating a second argument copy.
func (m *ASTManager) MkArrayAppArguments(fid FamilyID, op int32, args AppArguments) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkArrayAppArguments(fid, op, nil, args)
}

func (m *ASTManager) mkArrayAppArguments(fid FamilyID, op int32, ps []ASTParameter, args AppArguments) (ASTHandle, error) {
	if fid < 0 || !m.pluginAt(fid).present {
		return ASTHandle{}, nil
	}
	if m.pluginAt(fid).kind != astArrayPlugin {
		return ASTHandle{}, ErrASTDomain
	}
	var domain []ASTHandle
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return ASTHandle{}, e
		}
		r, e := m.requireExpr(h)
		if e != nil {
			return ASTHandle{}, e
		}
		s, e := m.sortOf(r)
		if e != nil {
			return ASTHandle{}, e
		}
		domain = append(domain, m.external(s))
	}
	d, e := m.arrayDecl(op, ps, domain, ASTHandle{})
	if e != nil || d.slot == 0 {
		return ASTHandle{}, e
	}
	r, e := m.mkAppArguments(d, args)
	return m.external(r), e
}
