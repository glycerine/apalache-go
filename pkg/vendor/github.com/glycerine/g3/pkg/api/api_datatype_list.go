package api

// Source: src/api/api_datatype.cpp:157-218.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type APIListSort struct {
	Sort, Nil, IsNil, Cons, IsCons, Head, Tail ast.ASTHandle
}

func (c *SMTContext) ListSort(name string, elementSort ast.ASTHandle) (APIListSort, error) {
	symbol, err := c.StringSymbol(name)
	if err != nil {
		return APIListSort{}, err
	}
	return c.ListSortWithSymbol(symbol, elementSort)
}

func (c *SMTContext) ListSortWithSymbol(name usymbol.Symbol, elementSort ast.ASTHandle) (APIListSort, error) {
	definition := APIDatatypeDefinition{

		Constructors: []APIDatatypeConstructor{
			{Name: "nil", Recognizer: "is_nil"},
			{Name: "cons", Recognizer: "is_cons", Accessors: []APIDatatypeAccessor{
				{Name: "head", Sort: elementSort},
				{Name: "tail", Recursive: true},
			}},
		},
	}
	converted := ast.DatatypeDefinition{Name: name}
	for _, constructor := range definition.Constructors {
		native, err := c.datatypeConstructorSymbols(constructor)
		if err != nil {
			return APIListSort{}, err
		}
		converted.Constructors = append(converted.Constructors, native)
	}
	sort, err := c.DatatypeSortWithSymbols(converted)
	if err != nil {
		return APIListSort{}, err
	}
	nilDecl, err := c.DatatypeConstructor(sort, 0)
	if err != nil {
		return APIListSort{}, err
	}
	isNil, err := c.DatatypeRecognizer(sort, 0)
	if err != nil {
		return APIListSort{}, err
	}
	cons, err := c.DatatypeConstructor(sort, 1)
	if err != nil {
		return APIListSort{}, err
	}
	isCons, err := c.DatatypeRecognizer(sort, 1)
	if err != nil {
		return APIListSort{}, err
	}
	fields, err := c.datatypeConstructorAccessors(cons)
	if err != nil {
		return APIListSort{}, err
	}
	if len(fields) != 2 {
		return APIListSort{}, ast.ErrASTDomain
	}
	return APIListSort{Sort: sort, Nil: nilDecl, IsNil: isNil, Cons: cons, IsCons: isCons, Head: fields[0], Tail: fields[1]}, nil
}
