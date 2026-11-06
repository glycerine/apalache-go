package cmd_context

// Source: dparser::parse_arg, parse_infix, mk_const and mk_symbol_const.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseArgument(sort ast.ASTHandle) (ast.ASTHandle, error) {
	token := p.current
	var result ast.ASTHandle
	var err error
	switch token.kind {
	case dlWildcard:
		result, err = p.manager.MkVar(p.nextVar, sort)
		p.nextVar++
	case dlIdentifier:
		if prior, found := p.variables[token.text]; found {
			priorSort, e := p.manager.GetSort(prior)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if priorSort != sort {
				return ast.ASTHandle{}, fmt.Errorf("datalog:%d: variable %s used with inconsistent sorts", token.line, token.text)
			}
			result = prior
		} else {
			result, err = p.manager.MkVar(p.nextVar, sort)
			if err == nil {
				p.variables[token.text], p.nextVar = result, p.nextVar+1
			}
		}
	case dlString:
		result, err = p.symbolicConstant(token.text, sort)
	case dlNumber:
		// Source parse_arg converts the token to uint64 before mk_symbol_const
		// maps its canonical decimal spelling into a finite-domain symbol.
		result, err = p.numericConstant(token.text, sort)
	default:
		return ast.ASTHandle{}, p.expected("argument")
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = p.pin(result)
	if err == nil {
		err = p.advance()
	}
	return result, err
}

func (p *datalogParser) numericConstant(spelling string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	// Source rational parsing accepts signed zero as an unsigned value.
	if len(spelling) > 1 && spelling[0] == '-' && strings.Trim(spelling[1:], "0") == "" {
		spelling = "0"
	}
	value, err := strconv.ParseUint(spelling, 10, 64)
	if err != nil {
		return ast.ASTHandle{}, fmt.Errorf("invalid unsigned integer %q", spelling)
	}
	return p.symbolicConstant(strconv.FormatUint(value, 10), sort)
}

func (p *datalogParser) symbolicConstant(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	finite, err := p.manager.IsDatalogFiniteSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !finite {
		value, parseErr := strconv.ParseUint(name, 10, 64)
		if parseErr != nil {
			return ast.ASTHandle{}, fmt.Errorf("invalid integer %q", name)
		}
		return p.manager.MkDatalogNumeral(value, sort)
	}
	value, err := p.symbolicConstantNumber(name, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.manager.MkDatalogNumeral(value, sort)
}

// symbolicConstantNumber is shared by source rule constants and WPA map names.
// Both consume context::get_constant_number's first-use symbolic numbering.
func (p *datalogParser) symbolicConstantNumber(name string, sort ast.ASTHandle) (uint64, error) {
	if _, numeric := p.numericConstants[sort]; numeric {
		return 0, fmt.Errorf("datalog: symbolic rule constant %q requires a symbolic domain", name)
	}
	table := p.constants[sort]
	if table == nil {
		table = make(map[string]uint64)
		p.constants[sort] = table
	}
	value, found := table[name]
	if !found {
		value = uint64(len(table))
		table[name] = value
	}
	size, _, err := p.manager.DatalogFiniteSortSize(sort)
	if err != nil {
		return 0, err
	}
	if value >= size {
		return 0, fmt.Errorf("constant %q exceeds finite domain size %d", name, size)
	}
	return value, nil
}

func (p *datalogParser) parseInfix() (ast.ASTHandle, error) {
	leftToken := p.current
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	op := p.current.kind
	if op != dlEqual && op != dlNotEqual && op != dlLess && op != dlGreater {
		return ast.ASTHandle{}, p.expected("infix operator")
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	rightToken := p.current
	if rightToken.kind != dlIdentifier && rightToken.kind != dlString && rightToken.kind != dlNumber {
		return ast.ASTHandle{}, p.expected("infix operand")
	}
	leftVar, leftOK := p.variables[leftToken.text]
	rightVar, rightOK := p.variables[rightToken.text]
	leftOK = leftOK && leftToken.kind == dlIdentifier
	rightOK = rightOK && rightToken.kind == dlIdentifier
	if rightToken.kind == dlIdentifier && !rightOK {
		return ast.ASTHandle{}, p.expected("previously typed variable")
	}
	if !leftOK && !rightOK {
		return ast.ASTHandle{}, fmt.Errorf("datalog:%d: at least one infix argument must be a variable", leftToken.line)
	}
	sort, err := p.manager.GetSort(leftVar)
	if !leftOK {
		sort, err = p.manager.GetSort(rightVar)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	operand := func(token datalogLexeme, variable ast.ASTHandle, found bool) (ast.ASTHandle, error) {
		if found {
			return variable, nil
		}
		// Unlike predicate arguments, source parse_infix sends both numeric
		// and quoted tokens directly to mk_const, preserving their spelling.
		return p.symbolicConstant(token.text, sort)
	}
	left, err := operand(leftToken, leftVar, leftOK)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := operand(rightToken, rightVar, rightOK)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	switch op {
	case dlEqual:
		result, err = p.manager.MkEq(left, right)
	case dlNotEqual:
		var equality ast.ASTHandle
		equality, err = p.manager.MkEq(left, right)
		if err == nil {
			result, err = p.manager.MkNot(equality)
		}
	case dlLess, dlGreater:
		if op == dlGreater {
			left, right = right, left
		}
		finite, e := p.manager.IsDatalogFiniteSort(sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if finite {
			result, err = p.manager.MkDatalogLT(left, right)
		} else {
			result, err = p.manager.MkArithmeticLT(left, right)
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = p.pin(result)
	if err == nil {
		err = p.advance()
	}
	return result, err
}
