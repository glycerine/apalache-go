package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_indexed_identifier_core and
// parse_func_decl_ref. Retained applications include array map and as-array.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type identifierIndex struct {
	number   uint32
	function ast.ASTHandle
}

func (p *commandParser) indexedExpression() (string, []identifierIndex, error) {
	name, err := p.symbol()
	if err != nil {
		return "", nil, err
	}
	var indices []identifierIndex
	for p.token != RightParenToken {
		var index identifierIndex
		switch p.token {
		case IntegerToken:
			index.number, err = p.unsigned()
		case SymbolToken, LeftParenToken:
			index.function, err = p.functionDeclarationReference()
		default:
			err = fmt.Errorf("invalid indexed identifier, integer, identifier or '(' expected")
		}
		if err != nil {
			return "", nil, err
		}
		indices = append(indices, index)
	}
	if len(indices) == 0 {
		return "", nil, fmt.Errorf("invalid indexed identifier, index expected")
	}
	return name, indices, p.advance()
}

func (p *commandParser) functionDeclarationReference() (ast.ASTHandle, error) {
	if p.token == SymbolToken {
		name, err := p.symbol()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.pin(p.context.FindFunctionDeclaration(name, nil, nil, ast.ASTHandle{}))
	}
	if err := p.expect(LeftParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	var name string
	var indices []uint32
	var err error
	if p.token == SymbolToken {
		name, err = p.symbol()
	} else {
		if err = p.expect(LeftParenToken); err != nil {
			return ast.ASTHandle{}, err
		}
		var underscore string
		underscore, err = p.symbol()
		if err == nil && underscore != "_" {
			err = fmt.Errorf("invalid indexed function declaration reference, '_' expected")
		}
		if err == nil {
			name, indices, err = p.indexed()
		}
		if err == nil && len(indices) == 0 {
			err = fmt.Errorf("invalid indexed function declaration reference, index expected")
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.expect(LeftParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0)
	for p.token != RightParenToken {
		sort, err := p.sort()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		domain = append(domain, sort)
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	rangeSort, err := p.sort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pin(p.context.FindFunctionDeclaration(name, indices, domain, rangeSort))
}
