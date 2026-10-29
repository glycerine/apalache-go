package model

// Source: src/model/model2expr.cpp:24-61; src/model/model2expr.h:28-39.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// FreshName is model2expr's concrete symbol_set and A..Z, A1.. name source.
// The set is native Go storage; symbols remain owned by the AST manager.
type FreshName struct {
	manager *ast.ASTManager
	used    map[usymbol.Symbol]struct{}
	letter  byte
	number  uint32
}

func NewFreshName(m *ast.ASTManager) *FreshName {
	return &FreshName{manager: m, used: make(map[usymbol.Symbol]struct{}), letter: 'A'}
}

func (f *FreshName) AddSymbol(s usymbol.Symbol) {
	f.used[s] = struct{}{}
}

func (f *FreshName) Contains(s usymbol.Symbol) bool {
	_, ok := f.used[s]
	return ok
}

// AddAST collects exactly the symbol kinds visited by for_each_symbol_proc.
// Sort and variable visits add no names; applications visit their declarations
// and arguments, while quantifiers add bound names and visit annotations/body.
func (f *FreshName) AddAST(h ast.ASTHandle) error {
	m := f.manager
	stack := []ast.ASTHandle{h}
	seen := make(map[ast.ASTHandle]struct{})
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		kind, e := m.GetKind(current)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTFuncDecl:
			name, e := m.GetASTName(current)
			if e != nil {
				return e
			}
			f.AddSymbol(name)
		case ast.ASTApp:
			d, e := m.GetDecl(current)
			if e != nil {
				return e
			}
			stack = append(stack, d)
			n, e := m.GetNumArgs(current)
			if e != nil {
				return e
			}
			for i := uint32(0); i < n; i++ {
				a, e := m.GetArg(current, i)
				if e != nil {
					return e
				}
				stack = append(stack, a)
			}
		case ast.ASTQuantifier:
			n, e := m.GetNumDeclarations(current)
			if e != nil {
				return e
			}
			for i := uint32(0); i < n; i++ {
				name, e := m.GetDeclarationName(current, i)
				if e != nil {
					return e
				}
				f.AddSymbol(name)
			}
			body, e := m.GetQuantifierBody(current)
			if e != nil {
				return e
			}
			stack = append(stack, body)
			patterns, e := m.GetNumPatterns(current)
			if e != nil {
				return e
			}
			for i := uint32(0); i < patterns; i++ {
				p, e := m.GetPattern(current, i)
				if e != nil {
					return e
				}
				stack = append(stack, p)
			}
			noPatterns, e := m.GetNumNoPatterns(current)
			if e != nil {
				return e
			}
			for i := uint32(0); i < noPatterns; i++ {
				p, e := m.GetNoPattern(current, i)
				if e != nil {
					return e
				}
				stack = append(stack, p)
			}
		}
	}
	return nil
}

func (f *FreshName) Next() (usymbol.Symbol, error) {
	for {
		for f.letter <= 'Z' {
			name := string(f.letter)
			if f.number > 0 {
				name += strconv.FormatUint(uint64(f.number), 10)
			}
			f.letter++
			s, e := f.manager.Symbols.Intern(name)
			if e != nil {
				return usymbol.Symbol{}, e
			}
			if !f.Contains(s) {
				return s, nil
			}
		}
		f.letter = 'A'
		f.number++
	}
}
