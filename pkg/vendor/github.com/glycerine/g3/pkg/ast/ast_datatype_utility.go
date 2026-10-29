package ast

// Source: src/ast/datatype_decl_plugin.cpp:753-872;
// src/ast/datatype_decl_plugin.h:290-391.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) DatatypeFamilyID() (FamilyID, error) {
	if err := m.lock(); err != nil {
		return NullFamilyID, err
	}
	defer m.mu.Unlock()
	if m.datatypePlugin == nil || !m.datatypePlugin.published {
		return NullFamilyID, ErrASTDomain
	}
	return m.datatypePlugin.family, nil
}

func (m *ASTManager) datatypeDeclParameter(decl astNodeRef, index uint32) (ASTParameter, error) {
	info, err := m.funcInfo(m.declAt(decl.slot).info)
	if err != nil {
		return ASTParameter{}, err
	}
	return m.infoParameter(info, index)
}

func (m *ASTManager) datatypeDeclASTParameter(decl astNodeRef, index uint32) (astNodeRef, error) {
	parameter, err := m.datatypeDeclParameter(decl, index)
	if err != nil {
		return astNodeRef{}, err
	}
	value, err := m.parameter(parameter)
	if err != nil {
		return astNodeRef{}, err
	}
	if value.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	if err = m.validNode(value.ast); err != nil {
		return astNodeRef{}, err
	}
	return value.ast, nil
}

func (m *ASTManager) datatypeDeclSymbolParameter(decl astNodeRef, index uint32) (result usymbol.Symbol, err error) {
	parameter, err := m.datatypeDeclParameter(decl, index)
	if err != nil {
		return result, err
	}
	value, err := m.parameter(parameter)
	if err != nil {
		return result, err
	}
	if value.Kind != ASTParameterSymbol {
		return result, ErrASTDomain
	}
	return value.symbol, nil
}

// DatatypeConstructorApp returns the borrowed constructor declaration for an
// application. Non-applications and applications from other families do not
// match.
func (m *ASTManager) DatatypeConstructorApp(h ASTHandle) (decl ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.datatypePlugin == nil {
		return ASTHandle{}, false, nil
	}
	d := m.appAt(r.slot).decl
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != m.datatypePlugin.family || kind != ASTDatatypeConstructor {
		return ASTHandle{}, false, nil
	}
	return m.external(d), true, nil
}

// DatatypeRecognizerConstructor reads the constructor declaration stored by a
// recognizer or is declaration.
func (m *ASTManager) DatatypeRecognizerConstructor(decl ASTHandle) (constructor ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.datatypePlugin == nil || family != m.datatypePlugin.family || kind != ASTDatatypeRecognizer && kind != ASTDatatypeIs {
		return ASTHandle{}, false, nil
	}
	c, err := m.datatypeDeclASTParameter(d, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.nodeAt(c.slot).kind != ASTFuncDecl {
		return ASTHandle{}, false, ErrASTDomain
	}
	return m.external(c), true, nil
}

// DatatypeUpdateAccessor reads the accessor declaration stored by an
// update-field declaration.
func (m *ASTManager) DatatypeUpdateAccessor(decl ASTHandle) (accessor ASTHandle, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if m.datatypePlugin == nil || family != m.datatypePlugin.family || kind != ASTDatatypeUpdateField {
		return ASTHandle{}, false, nil
	}
	a, err := m.datatypeDeclASTParameter(d, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	return m.external(a), true, nil
}

// DatatypeAccessorIndex returns the constructor field selected by accessor.
func (m *ASTManager) DatatypeAccessorIndex(accessor, constructor ASTHandle) (index uint32, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	a, err := m.requireKind(accessor, ASTFuncDecl)
	if err != nil {
		return 0, false, err
	}
	c, err := m.requireKind(constructor, ASTFuncDecl)
	if err != nil {
		return 0, false, err
	}
	if m.datatypePlugin == nil {
		return 0, false, nil
	}
	af, ak, err := m.declIdentity(a)
	if err != nil {
		return 0, false, err
	}
	cf, ck, err := m.declIdentity(c)
	if err != nil {
		return 0, false, err
	}
	if af != m.datatypePlugin.family || ak != ASTDatatypeAccessor || cf != af || ck != ASTDatatypeConstructor {
		return 0, false, nil
	}
	accessorName, err := m.datatypeDeclSymbolParameter(a, 0)
	if err != nil {
		return 0, false, err
	}
	accessorConstructor, err := m.datatypeDeclSymbolParameter(a, 1)
	if err != nil {
		return 0, false, err
	}
	constructorName, err := m.datatypeDeclSymbolParameter(c, 0)
	if err != nil {
		return 0, false, err
	}
	if accessorConstructor != constructorName {
		return 0, false, nil
	}
	sort := m.declAt(c.slot).rangeSort
	domain, err := m.readInternal(m.declAt(a.slot).domain, 0)
	if err != nil {
		return 0, false, err
	}
	if sort != domain {
		return 0, false, nil
	}
	key := astDatatypeAccessorKey{sort: sort, constructor: constructorName}
	for i, name := range m.datatypePlugin.accessors[key] {
		if name == accessorName {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}
