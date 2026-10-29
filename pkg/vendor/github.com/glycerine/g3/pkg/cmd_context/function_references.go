package cmd_context

// Source: src/cmd_context/cmd_context.cpp find_func_decl overloads.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// FindFunctionDeclaration resolves an SMT-LIB declaration reference. A nil
// domain denotes a bare name; a non-nil domain supplies an exact signature.
// The caller must pin the borrowed result before creating other ASTs.
func (c *Context) FindFunctionDeclaration(name string, indices []uint32, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (declaration ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return declaration, err
	}
	if builtins := c.builtins[name]; len(builtins) != 0 {
		op := builtins[0]
		if len(domain) > 0 {
			family, e := m.GetFamilyID(domain[0])
			if e != nil {
				return declaration, e
			}
			for _, candidate := range builtins {
				if candidate.family == family {
					op = candidate
					break
				}
			}
		}
		parameters := make([]ast.ASTParameter, len(indices))
		defer func() {
			for i := range parameters {
				if parameters[i] != (ast.ASTParameter{}) {
					err = errors.Join(err, m.DelParameter(&parameters[i]))
				}
			}
		}()
		for i, index := range indices {
			parameters[i], err = m.NewParameterUint(index)
			if err != nil {
				return declaration, err
			}
		}
		declaration, err = m.MkFamilyFuncDecl(op.family, op.kind, parameters, domain, rangeSort)
		if err != nil && domain == nil {
			return declaration, fmt.Errorf("invalid function declaration reference, must provide signature for builtin symbol %s: %w", name, err)
		}
		return declaration, err
	}
	for _, macro := range c.macros[name] {
		matches := domain == nil
		if domain != nil && len(macro.domain) == len(domain) {
			matches = true
			for i, sort := range domain {
				matches = matches && macro.domain[i] == sort
			}
		}
		if matches {
			return declaration, fmt.Errorf("invalid function declaration reference, named expressions (aka macros) cannot be referenced %s", name)
		}
	}
	if len(indices) != 0 {
		return declaration, fmt.Errorf("invalid indexed function declaration reference, unknown builtin function %s", name)
	}
	for _, candidate := range c.functions[name] {
		if domain != nil {
			arity, e := m.GetArity(candidate)
			if e != nil {
				return declaration, e
			}
			if int(arity) != len(domain) {
				continue
			}
			rangeValue, e := m.GetRange(candidate)
			if e != nil {
				return declaration, e
			}
			if rangeValue != rangeSort {
				continue
			}
			matches := true
			for i, sort := range domain {
				actual, e := m.GetDomain(candidate, uint32(i))
				if e != nil {
					return declaration, e
				}
				matches = matches && actual == sort
			}
			if !matches {
				continue
			}
		}
		if !declaration.IsNull() {
			return ast.ASTHandle{}, fmt.Errorf("ambiguous function declaration reference, provide full signature to disambiguate %s", name)
		}
		declaration = candidate
	}
	if declaration.IsNull() {
		return declaration, fmt.Errorf("invalid function declaration reference, unknown function %s", name)
	}
	return declaration, nil
}
