package util

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

var errParserSyntax = errors.New("parser error")

type builtinOp struct {
	family ast.FamilyID
	kind   int32
}

type SimpleParser struct {
	manager  *ast.ASTManager
	builtins map[string]builtinOp
	vars     map[string]ast.ASTHandle
	integer  func(*big.Rat) (ast.ASTHandle, error)
	floating func(*big.Rat) (ast.ASTHandle, error)
	warnings io.Writer
}

func NewSimpleParser(manager *ast.ASTManager) (*SimpleParser, error) {
	if manager == nil {
		return nil, fmt.Errorf("parser: nil AST manager")
	}
	return &SimpleParser{manager: manager, builtins: make(map[string]builtinOp), vars: make(map[string]ast.ASTHandle), warnings: os.Stderr}, nil
}

func (p *SimpleParser) SetWarnings(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	p.warnings = w
}

func (p *SimpleParser) SetNumberParsers(integer, floating func(*big.Rat) (ast.ASTHandle, error)) {
	p.integer, p.floating = integer, floating
}

func (p *SimpleParser) AddBuiltin(name string, family ast.FamilyID, kind int32) error {
	if name == "" || p.manager == nil {
		return fmt.Errorf("parser: invalid builtin")
	}
	if _, found := p.builtins[name]; found {
		return fmt.Errorf("parser: duplicate name %q", name)
	}
	if _, found := p.vars[name]; found {
		return fmt.Errorf("parser: duplicate name %q", name)
	}
	p.builtins[name] = builtinOp{family, kind}
	return nil
}

func (p *SimpleParser) AddVariable(name string, variable ast.ASTHandle) error {
	if name == "" || variable.IsNull() {
		return fmt.Errorf("parser: invalid variable")
	}
	if _, found := p.builtins[name]; found {
		return fmt.Errorf("parser: duplicate name %q", name)
	}
	if _, found := p.vars[name]; found {
		return fmt.Errorf("parser: duplicate name %q", name)
	}
	p.vars[name] = variable
	return nil
}

func (p *SimpleParser) ResetVariables() { p.vars = make(map[string]ast.ASTHandle) }
func (p *SimpleParser) Reset() {
	p.builtins = make(map[string]builtinOp)
	p.vars = make(map[string]ast.ASTHandle)
}

func (p *SimpleParser) parseExpression(scanner *Scanner, trail *[]ast.ASTRef) (ast.ASTHandle, bool, error) {
	token := scanner.Scan()
	switch token {
	case LeftParen:
		if scanner.Scan() != Identifier {
			return ast.ASTHandle{}, false, errParserSyntax
		}
		op, found := p.builtins[scanner.ID()]
		if !found {
			return ast.ASTHandle{}, false, errParserSyntax
		}
		var args []ast.ASTHandle
		for {
			arg, present, err := p.parseExpression(scanner, trail)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if !present {
				break
			}
			args = append(args, arg)
		}
		result, err := p.manager.MkFamilyApp(op.family, op.kind, nil, args)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		ref, err := p.manager.NewRef(result)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		*trail = append(*trail, ref)
		return result, true, nil
	case RightParen:
		return ast.ASTHandle{}, false, nil
	case Identifier:
		if op, found := p.builtins[scanner.ID()]; found {
			result, err := p.manager.MkFamilyApp(op.family, op.kind, nil, nil)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			ref, err := p.manager.NewRef(result)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			*trail = append(*trail, ref)
			return result, true, nil
		}
		if variable, found := p.vars[scanner.ID()]; found {
			return variable, true, nil
		}
		return ast.ASTHandle{}, false, errParserSyntax
	case Integer, Float:
		factory := p.integer
		if token == Float {
			factory = p.floating
		}
		if factory == nil {
			return ast.ASTHandle{}, false, errParserSyntax
		}
		result, err := factory(scanner.Number())
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		ref, err := p.manager.NewRef(result)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		*trail = append(*trail, ref)
		return result, true, nil
	default:
		return ast.ASTHandle{}, false, errParserSyntax
	}
}

func (p *SimpleParser) Parse(reader io.Reader) (result ast.ASTRef, ok bool, err error) {
	if reader == nil {
		return ast.ASTRef{}, false, fmt.Errorf("parser: nil reader")
	}
	scanner := NewScanner(reader, os.Stderr, false, false)
	var trail []ast.ASTRef
	defer func() {
		for i := len(trail) - 1; i >= 0; i-- {
			err = errors.Join(err, p.manager.DelRef(&trail[i]))
		}
		ok = ok && err == nil
	}()
	handle, present, err := p.parseExpression(scanner, &trail)
	if errors.Is(err, errParserSyntax) {
		_, _ = io.WriteString(p.warnings, "parser error\n")
		return ast.ASTRef{}, false, nil
	}
	if err != nil || !present {
		return ast.ASTRef{}, false, err
	}
	result, err = p.manager.NewRef(handle)
	return result, err == nil, err
}

func (p *SimpleParser) ParseString(text string) (ast.ASTRef, bool, error) {
	return p.Parse(strings.NewReader(text))
}

func (p *SimpleParser) ParseFile(path string) (ast.ASTRef, bool, error) {
	if path == "" {
		return p.Parse(os.Stdin)
	}
	file, err := os.Open(path)
	if err != nil {
		return ast.ASTRef{}, false, err
	}
	defer file.Close()
	return p.Parse(file)
}
