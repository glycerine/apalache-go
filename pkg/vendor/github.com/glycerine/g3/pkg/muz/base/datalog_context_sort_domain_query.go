package base

import "github.com/glycerine/g3/pkg/ast"

func (c *DatalogContext) HasSortDomain(sort ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	_, found := c.sorts[sort]
	return found, nil
}

func (c *DatalogContext) SortDomainKind(sort ast.ASTHandle) (DatalogSortKind, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	domain := c.sorts[sort]
	if domain == nil {
		return 0, ErrDatalogContextOwner
	}
	return domain.kind, nil
}

// TrySortConstantCount ports context::try_get_sort_constant_count.
func (c *DatalogContext) TrySortConstantCount(sort ast.ASTHandle) (uint64, bool, error) {
	if err := c.open(); err != nil {
		return 0, false, err
	}
	domain := c.sorts[sort]
	if domain == nil {
		return 0, false, nil
	}
	if domain.kind == DatalogSortSymbol {
		return uint64(len(domain.symbolOrder)), true, nil
	}
	return uint64(len(domain.uintOrder)), true, nil
}
