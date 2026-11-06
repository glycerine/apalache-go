package base

import (
	"fmt"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type DatalogSortKind uint8

const (
	DatalogSortSymbol DatalogSortKind = iota
	DatalogSortUint64
)

type datalogSortDomain struct {
	kind        DatalogSortKind
	limit       uint64
	finite      bool
	symbols     map[usymbol.Symbol]uint64
	symbolOrder []usymbol.Symbol
	uints       map[uint64]uint64
	uintOrder   []uint64
}

func (c *DatalogContext) RegisterFiniteSort(sort ast.ASTHandle, kind DatalogSortKind) error {
	if err := c.open(); err != nil {
		return err
	}
	if kind != DatalogSortSymbol && kind != DatalogSortUint64 {
		return fmt.Errorf("muz/base: unsupported datalog sort domain")
	}
	if _, exists := c.sorts[sort]; exists {
		return fmt.Errorf("muz/base: sort domain is already registered")
	}
	size, err := c.manager.GetSortSize(sort)
	if err != nil {
		return err
	}
	d := &datalogSortDomain{kind: kind, symbols: make(map[usymbol.Symbol]uint64), uints: make(map[uint64]uint64), finite: size.IsFinite()}
	if d.finite {
		d.limit, err = size.Size()
		if err != nil {
			return err
		}
	}
	if err = c.retain(sort); err != nil {
		return err
	}
	c.sorts[sort] = d
	return nil
}

func (c *DatalogContext) sortDomain(sort ast.ASTHandle, kind DatalogSortKind) (*datalogSortDomain, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	d := c.sorts[sort]
	if d == nil || d.kind != kind {
		return nil, fmt.Errorf("muz/base: sort has no matching datalog domain")
	}
	return d, nil
}
func (c *DatalogContext) checkDomainLimit(sort ast.ASTHandle, d *datalogSortDomain, n uint64) error {
	if d.finite && n >= d.limit {
		name, err := c.manager.GetASTName(sort)
		if err != nil {
			return err
		}
		text, err := c.manager.Symbols.Display(name)
		if err != nil {
			return err
		}
		return fmt.Errorf("sort %s contains more constants than its declared size %d", text, d.limit)
	}
	return nil
}

func (c *DatalogContext) SymbolNumber(sort ast.ASTHandle, value usymbol.Symbol) (uint64, error) {
	d, err := c.sortDomain(sort, DatalogSortSymbol)
	if err != nil {
		return 0, err
	}
	n, ok := d.symbols[value]
	if !ok {
		n = uint64(len(d.symbolOrder))
		d.symbols[value] = n
		d.symbolOrder = append(d.symbolOrder, value)
	}
	if err = c.checkDomainLimit(sort, d, n); err != nil {
		return 0, err
	}
	return n, nil
}
func (c *DatalogContext) Uint64Number(sort ast.ASTHandle, value uint64) (uint64, error) {
	d, err := c.sortDomain(sort, DatalogSortUint64)
	if err != nil {
		return 0, err
	}
	n, ok := d.uints[value]
	if !ok {
		n = uint64(len(d.uintOrder))
		d.uints[value] = n
		d.uintOrder = append(d.uintOrder, value)
	}
	if err = c.checkDomainLimit(sort, d, n); err != nil {
		return 0, err
	}
	return n, nil
}
func (c *DatalogContext) ConstantCount(sort ast.ASTHandle) (uint64, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	d := c.sorts[sort]
	if d == nil {
		return 0, fmt.Errorf("muz/base: sort domain is not registered")
	}
	if d.kind == DatalogSortSymbol {
		return uint64(len(d.symbolOrder)), nil
	}
	return uint64(len(d.uintOrder)), nil
}
func (c *DatalogContext) SortSizeEstimate(sort ast.ASTHandle) (uint64, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	ruleSort, err := c.manager.IsDatalogRuleSort(sort)
	if err != nil {
		return 0, err
	}
	if ruleSort {
		return 1, nil
	}
	d := c.sorts[sort]
	if d != nil {
		return c.ConstantCount(sort)
	}
	size, err := c.manager.GetSortSize(sort)
	if err != nil {
		return 0, err
	}
	if size.IsFinite() {
		return size.Size()
	}
	return math.MaxUint64, nil
}
func (c *DatalogContext) FormatConstant(sort ast.ASTHandle, number uint64) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	d := c.sorts[sort]
	if d == nil {
		return "", fmt.Errorf("muz/base: sort domain is not registered")
	}
	if d.kind == DatalogSortSymbol && number < uint64(len(d.symbolOrder)) {
		return c.manager.Symbols.Display(d.symbolOrder[number])
	}
	if d.kind == DatalogSortUint64 && number < uint64(len(d.uintOrder)) {
		return strconv.FormatUint(d.uintOrder[number], 10), nil
	}
	name, err := c.manager.GetASTName(sort)
	if err != nil {
		return "", err
	}
	text, err := c.manager.Symbols.Display(name)
	if err != nil {
		return "", err
	}
	if d.kind == DatalogSortSymbol {
		return strconv.FormatUint(number, 10), nil
	}
	return "<unk " + text + ":" + strconv.FormatUint(number, 10) + ">", nil
}
