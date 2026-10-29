package ast

// Source: src/ast/bv_decl_plugin.h:387; src/util/rational.h:65.

func (m *ASTManager) BitVectorNumeralUint64(value uint64, bits uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	r, e := m.bitVectorNumeralUint64(value, bits)
	return m.external(r), e
}

func (m *ASTManager) bitVectorNumeralUint64(value uint64, bits uint32) (node astNodeRef, result error) {
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return node, e
	}
	f.unsignedNumeral = true
	defer f.finishNumeral(&result, nil)
	// The outer rational(uint64,ui64) temporary ends after the numeral call's
	// parameter temporary. Constructor failure retains an incomplete owner.
	f.values[5], e = m.numerals.NewValue()
	if e != nil {
		return node, e
	}
	f.placed[5] = true
	if e = m.numerals.SetUint64WithInvocation(&f.values[5], value, &f.children[9]); e != nil {
		return node, e
	}
	f.complete[5] = true
	if e = f.copyConstruct(0, &f.values[5]); e != nil {
		return node, e
	}
	return m.bitVectorNumeralAppCore(f, bits)
}
