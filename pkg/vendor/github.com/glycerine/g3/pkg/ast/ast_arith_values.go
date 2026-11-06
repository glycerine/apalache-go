package ast

// Source: src/ast/arith_decl_plugin.cpp:561-577,588-594,602-630.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (m *ASTManager) arithmeticValue(fid FamilyID, r astNodeRef, unique bool) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	f, kind, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || f != fid {
		return false, e
	}
	return kind == 0 || kind == 31 || kind == 32 || !unique && kind == 1, nil
}

func (m *ASTManager) arithmeticAlgebraicValue(fid FamilyID, r astNodeRef) (*polynomial.Algebraic, bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return nil, false, nil
	}
	f, kind, e := m.declIdentity(m.appAt(r.slot).decl)
	if e != nil || f != fid || kind != 1 {
		return nil, false, e
	}
	if m.arithmeticPlugin == nil || m.arithmeticPlugin.family != fid || m.arithmeticPlugin.algebraic == nil {
		return nil, false, ErrASTHandle
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		return nil, false, e
	}
	parameter, e := m.infoParameter(info, 0)
	if e != nil {
		return nil, false, e
	}
	external, e := m.parameterOfKind(parameter, ASTParameterExternal)
	if e != nil {
		return nil, false, e
	}
	v, e := m.arithmeticPlugin.algebraic.value(external.external)
	return v, e == nil, e
}

func (m *ASTManager) arithmeticAlgebraicEqual(fid FamilyID, a, b astNodeRef) (bool, bool, error) {
	x, matched, e := m.arithmeticAlgebraicValue(fid, a)
	if e != nil || !matched {
		return false, false, e
	}
	y, matched, e := m.arithmeticAlgebraicValue(fid, b)
	if e != nil || !matched {
		return false, false, e
	}
	equal, e := m.arithmeticPlugin.algebraic.manager.Equal(x, y)
	return equal, true, e
}

func (m *ASTManager) arithmeticSomeValue(sort astNodeRef) (ASTHandle, error) {
	p := m.arithmeticPlugin
	if sort != p.fields[0] && sort != p.fields[1] {
		return ASTHandle{}, ErrASTDomain
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.mkArithmeticNumeral(nil, sort == p.fields[1], true)
	return m.external(r), e
}

func (m *ASTManager) arithmeticOffsetCase(fid FamilyID, sum, base astNodeRef, index uint32) (bool, error) {
	f, kind, e := m.declIdentity(m.appAt(sum.slot).decl)
	if e != nil || f != fid || kind != 6 {
		return false, e
	}
	app := m.appAt(sum.slot)
	if app.count != 2 {
		return false, nil
	}
	argument, e := m.readInternal(app.args, index)
	if e != nil || argument != base {
		return false, e
	}
	offset, e := m.readInternal(app.args, 1-index)
	if e != nil {
		return false, e
	}
	if m.nodeAt(offset.slot).kind != ASTApp {
		return false, nil
	}
	f, kind, e = m.declIdentity(m.appAt(offset.slot).decl)
	if e != nil || f != fid || kind != 0 {
		return false, e
	}
	info, e := m.funcInfo(m.declAt(m.appAt(offset.slot).decl.slot).info)
	if e != nil {
		return false, e
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return false, e
	}
	value, e := m.parameter(p)
	if e != nil {
		return false, e
	}
	if value.Kind != ASTParameterRational {
		return false, ErrASTDomain
	}
	zero, e := m.numerals.IsZero(&value.Rational)
	return !zero, e
}

func (m *ASTManager) arithmeticOffsetDistinct(fid FamilyID, a, b astNodeRef) (bool, error) {
	// Preserve the original four branches and their short-circuit read order.
	if yes, e := m.arithmeticOffsetCase(fid, a, b, 0); e != nil || yes {
		return yes, e
	}
	if yes, e := m.arithmeticOffsetCase(fid, a, b, 1); e != nil || yes {
		return yes, e
	}
	if yes, e := m.arithmeticOffsetCase(fid, b, a, 1); e != nil || yes {
		return yes, e
	}
	return m.arithmeticOffsetCase(fid, b, a, 0)
}
