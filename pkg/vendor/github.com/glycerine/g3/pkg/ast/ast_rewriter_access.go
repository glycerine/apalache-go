package ast

// Source: src/ast/ast.h; src/ast/rewriter/rewriter_def.h.

func (m *ASTManager) RewriterMakeApp(d ASTHandle, args AppArguments) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	decl, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	h, e := m.mkAppArguments(decl, args)
	return m.external(h), e
}

func (m *ASTManager) RewriterAssociative(d ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	decl, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(decl)
	return flags.LeftAssociative && flags.RightAssociative, e
}

func (m *ASTManager) RewriterCommutative(d ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	decl, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(decl)
	return flags.Commutative, e
}

func (m *ASTManager) RewriterPattern(t ASTHandle) (bool, error) {
	kind, e := m.GetKind(t)
	if e != nil || kind != ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(t)
	if e != nil || fid != 2 {
		return false, e
	}
	op, e := m.GetDeclKind(t)
	return op == 0, e
}

// AppArgumentView borrows an application's checked handle array for a source
// callback. The application must remain alive until the view is discarded.
func (m *ASTManager) AppArgumentView(app ASTHandle) (AppArguments, error) {
	if e := m.lock(); e != nil {
		return AppArguments{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(app, ASTApp)
	if e != nil {
		return AppArguments{}, e
	}
	return AppArguments{app: app, appOwner: m, count: m.appAt(r.slot).count}, nil
}
