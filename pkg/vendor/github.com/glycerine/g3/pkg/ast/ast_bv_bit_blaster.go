package ast

// Source: src/ast/bv_decl_plugin.h:403-405, 437-468;
// src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:147-159, 183-187.

func (m *ASTManager) MkBitVectorBit2Bool(index uint32, arg ASTHandle) (ASTHandle, error) {
	values := [1]astParameterRecord{{Kind: ASTParameterInt, integer: int32(index)}}
	return m.bitVectorIntegerParameterApp(55, values[:], arg)
}

func (m *ASTManager) MkBitVectorFromBits(bits []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.mkFamilyApp(m.bitVectorPlugin.family, 56, nil, bits)
	return m.external(r), e
}
