package api

// Source: src/api/api_ast.cpp:46-70,268-304.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type APISymbolKind uint8

const (
	APISymbolInteger APISymbolKind = iota
	APISymbolString
)

func (c *SMTContext) SignedIntegerSymbol(value int32) (usymbol.Symbol, error) {
	if _, err := c.Manager(); err != nil {
		return usymbol.Symbol{}, err
	}
	if value < 0 {
		return usymbol.Symbol{}, ast.ErrASTDomain
	}
	return usymbol.Numerical(uint32(value)), nil
}

func (c *SMTContext) SymbolKind(symbol usymbol.Symbol) (APISymbolKind, error) {
	m, err := c.Manager()
	if err != nil {
		return APISymbolInteger, err
	}
	if symbol.IsNumerical() {
		if _, err = symbol.Number(); err != nil {
			return APISymbolInteger, err
		}
		return APISymbolInteger, nil
	}
	if _, err = m.Symbols.Display(symbol); err != nil {
		return APISymbolInteger, err
	}
	return APISymbolString, nil
}

func (c *SMTContext) SymbolInteger(symbol usymbol.Symbol) (int32, error) {
	if _, err := c.Manager(); err != nil {
		return 0, err
	}
	if !symbol.IsNumerical() {
		return 0, ast.ErrASTDomain
	}
	value, err := symbol.Number()
	if err != nil {
		return 0, err
	}
	// get_num is unsigned but the source API returns int, preserving the
	// selected native two's-complement narrowing for generated numeric names.
	return int32(value), nil
}
