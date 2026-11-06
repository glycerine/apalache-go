package base

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type DatalogSortDomainSnapshot struct {
	Kind         DatalogSortKind
	Finite       bool
	DeclaredSize uint64
	Symbols      []usymbol.Symbol
	Uint64s      []uint64
}

func (c *DatalogContext) SortDomainSnapshot(sort ast.ASTHandle) (DatalogSortDomainSnapshot, error) {
	if err := c.open(); err != nil {
		return DatalogSortDomainSnapshot{}, err
	}
	domain := c.sorts[sort]
	if domain == nil {
		return DatalogSortDomainSnapshot{}, ErrDatalogContextOwner
	}
	return DatalogSortDomainSnapshot{
		Kind:         domain.kind,
		Finite:       domain.finite,
		DeclaredSize: domain.limit,
		Symbols:      append([]usymbol.Symbol(nil), domain.symbolOrder...),
		Uint64s:      append([]uint64(nil), domain.uintOrder...),
	}, nil
}
