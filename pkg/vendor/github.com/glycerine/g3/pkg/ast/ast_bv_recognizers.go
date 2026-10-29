package ast

// Source: src/ast/bv_decl_plugin.cpp:793-830;
// src/ast/bv_decl_plugin.h:307-310; src/util/rational.h:462-464.

func (m *ASTManager) bitVectorAppKind(r astNodeRef, kind int32) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp || m.bitVectorPlugin == nil {
		return false, nil
	}
	fid, k, e := m.declIdentity(m.appAt(r.slot).decl)
	return fid == m.bitVectorPlugin.family && k == kind, e
}
func (m *ASTManager) bitVectorIntegerParameter(r astNodeRef, index uint32) (int32, error) {
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		return 0, e
	}
	p, e := m.infoParameter(info, index)
	if e != nil {
		return 0, e
	}
	v, e := m.parameter(p)
	if e != nil {
		return 0, e
	}
	if v.Kind != ASTParameterInt {
		return 0, ErrASTDomain
	}
	return v.integer, nil
}

func (m *ASTManager) IsBitVectorZero(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, 0)
	if e != nil || !found {
		return false, e
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
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
	return m.numerals.IsZero(&value.Rational)
}

func (m *ASTManager) IsBitVectorAllOne(h ASTHandle) (bool, error) { return m.isBitVectorAllOne(h, nil) }

// IsBitVectorAllOneWithInvocation names the actual copied-numeral/condition
// frame so an older caller can join a failed child before reference destruction.
func (m *ASTManager) IsBitVectorAllOneWithInvocation(h ASTHandle, c *ASTBitVectorNumeralInvocation) (bool, error) {
	if c == nil {
		return false, ErrASTHandle
	}
	return m.isBitVectorAllOne(h, c)
}
func (m *ASTManager) isBitVectorAllOne(h ASTHandle, c *ASTBitVectorNumeralInvocation) (all bool, result error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	var receipt *astBVNormalizationReceipt
	if c != nil {
		if e := c.prepare(m); e != nil {
			return false, e
		}
		defer c.finish(&result)
		receipt = &c.receipt
	}
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, 0)
	if e != nil || !found {
		return false, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return false, e
	}
	f.allOne = true
	if receipt != nil {
		*receipt = astBVNormalizationReceipt{frame: f, invocation: f.invocation}
	}
	defer f.finishNumeral(&result, receipt)
	all, e = m.bitVectorAllOneCondition(f, r)
	if e != nil {
		return false, e
	}
	// Full expression: returned subtraction value, rational(1), then power.
	for _, i := range []int{4, 2, 1} {
		if e = f.destroyOne(i, false); e != nil {
			return false, e
		}
	}
	if e = m.traceBitVectorAllOne(&f.values[0], all); e != nil {
		return false, e
	}
	return all, nil
}

// The common source arithmetic condition leaves its full-expression values
// alive. Its caller chooses the source full-expression destruction point.
func (m *ASTManager) bitVectorAllOneCondition(f *astBVNumeralFrame, r astNodeRef) (bool, error) {
	var e error
	q := m.numerals
	f.values[0], e = q.NewValue()
	if e != nil {
		return false, e
	}
	f.placed[0], f.complete[0] = true, true
	width := uint32(0)
	_, e = m.readBitVectorNumeralWithInvocation(r, &f.values[0], &width, &f.children[2])
	if e != nil {
		return false, e
	}
	f.values[1], e = q.PowerOfTwoWithInvocation(width, &f.children[0])
	if e != nil {
		return false, e
	}
	f.placed[1], f.complete[1] = true, true
	f.values[2], e = q.NewValue()
	if e != nil {
		return false, e
	}
	f.placed[2] = true
	if e = q.SetInt64WithInvocation(&f.values[2], 1, &f.children[1]); e != nil {
		return false, e
	}
	f.complete[2] = true
	// operator- returns a copied lvalue after subtracting from rational(lhs).
	if e = f.copyConstruct(3, &f.values[1]); e != nil {
		return false, e
	}
	if e = q.SubAssignWithInvocation(&f.values[3], &f.values[2], &f.children[4]); e != nil {
		return false, e
	}
	if e = f.copyConstruct(4, &f.values[3]); e != nil {
		return false, e
	}
	for _, i := range []int{6, 4, 5} {
		if e = f.children[i].SourceReady(); e != nil {
			return false, e
		}
	}
	if e = f.destroyOne(3, false); e != nil {
		return false, e
	}
	different, e := q.NotEqual(&f.values[0], &f.values[4])
	if e != nil {
		return false, e
	}
	return !different, nil
}

// Output handles borrow the original operand; recognition acquires no refs.
func (m *ASTManager) BitVectorExtractValue(h ASTHandle, low, high *uint32, base *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, 40)
	if e != nil || !found {
		return false, e
	}
	lo, e := m.bitVectorIntegerParameter(r, 1)
	if e != nil {
		return false, e
	}
	if low == nil {
		return false, ErrASTDomain
	}
	*low = uint32(lo)
	hi, e := m.bitVectorIntegerParameter(r, 0)
	if e != nil {
		return false, e
	}
	if high == nil {
		return false, ErrASTDomain
	}
	*high = uint32(hi)
	operand, e := m.readInternal(m.appAt(r.slot).args, 0)
	if e != nil {
		return false, e
	}
	if base == nil {
		return false, ErrASTDomain
	}
	*base = m.external(operand)
	return true, nil
}
func (m *ASTManager) BitVectorToIntValue(h ASTHandle, base *ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	found, e := m.bitVectorAppKind(r, 58)
	if e != nil || !found {
		return false, e
	}
	operand, e := m.readInternal(m.appAt(r.slot).args, 0)
	if e != nil {
		return false, e
	}
	if base == nil {
		return false, ErrASTDomain
	}
	*base = m.external(operand)
	return true, nil
}
