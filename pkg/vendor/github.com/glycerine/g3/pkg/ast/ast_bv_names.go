package ast

// Source: src/ast/bv_decl_plugin.cpp:672-743.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

func (m *ASTManager) GetBitVectorSortNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.bitVectorPlugin == nil {
		return ErrASTHandle
	}
	all, e := m.builtinAllLogic(logic)
	if e != nil {
		return e
	}
	if all {
		if e := m.appendBuiltinName(names, "bv", 0); e != nil {
			return e
		}
	}
	return m.appendBuiltinName(names, "BitVec", 0)
}

func (m *ASTManager) GetBitVectorOpNames(names *ASTBuiltinNames, logic usymbol.Symbol) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if names == nil || m.bitVectorPlugin == nil {
		return ErrASTHandle
	}
	entries := [...]struct {
		text string
		kind int32
	}{
		{"bit1", 1},
		{"bit0", 2},
		{"bvneg", 3},
		{"bvadd", 4},
		{"bvsub", 5},
		{"bvmul", 6},
		{"bvsdiv", 7},
		{"bvudiv", 8},
		{"bvsrem", 9},
		{"bvurem", 10},
		{"bvsmod", 11},
		{"bvule", 22},
		{"bvsle", 23},
		{"bvuge", 24},
		{"bvsge", 25},
		{"bvult", 26},
		{"bvslt", 27},
		{"bvugt", 28},
		{"bvsgt", 29},
		{"bvand", 30},
		{"bvor", 31},
		{"bvnot", 32},
		{"bvxor", 33},
		{"bvnand", 34},
		{"bvnor", 35},
		{"bvxnor", 36},
		{"concat", 37},
		{"sign_extend", 38},
		{"zero_extend", 39},
		{"extract", 40},
		{"repeat", 41},
		{"bvredor", 42},
		{"bvredand", 43},
		{"bvcomp", 44},
		{"bvshl", 45},
		{"bvlshr", 46},
		{"bvashr", 47},
		{"rotate_left", 48},
		{"rotate_right", 49},
		{"bvumul_noovfl", 52},
		{"bvsmul_noovfl", 53},
		{"bvsmul_noudfl", 54},
		{"bvsdiv0", 12},
		{"bvudiv0", 13},
		{"bvsrem0", 14},
		{"bvurem0", 15},
		{"bvsmod0", 16},
		{"bvsdiv_i", 17},
		{"bvudiv_i", 18},
		{"bvsrem_i", 19},
		{"bvurem_i", 20},
		{"bvsmod_i", 21},
		{"ext_rotate_left", 50},
		{"ext_rotate_right", 51},
		{"int2bv", 57},
		{"bv2int", 58},
		{"bv2nat", 58},
		{"mkbv", 56},
	}
	for _, entry := range entries[:39] {
		if e := m.appendBuiltinName(names, entry.text, entry.kind); e != nil {
			return e
		}
	}
	all, e := m.builtinAllLogic(logic)
	if e != nil || !all {
		return e
	}
	for _, entry := range entries[39:] {
		if e := m.appendBuiltinName(names, entry.text, entry.kind); e != nil {
			return e
		}
	}
	return nil
}
