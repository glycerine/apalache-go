package ast

// Source: src/ast/ast_util.cpp:173-180; src/ast/ast.cpp:2064-2211.

// This concrete call-local argument reader selects actual borrowed storage.
// It never clones a pointer array or grants ownership of its elements.
type AppArguments struct {
	argumentOffset uint32
	app            ASTHandle
	appOwner       *ASTManager
	slice          []ASTHandle
	span           ASTLiteralVectorSpan
	count          uint32
	raw            bool
}

func (a AppArguments) at(i uint32) (ASTHandle, error) {
	if i >= a.count || i > ^uint32(0)-a.argumentOffset {
		return ASTHandle{}, ErrASTDomain
	}
	i += a.argumentOffset
	if a.appOwner != nil {
		m := a.appOwner
		r, e := m.requireKind(a.app, ASTApp)
		if e != nil {
			return ASTHandle{}, e
		}
		app := m.appAt(r.slot)
		if i >= app.count {
			return ASTHandle{}, ErrASTDomain
		}
		child, e := m.readInternal(app.args, i)
		return m.external(child), e
	}
	if a.raw {
		return a.span.At(i)
	}
	return a.slice[i], nil
}

func (a AppArguments) size() uint64 { return uint64(a.count) }
func (m *ASTManager) MkOrSpan(n uint32, span ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	switch n {
	case 0:
		return m.external(m.falseApp), nil
	case 1:
		return span.At(0)
	}
	// Source basic-plugin expr overload uses this same OR declaration.
	if !m.pluginAt(0).present {
		return ASTHandle{}, nil
	}
	r, e := m.mkAppArguments(m.basic.declarations[ASTOpOr], AppArguments{span: span, count: n, raw: true})
	return m.external(r), e
}

// At reads a borrowed argument without acquiring ownership.
func (a AppArguments) At(i uint32) (ASTHandle, error) { return a.at(i) }
func (a AppArguments) Count() uint32                  { return a.count }
func (a AppArguments) Size() uint64                   { return a.size() }
func SliceArguments(values []ASTHandle) AppArguments {
	return AppArguments{slice: values, count: uint32(len(values))}
}
func (m *ASTManager) UpdateQuantifierArguments(q, body ASTHandle, patterns, noPatterns AppArguments) (ASTHandle, error) {
	return m.updateQuantifierArguments(q, body, false, 0, patterns, noPatterns, astUpdatePatterns|astUpdateNoPatterns)
}

// Range borrows a checked subrange without copying its backing arguments.
func (a AppArguments) Range(offset, count uint32) (AppArguments, error) {
	if offset > a.count || count > a.count-offset {
		return AppArguments{}, ErrASTDomain
	}
	if offset > ^uint32(0)-a.argumentOffset {
		return AppArguments{}, ErrASTDomain
	}
	a.argumentOffset += offset
	a.count = count
	return a, nil
}
