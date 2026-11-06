package api

// Source: src/api/api_datatype.cpp:26-86.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type APITupleSort struct {
	Sort, Constructor ast.ASTHandle
	Fields            []ast.ASTHandle
}

func (c *SMTContext) TupleSort(name string, fieldNames []string, fieldSorts []ast.ASTHandle) (APITupleSort, error) {
	symbol, err := c.StringSymbol(name)
	if err != nil {
		return APITupleSort{}, err
	}
	names := make([]usymbol.Symbol, len(fieldNames))
	for i, field := range fieldNames {
		names[i], err = c.StringSymbol(field)
		if err != nil {
			return APITupleSort{}, err
		}
	}
	return c.TupleSortWithSymbol(symbol, names, fieldSorts)
}

func (c *SMTContext) TupleSortWithSymbol(name usymbol.Symbol, fieldNames []usymbol.Symbol, fieldSorts []ast.ASTHandle) (APITupleSort, error) {
	if len(fieldNames) != len(fieldSorts) {
		return APITupleSort{}, ast.ErrASTDomain
	}
	m, err := c.datatypeManager()
	if err != nil {
		return APITupleSort{}, err
	}
	spelling, err := m.Symbols.String(name)
	if err != nil {
		return APITupleSort{}, err
	}
	recognizer, err := c.StringSymbol("is_" + spelling)
	if err != nil {
		return APITupleSort{}, err
	}
	accessors := make([]ast.DatatypeAccessorDefinition, len(fieldNames))
	for i := range fieldNames {
		accessors[i] = ast.DatatypeAccessorDefinition{Name: fieldNames[i], Range: fieldSorts[i]}
	}
	sort, err := c.DatatypeSortWithSymbols(ast.DatatypeDefinition{Name: name, Constructors: []ast.DatatypeConstructorDefinition{{Name: name, Recognizer: recognizer, Accessors: accessors}}})
	if err != nil {
		return APITupleSort{}, err
	}
	constructor, err := c.DatatypeConstructor(sort, 0)
	if err != nil {
		return APITupleSort{}, err
	}
	fields, err := c.datatypeConstructorAccessors(constructor)
	if err != nil {
		return APITupleSort{}, err
	}
	return APITupleSort{Sort: sort, Constructor: constructor, Fields: fields}, nil
}
