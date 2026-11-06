package smt2

// Source: src/parsers/smt2/smt2parser.cpp:2619-2648,
// parse_assumptions, parse_check_sat and parse_check_sat_assuming.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *commandParser) checkSatAssumptions(parenthesized bool) ([]ast.ASTHandle, error) {
	if parenthesized {
		if err := p.expect(LeftParenToken); err != nil {
			return nil, fmt.Errorf("invalid check-sat-assuming command, '(' expected")
		}
	}
	var result []ast.ASTHandle
	for p.token != RightParenToken && p.token != EOFToken {
		value, _, err := p.expression()
		if err != nil {
			return nil, err
		}
		boolean, err := p.booleanExpression(value)
		if err != nil {
			return nil, err
		}
		if !boolean {
			return nil, fmt.Errorf("invalid check-sat command, argument must be a Boolean literal")
		}
		result = append(result, value)
	}
	if parenthesized {
		if p.token != RightParenToken {
			return nil, fmt.Errorf("invalid check-sat-assuming command, ')' expected")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *commandParser) booleanExpression(value ast.ASTHandle) (bool, error) {
	manager, err := p.astManager()
	if err != nil {
		return false, err
	}
	actual, err := manager.GetSort(value)
	if err != nil {
		return false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	return actual == boolean, nil
}
