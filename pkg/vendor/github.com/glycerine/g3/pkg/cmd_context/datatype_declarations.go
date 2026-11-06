package cmd_context

// Source: src/cmd_context/cmd_context.cpp dt_eh::operator(), concrete datatype
// instantiations. Parametric declaration owners remain a separate parser port.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// DeclareDatatypes publishes a mutually recursive block in the current
// declaration scope. Nullary declarations publish their functions immediately;
// parametric declarations publish each concrete instantiation on first use.
func (c *Context) DeclareDatatypes(definitions []ast.DatatypeDefinition) (result error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if len(definitions) == 0 {
		return nil
	}
	names := make([]string, len(definitions))
	seen := make(map[string]bool)
	for i, d := range definitions {
		name, e := m.Symbols.String(d.Name)
		if e != nil {
			return e
		}
		names[i] = name
		if c.sortNameAvailable(name) || seen[name] {
			return fmt.Errorf("sort already defined %s", name)
		}
		seen[name] = true
	}
	sorts, err := m.MkDatatypes(definitions)
	if err != nil {
		return err
	}
	functionsStart, sortsStart := len(c.functionTrail), len(c.sortTrail)
	defer func() {
		if result != nil {
			result = errors.Join(result, c.restoreDeclarations(functionsStart, sortsStart))
		}
	}()
	for i, s := range sorts {
		ref, e := m.NewRef(s)
		if e != nil {
			return e
		}
		c.sorts[names[i]] = s
		c.sortArities[names[i]] = uint32(len(definitions[i].Parameters))
		c.sortTrail = append(c.sortTrail, declaration{names[i], s, ref})
		if len(definitions[i].Parameters) == 0 {
			if e = c.ensureDatatypeFunctions(s); e != nil {
				return e
			}
		}
	}
	return nil
}

// InstantiateSort applies a declared user, alias, or datatype sort constructor.
// Datatype applications also publish the corresponding function family.
func (c *Context) InstantiateSort(name string, arguments []ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if declaration, found := c.sortConstructors[name]; found {
		return c.instantiateSortConstructor(declaration, arguments)
	}
	if _, found := c.sorts[name]; !found {
		return ast.ASTHandle{}, fmt.Errorf("unknown sort %s", name)
	}
	arity := c.sortArities[name]
	if uint32(len(arguments)) != arity {
		return ast.ASTHandle{}, fmt.Errorf("invalid number of parameters to sort constructor %s: got %d, want %d", name, len(arguments), arity)
	}
	if arity == 0 {
		return c.sorts[name], nil
	}
	functionsStart := len(c.functionTrail)
	defer func() {
		if err != nil {
			err = errors.Join(err, c.restoreDeclarations(functionsStart, len(c.sortTrail)))
		}
	}()
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parameters := make([]ast.ASTParameter, 0, len(arguments)+1)
	nameParameter, err := m.NewParameterSymbol(symbol)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parameters = append(parameters, nameParameter)
	defer func() {
		for i := len(parameters) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelParameter(&parameters[i]))
		}
	}()
	for _, argument := range arguments {
		parameter, e := m.NewParameterAST(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		parameters = append(parameters, parameter)
	}
	family, err := m.DatatypeFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = m.MkFamilySort(family, ast.ASTDatatypeSort, parameters)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.ensureDatatypeFunctions(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (c *Context) ensureDatatypeFunctions(sort ast.ASTHandle) error {
	constructors, err := c.manager.DatatypeConstructors(sort)
	if err != nil {
		return err
	}
	for _, constructor := range constructors {
		if err = c.ensureDatatypeFunction(constructor); err != nil {
			return err
		}
		recognizer, e := c.manager.DatatypeConstructorRecognizer(constructor)
		if e != nil {
			return e
		}
		if err = c.ensureDatatypeFunction(recognizer); err != nil {
			return err
		}
		accessors, e := c.manager.DatatypeConstructorAccessors(constructor)
		if e != nil {
			return e
		}
		for _, accessor := range accessors {
			if err = c.ensureDatatypeFunction(accessor); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Context) ensureDatatypeFunction(declaration ast.ASTHandle) error {
	symbol, err := c.manager.GetASTName(declaration)
	if err != nil {
		return err
	}
	name, err := c.manager.Symbols.String(symbol)
	if err != nil {
		return err
	}
	for _, prior := range c.functions[name] {
		if prior == declaration {
			return nil
		}
	}
	return c.insertDatatypeFunction(declaration)
}

func (c *Context) insertDatatypeFunction(d ast.ASTHandle) error {
	m := c.manager
	if err := c.logicChecker.CheckDeclaration(d); err != nil {
		return err
	}
	symbol, err := m.GetASTName(d)
	if err != nil {
		return err
	}
	name, err := m.Symbols.String(symbol)
	if err != nil {
		return err
	}
	arity, err := m.GetArity(d)
	if err != nil {
		return err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i], err = m.GetDomain(d, uint32(i))
		if err != nil {
			return err
		}
	}
	if c.containsMacro(name, domain) {
		return fmt.Errorf("invalid declaration, named expression already defined with this name %s", name)
	}
	if _, found := c.builtins[name]; found {
		return fmt.Errorf("invalid declaration, builtin symbol %s", name)
	}
	for _, prior := range c.functions[name] {
		if prior == d {
			return fmt.Errorf("invalid declaration, '%s' (with the given signature) already declared", name)
		}
	}
	ref, err := m.NewRef(d)
	if err != nil {
		return err
	}
	c.functions[name] = append(c.functions[name], d)
	c.functionTrail = append(c.functionTrail, declaration{name, d, ref})
	return nil
}
