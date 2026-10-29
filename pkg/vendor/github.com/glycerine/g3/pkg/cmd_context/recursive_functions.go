package cmd_context

// Source: src/cmd_context/cmd_context.cpp cmd_context::insert_rec_fun.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// DefineRecursiveFunction consumes an already published declaration so the
// body can refer to itself or the other declarations of a mutual definition.
func (c *Context) DefineRecursiveFunction(declaration ast.ASTHandle, names []usymbol.Symbol, body ast.ASTHandle) (result error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	arity, err := m.GetArity(declaration)
	if err != nil {
		return err
	}
	if int(arity) != len(names) {
		return ast.ErrASTDomain
	}
	rangeSort, err := m.GetRange(declaration)
	if err != nil {
		return err
	}
	actual, err := m.GetSort(body)
	if err != nil {
		return err
	}
	if actual != rangeSort {
		return fmt.Errorf("invalid function definition, sort mismatch")
	}
	domain := make([]ast.ASTHandle, arity)
	bindings := make([]ast.ASTHandle, arity)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err == nil {
			pins = append(pins, ref)
		}
		return h, err
	}
	for i := uint32(0); i < arity; i++ {
		domain[i], err = m.GetDomain(declaration, i)
		if err != nil {
			return err
		}
		bindings[i], err = pin(m.MkVar(arity-i-1, domain[i]))
		if err != nil {
			return err
		}
	}
	kind, err := m.GetKind(body)
	if err != nil {
		return err
	}
	if arity != 0 && kind == ast.ASTVar {
		name, err := m.GetASTName(declaration)
		if err != nil {
			return err
		}
		text, err := m.Symbols.String(name)
		if err != nil {
			return err
		}
		return c.defineFunction(text, domain, body, true)
	}
	if arity != 0 && kind != ast.ASTApp {
		return fmt.Errorf("Z3 only supports recursive definitions that are proper terms (not binders or variables)")
	}
	lhs, err := pin(m.MkApp(declaration, bindings))
	if err != nil {
		return err
	}
	definition, err := pin(m.MkEq(lhs, body))
	if err != nil {
		return err
	}
	if arity != 0 {
		leftPattern, err := pin(m.MkPattern([]ast.ASTHandle{lhs}))
		if err != nil {
			return err
		}
		rightPattern, err := pin(m.MkPattern([]ast.ASTHandle{body}))
		if err != nil {
			return err
		}
		qid, err := m.RecursiveFunctionQID()
		if err != nil {
			return err
		}
		definition, err = pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: domain, Names: names, Body: definition, QID: qid, Weight: 0, Patterns: []ast.ASTHandle{leftPattern, rightPattern}}))
		if err != nil {
			return err
		}
	}
	return c.Assert(definition, "")
}
