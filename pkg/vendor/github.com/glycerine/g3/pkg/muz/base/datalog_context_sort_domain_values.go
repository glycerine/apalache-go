package base

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) SymbolConstants(sort ast.ASTHandle) ([]usymbol.Symbol, error) {
	domain, err := c.sortDomain(sort, DatalogSortSymbol)
	if err != nil {
		return nil, err
	}
	return append([]usymbol.Symbol(nil), domain.symbolOrder...), nil
}

func (c *DatalogContext) Uint64Constants(sort ast.ASTHandle) ([]uint64, error) {
	domain, err := c.sortDomain(sort, DatalogSortUint64)
	if err != nil {
		return nil, err
	}
	return append([]uint64(nil), domain.uintOrder...), nil
}

func (c *DatalogContext) SymbolConstant(sort ast.ASTHandle, number uint64) (usymbol.Symbol, error) {
	domain, err := c.sortDomain(sort, DatalogSortSymbol)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if number >= uint64(len(domain.symbolOrder)) {
		return usymbol.Symbol{}, fmt.Errorf("muz/base: symbol constant number out of range")
	}
	return domain.symbolOrder[number], nil
}

func (c *DatalogContext) Uint64Constant(sort ast.ASTHandle, number uint64) (uint64, error) {
	domain, err := c.sortDomain(sort, DatalogSortUint64)
	if err != nil {
		return 0, err
	}
	if number >= uint64(len(domain.uintOrder)) {
		return 0, fmt.Errorf("muz/base: uint64 constant number out of range")
	}
	return domain.uintOrder[number], nil
}
