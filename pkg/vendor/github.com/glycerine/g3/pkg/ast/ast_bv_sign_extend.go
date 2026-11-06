package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1532-1540;
// src/util/rational.h:131-134,198-200.

// ExtendBitVectorNumeralSign owns the local rational and each source full
// expression through publication. Symbolic input does not access out.
func (m *ASTManager) ExtendBitVectorNumeralSign(n uint32, arg ASTHandle, out ASTRef) (matched bool, result error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(arg)
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
		return true, e
	}
	f.signExtension = true
	defer f.finishNumeral(&result, nil)
	q := m.numerals
	f.values[0], e = q.NewValue()
	if e != nil {
		return true, e
	}
	f.placed[0], f.complete[0] = true, true
	var width uint32
	_, e = m.readBitVectorNumeralWithInvocation(r, &f.values[0], &width, &f.children[2])
	if e != nil {
		return true, e
	}
	// Preserve the source unsigned addition, including width wraparound.
	resultWidth := width + n
	f.values[1], e = m.normalizeBitVectorWithFrame(&f.values[0], width, true, nil, &f.normalizationChild)
	if e != nil {
		return true, e
	}
	f.placed[1], f.complete[1] = true, true
	// rational has copy assignment, not move assignment, for this expression.
	if e = q.CopyWithInvocation(&f.values[0], &f.values[1], &f.children[3]); e != nil {
		return true, e
	}
	if e = f.children[3].SourceReady(); e != nil {
		return true, e
	}
	if e = f.destroyOne(1, false); e != nil {
		return true, e
	}
	f.values[2], e = q.PowerOfTwoWithInvocation(resultWidth, &f.children[0])
	if e != nil {
		return true, e
	}
	f.placed[2], f.complete[2] = true, true
	if e = q.ModIntoWithInvocation(&f.values[0], &f.values[2], &f.values[0], &f.children[1]); e != nil {
		return true, e
	}
	for _, i := range []int{1, 0} {
		if e = f.children[i].SourceReady(); e != nil {
			return true, e
		}
	}
	if e = f.destroyOne(2, false); e != nil {
		return true, e
	}
	// The normalization child retired before its returned value was assigned.
	// Only now can its receipt be replaced by the next nested constructor.
	node, e := m.mkBitVectorNumeralCaptured(&f.values[0], resultWidth, &f.normalizationChild)
	if e != nil {
		return true, e
	}
	return true, m.assignRef(out, node)
}
