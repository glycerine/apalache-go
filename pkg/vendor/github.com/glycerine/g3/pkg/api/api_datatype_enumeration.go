package api

// Source: src/api/api_datatype.cpp:88-155.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type APIEnumerationSort struct {
	Sort                   ast.ASTHandle
	Constants, Recognizers []ast.ASTHandle
}

func (c *SMTContext) EnumerationSort(name string, elements []string) (APIEnumerationSort, error) {
	symbol, err := c.StringSymbol(name)
	if err != nil {
		return APIEnumerationSort{}, err
	}
	names := make([]usymbol.Symbol, len(elements))
	for i, element := range elements {
		names[i], err = c.StringSymbol(element)
		if err != nil {
			return APIEnumerationSort{}, err
		}
	}
	return c.EnumerationSortWithSymbol(symbol, names)
}

func (c *SMTContext) EnumerationSortWithSymbol(name usymbol.Symbol, elements []usymbol.Symbol) (APIEnumerationSort, error) {
	if len(elements) == 0 {
		return APIEnumerationSort{}, ast.ErrASTDomain
	}
	m, err := c.datatypeManager()
	if err != nil {
		return APIEnumerationSort{}, err
	}
	constructors := make([]ast.DatatypeConstructorDefinition, len(elements))
	for i, element := range elements {
		spelling, err := m.Symbols.String(element)
		if err != nil {
			return APIEnumerationSort{}, err
		}
		recognizer, err := c.StringSymbol("is_" + spelling)
		if err != nil {
			return APIEnumerationSort{}, err
		}
		constructors[i] = ast.DatatypeConstructorDefinition{Name: element, Recognizer: recognizer}
	}
	sort, err := c.DatatypeSortWithSymbols(ast.DatatypeDefinition{Name: name, Constructors: constructors})
	if err != nil {
		return APIEnumerationSort{}, err
	}
	result := APIEnumerationSort{Sort: sort}
	for i := range elements {
		constructor, e := c.DatatypeConstructor(sort, uint32(i))
		if e != nil {
			return APIEnumerationSort{}, e
		}
		recognizer, e := c.DatatypeRecognizer(sort, uint32(i))
		if e != nil {
			return APIEnumerationSort{}, e
		}
		result.Constants = append(result.Constants, constructor)
		result.Recognizers = append(result.Recognizers, recognizer)
	}
	return result, nil
}
