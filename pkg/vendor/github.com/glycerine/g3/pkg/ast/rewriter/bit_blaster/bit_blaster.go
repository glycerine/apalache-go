package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster.h:52-61;
// src/ast/rewriter/bit_blaster/bit_blaster.cpp:118-128.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// BitBlaster is the concrete bit_blaster_tpl<bit_blaster_cfg>
// specialization. It borrows the AST manager and retains the source parameter
// snapshot by value.
type BitBlaster struct {
	*Engine
	params SMTBitBlasterParams
}

func NewBitBlaster(m *ast.ASTManager, params SMTBitBlasterParams) (*BitBlaster, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	name, err := m.Symbols.Intern("bv")
	if err != nil {
		return nil, err
	}
	family, err := m.FamilyIDForSymbol(name)
	if err != nil {
		return nil, err
	}
	config, err := NewConfig(m, family, params)
	if err != nil {
		return nil, err
	}
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}
	return &BitBlaster{Engine: engine, params: params}, nil
}

func (b *BitBlaster) Params() SMTBitBlasterParams {
	if b == nil {
		return SMTBitBlasterParams{}
	}
	return b.params
}
