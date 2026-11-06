package util

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type CostParser struct {
	*SimpleParser
	variables []ast.ASTRef
	real      ast.ASTHandle
}

func NewCostParser(manager *ast.ASTManager) (*CostParser, error) {
	if manager == nil {
		return nil, fmt.Errorf("parser: nil AST manager")
	}
	if _, err := manager.RegisterArithmeticPlugin(); err != nil {
		return nil, err
	}
	parser, err := NewSimpleParser(manager)
	if err != nil {
		return nil, err
	}
	result := &CostParser{SimpleParser: parser}
	for name, kind := range map[string]int32{
		"true": ast.ASTOpTrue, "false": ast.ASTOpFalse, "not": ast.ASTOpNot,
		"and": ast.ASTOpAnd, "implies": ast.ASTOpImplies, "or": ast.ASTOpOr,
		"ite": ast.ASTOpIte, "=": ast.ASTOpEq, "iff": ast.ASTOpIff, "xor": ast.ASTOpXor,
	} {
		if err = parser.AddBuiltin(name, manager.BasicFamilyID(), kind); err != nil {
			return nil, err
		}
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	for name, kind := range map[string]int32{"<=": 2, ">=": 3, "<": 4, ">": 5, "+": 6, "-": 7, "*": 9, "/": 10} {
		if err = parser.AddBuiltin(name, arithmetic, kind); err != nil {
			return nil, err
		}
	}
	result.real, err = manager.MkFamilySort(arithmetic, 0, nil)
	if err != nil {
		return nil, err
	}
	parser.SetNumberParsers(func(value *big.Rat) (ast.ASTHandle, error) {
		return manager.ArithmeticNumeralRat(value, false)
	}, func(value *big.Rat) (ast.ASTHandle, error) {
		return manager.ArithmeticNumeralRat(value, false)
	})
	return result, nil
}

func (p *CostParser) AddCostVariable(name string) (uint32, error) {
	index := uint32(len(p.variables))
	variable, err := p.manager.MkVar(index, p.real)
	if err != nil {
		return 0, err
	}
	ref, err := p.manager.NewRef(variable)
	if err != nil {
		return 0, err
	}
	if err = p.SimpleParser.AddVariable(name, variable); err != nil {
		_ = p.manager.DelRef(&ref)
		return 0, err
	}
	p.variables = append(p.variables, ref)
	return index, nil
}

func (p *CostParser) ResetCostVariables() error {
	p.SimpleParser.ResetVariables()
	for i := len(p.variables) - 1; i >= 0; i-- {
		if err := p.manager.DelRef(&p.variables[i]); err != nil {
			return err
		}
	}
	p.variables = nil
	return nil
}

func (p *CostParser) Close() error { return p.ResetCostVariables() }
