package ast

// Source: src/ast/ast.cpp:909-915,1127-1141,1246-1254,1589-1632,2556-2592; src/ast/ast.h:991-1020.

func (m *ASTManager) pluginValue(fid FamilyID, r astNodeRef) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	decl := m.appAt(r.slot).decl
	if fid == 0 {
		return decl == m.basic.declarations[ASTOpTrue] || decl == m.basic.declarations[ASTOpFalse], nil
	}
	if fid == 3 {
		f, k, e := m.declIdentity(decl)
		return f == 3 && k == 0, e
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astBitVectorPlugin {
		return m.isBitVectorNumeral(r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArithmeticPlugin {
		return m.arithmeticValue(fid, r, false)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astDatalogPlugin {
		family, kind, e := m.declIdentity(decl)
		return family == fid && kind == astDatalogConstantKind, e
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astFloatingPointPlugin {
		return m.floatingPointValue(fid, r, false)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astSequencePlugin {
		return m.sequenceValue(fid, r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astDatatypePlugin {
		return m.datatypeValue(r)
	}
	return false, nil
}
func (m *ASTManager) isValue(r astNodeRef) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	fid, _, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil {
		return false, e
	}
	if fid < 0 || !m.pluginAt(fid).present {
		return false, nil
	}
	return m.pluginValue(fid, r)
}
func (m *ASTManager) IsValue(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.isValue(r)
}

func (m *ASTManager) pluginUniqueValue(fid FamilyID, r astNodeRef) (bool, error) {
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astSequencePlugin {
		return false, nil
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArithmeticPlugin {
		return m.arithmeticValue(fid, r, true)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astFloatingPointPlugin {
		return m.floatingPointValue(fid, r, true)
	}
	return m.pluginValue(fid, r)
}

func (m *ASTManager) isUniqueValue(r astNodeRef) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	fid, _, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || fid < 0 || !m.pluginAt(fid).present {
		return false, e
	}
	return m.pluginUniqueValue(fid, r)
}
func (m *ASTManager) IsUniqueValue(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	fid, _, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || fid < 0 || !m.pluginAt(fid).present {
		return false, e
	}
	return m.pluginUniqueValue(fid, r)
}
func (m *ASTManager) AreEqual(a, b ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	x, e := m.internal(a)
	if e != nil {
		return false, e
	}
	y, e := m.internal(b)
	if e != nil {
		return false, e
	}
	if x == y {
		return true, nil
	}
	if m.nodeAt(x.slot).kind == ASTApp && m.nodeAt(y.slot).kind == ASTApp {
		fid, _, e := m.declIdentity(m.appAt(x.slot).decl)
		if e != nil {
			return false, e
		}
		if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArithmeticPlugin {
			equal, matched, e := m.arithmeticAlgebraicEqual(fid, x, y)
			if e != nil || matched {
				return equal, e
			}
		}
	}
	return false, nil
}
func (m *ASTManager) AreDistinct(a, b ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	x, e := m.internal(a)
	if e != nil {
		return false, e
	}
	y, e := m.internal(b)
	if e != nil {
		return false, e
	}
	if m.nodeAt(x.slot).kind != ASTApp || m.nodeAt(y.slot).kind != ASTApp {
		return false, nil
	}
	fid, _, e := m.declIdentity(m.appAt(x.slot).decl)
	if e != nil {
		return false, e
	}
	if fid < 0 || !m.pluginAt(fid).present {
		fid, _, e = m.declIdentity(m.appAt(y.slot).decl)
		if e != nil {
			return false, e
		}
	}
	if fid < 0 || !m.pluginAt(fid).present {
		return false, nil
	}
	if m.pluginAt(fid).kind == astBitVectorPlugin {
		return m.bitVectorOffsetDistinct(fid, x, y)
	}
	if m.pluginAt(fid).kind == astSequencePlugin {
		return m.sequenceDistinct(fid, x, y)
	}
	if x == y {
		return false, nil
	}
	if m.pluginAt(fid).kind == astArithmeticPlugin {
		equal, matched, e := m.arithmeticAlgebraicEqual(fid, x, y)
		if e != nil || matched {
			return !equal, e
		}
	}
	first, e := m.pluginUniqueValue(fid, x)
	if e != nil {
		return false, e
	}
	if first {
		second, e := m.pluginUniqueValue(fid, y)
		if e != nil || second {
			return second, e
		}
	}
	if m.pluginAt(fid).kind == astArithmeticPlugin {
		return m.arithmeticOffsetDistinct(fid, x, y)
	}
	return false, nil
}

// SomeValueProc is the borrowed source some_value_proc override. It receives
// the sort's stored hash because the manager is locked during the callback;
// callbacks must use borrowed scalar storage and must not reenter public manager
// methods. A null result delegates to the plugin and model-value fallback.
type SomeValueProc interface {
	SomeValue(sort ASTHandle, hash uint32) (ASTHandle, error)
}

func (m *ASTManager) GetSomeValue(sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.getSomeValue(r)
}

// GetSomeValueWithProc installs the source lexical override and restores its
// predecessor on all exits, including callback and allocation failures.
func (m *ASTManager) GetSomeValueWithProc(sort ASTHandle, proc SomeValueProc) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.getSomeValueWithProc(r, proc)
}
func (m *ASTManager) getSomeValueWithProc(r astNodeRef, proc SomeValueProc) (ASTHandle, error) {
	previous := m.someValueProc
	m.someValueProc = proc
	defer func() { m.someValueProc = previous }()
	return m.getSomeValue(r)
}
func (m *ASTManager) getSomeValue(r astNodeRef) (ASTHandle, error) {
	if m.someValueProc != nil {
		value, e := m.someValueProc.SomeValue(m.external(r), m.nodeAt(r.slot).hash)
		if e != nil {
			return ASTHandle{}, e
		}
		if !value.IsNull() {
			if _, e = m.internal(value); e != nil {
				return ASTHandle{}, e
			}
			return value, nil
		}
	}
	fid, kind, e := m.declIdentity(r)
	if e != nil {
		return ASTHandle{}, e
	}
	if fid == 0 && r == m.basic.boolSort {
		v, e := m.mkFamilyApp(0, ASTOpFalse, nil, nil)
		return m.external(v), e
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArrayPlugin {
		return m.arraySomeValue(r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astBitVectorPlugin {
		return m.bitVectorSomeValue(r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArithmeticPlugin {
		return m.arithmeticSomeValue(r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astFloatingPointPlugin {
		switch kind {
		case 0:
			value, e := m.floatingPointSomeValue(r)
			return m.external(value), e
		case 1:
			value, e := m.mkFamilyApp(fid, astFPARMTowardZero, nil, nil)
			return m.external(value), e
		}
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astSequencePlugin {
		return m.sequenceSomeValue(r)
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astDatatypePlugin {
		return m.datatypeSomeValue(r)
	}
	// Actual manager fallback constructs two lexical parameters; AST-valued
	// parameter construction doesn't retain until its declaration info attaches.
	index, e := m.newParameterInt(0)
	if e != nil {
		return ASTHandle{}, e
	}
	defer m.dropParameter(index)
	value, e := m.allocParameter()
	if e != nil {
		return ASTHandle{}, e
	}
	m.parameterAt(value.slot).Kind = ASTParameterAST
	m.parameterAt(value.slot).ast = r
	defer m.dropParameter(value)
	ps := [2]ASTParameter{index, value}
	v, e := m.mkFamilyApp(3, 0, ps[:], nil)
	return m.external(v), e
}
func (m *ASTManager) IsFullyInterpreted(sort ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(sort, ASTSort)
	if e != nil {
		return false, e
	}
	return m.isFullyInterpreted(r)
}
func (m *ASTManager) isFullyInterpreted(r astNodeRef) (bool, error) {
	fid, _, e := m.declIdentity(r)
	if e != nil {
		return false, e
	}
	if fid == NullFamilyID || fid == 4 {
		return false, nil
	}
	if fid >= 0 && m.pluginAt(fid).present && m.pluginAt(fid).kind == astArrayPlugin {
		return m.arrayFullyInterpreted(r)
	}
	return fid >= 0 && m.pluginAt(fid).present, nil
}
