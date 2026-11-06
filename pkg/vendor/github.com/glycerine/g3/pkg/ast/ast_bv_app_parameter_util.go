package ast

// Source: src/ast/bv_decl_plugin.h:399-402,416-423;
// src/ast/bv_decl_plugin.cpp:880-885; src/ast/ast.h:113-122.

import "math"

// Scalar source stack parameters have no owned destructor state. Keeping their
// records separate from declaration-info scratch permits nested factory calls.
// The cold manager accounts for these two records; calls allocate no arena slot.
type astBVUtilityParameters struct {
	values [2]astParameterRecord
	epoch  uint32
	count  uint32
	active bool
}

func (m *ASTManager) withBitVectorUtilityParameters(kind int32, values []astParameterRecord, arg ASTHandle) (ASTHandle, error) {
	f := &m.bitVectorUtilityParameters
	if f.active || f.epoch == math.MaxUint32/2 {
		return ASTHandle{}, ErrASTPending
	}
	if len(values) == 0 || len(values) > 2 {
		return ASTHandle{}, ErrASTDomain
	}
	f.epoch++
	f.active = true
	f.count = uint32(len(values))
	defer func() { f.active = false; f.count = 0; f.values = [2]astParameterRecord{} }()
	var params [2]ASTParameter
	for i, value := range values {
		f.values[i] = value
		f.values[i].Live = true
		params[i] = ASTParameter{m.origin, math.MaxUint32 - 2, f.epoch*2 + uint32(i)}
	}
	r, e := m.mkFamilyApp(m.bitVectorPlugin.family, kind, params[:len(values)], []ASTHandle{arg})
	return m.external(r), e
}

func (m *ASTManager) bitVectorIntegerParameterApp(kind int32, values []astParameterRecord, arg ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	return m.withBitVectorUtilityParameters(kind, values, arg)
}

func (m *ASTManager) MkBitVectorExtract(high, low uint32, arg ASTHandle) (ASTHandle, error) {
	values := [2]astParameterRecord{{Kind: ASTParameterInt, integer: int32(high)}, {Kind: ASTParameterInt, integer: int32(low)}}
	return m.bitVectorIntegerParameterApp(40, values[:], arg)
}

func (m *ASTManager) MkBitVectorZeroExtend(n uint32, arg ASTHandle) (ASTHandle, error) {
	values := [1]astParameterRecord{{Kind: ASTParameterInt, integer: int32(n)}}
	return m.bitVectorIntegerParameterApp(39, values[:], arg)
}

func (m *ASTManager) MkBitVectorSignExtend(n uint32, arg ASTHandle) (ASTHandle, error) {
	values := [1]astParameterRecord{{Kind: ASTParameterInt, integer: int32(n)}}
	return m.bitVectorIntegerParameterApp(38, values[:], arg)
}

func (m *ASTManager) MkIntToBitVector(width uint32, arg ASTHandle) (ASTHandle, error) {
	values := [1]astParameterRecord{{Kind: ASTParameterInt, integer: int32(width)}}
	return m.bitVectorIntegerParameterApp(57, values[:], arg)
}

func (m *ASTManager) MkBitVectorToInt(arg ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if m.bitVectorPlugin == nil {
		return ASTHandle{}, ErrASTDomain
	}
	name, e := m.Symbols.Intern("arith")
	if e != nil {
		return ASTHandle{}, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return ASTHandle{}, e
	}
	sort, e := m.mkFamilySort(fid, 1, nil)
	if e != nil {
		return ASTHandle{}, e
	}
	var borrowed astNodeRef
	if !sort.IsNull() {
		borrowed, e = m.internal(sort)
		if e != nil {
			return ASTHandle{}, e
		}
	}
	// parameter(AST*) is a scalar borrow. It does not INC the sort.
	values := [1]astParameterRecord{{Kind: ASTParameterAST, ast: borrowed}}
	return m.withBitVectorUtilityParameters(58, values[:], arg)
}
