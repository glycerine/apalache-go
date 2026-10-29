package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_declare_datatype(s),
// parse_datatype_dec and parse_constructor_decls: concrete declaration grammar.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type datatypePendingField struct {
	datatype, constructor, accessor int
	name                            string
}

func (p *commandParser) currentDatatypeSort(name string, arguments []ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
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
	return p.pin(result, nil)
}

func (p *commandParser) datatypeFormalParameters(names []string) ([]ast.ASTHandle, map[string]ast.ASTHandle, error) {
	m, err := p.astManager()
	if err != nil {
		return nil, nil, err
	}
	parameters := make([]ast.ASTHandle, 0, len(names))
	byName := make(map[string]ast.ASTHandle, len(names))
	for _, name := range names {
		if _, found := byName[name]; found {
			return nil, nil, fmt.Errorf("duplicate sort parameter %s", name)
		}
		symbol, e := m.Symbols.Intern(name)
		if e != nil {
			return nil, nil, e
		}
		formal, e := p.pin(m.MkUninterpretedSort(symbol, nil))
		if e != nil {
			return nil, nil, e
		}
		parameters = append(parameters, formal)
		byName[name] = formal
	}
	return parameters, byName, nil
}

func (p *commandParser) declareDatatypes(single bool) error {
	m, err := p.context.Manager()
	if err != nil {
		return err
	}
	var definitions []ast.DatatypeDefinition
	var pending []datatypePendingField
	var headers []string
	var arities []uint32
	var commonParameters []string
	modern := false
	previousHeaders := p.datatypeHeaders
	previousParams := p.sortParams
	p.datatypeHeaders = make(map[string]uint32)
	defer func() {
		p.datatypeHeaders = previousHeaders
		p.sortParams = previousParams
	}()
	if single {
		name, e := p.symbol()
		if e != nil {
			return e
		}
		headers = append(headers, name)
		arities = append(arities, 0)
		p.datatypeHeaders[name] = 0
	} else {
		if err = p.expect(LeftParenToken); err != nil {
			return err
		}
		modern = p.token == LeftParenToken
		if modern {
			for p.token != RightParenToken {
				if err = p.expect(LeftParenToken); err != nil {
					return err
				}
				name, e := p.symbol()
				if e != nil {
					return e
				}
				arity, e := p.unsigned()
				if e != nil {
					return e
				}
				headers = append(headers, name)
				arities = append(arities, arity)
				p.datatypeHeaders[name] = arity
				if err = p.expect(RightParenToken); err != nil {
					return err
				}
			}
		} else {
			for p.token != RightParenToken {
				name, e := p.symbol()
				if e != nil {
					return e
				}
				commonParameters = append(commonParameters, name)
			}
		}
		if err = p.expect(RightParenToken); err != nil {
			return err
		}
	}
	parseBody := func(name string, old bool, expectedArity int64) error {
		if e := p.expect(LeftParenToken); e != nil {
			return e
		}
		if old {
			var e error
			name, e = p.symbol()
			if e != nil {
				return e
			}
			p.datatypeHeaders[name] = uint32(len(commonParameters))
		}
		parameterNames := commonParameters
		wrapped := false
		if p.token == SymbolToken && p.scanner.ID() == "par" {
			wrapped = true
			if e := p.advance(); e != nil {
				return e
			}
			if e := p.expect(LeftParenToken); e != nil {
				return e
			}
			parameterNames = nil
			for p.token != RightParenToken {
				parameter, e := p.symbol()
				if e != nil {
					return e
				}
				parameterNames = append(parameterNames, parameter)
			}
			if e := p.expect(RightParenToken); e != nil {
				return e
			}
			if e := p.expect(LeftParenToken); e != nil {
				return e
			}
			p.datatypeHeaders[name] = uint32(len(parameterNames))
		}
		if expectedArity >= 0 && uint32(expectedArity) != uint32(len(parameterNames)) {
			return fmt.Errorf("datatype sort %s declares arity %d but defines %d parameters", name, expectedArity, len(parameterNames))
		}
		parameters, localParams, e := p.datatypeFormalParameters(parameterNames)
		if e != nil {
			return e
		}
		p.sortParams = localParams
		defer func() { p.sortParams = previousParams }()
		symbol, e := m.Symbols.Intern(name)
		if e != nil {
			return e
		}
		definition := ast.DatatypeDefinition{Name: symbol, Parameters: parameters}
		for p.token != RightParenToken {
			parenthesized := p.token == LeftParenToken
			if parenthesized {
				if e = p.advance(); e != nil {
					return e
				}
			}
			constructorName, e := p.symbol()
			if e != nil {
				return e
			}
			constructorSymbol, e := m.Symbols.Intern(constructorName)
			if e != nil {
				return e
			}
			recognizer, e := m.Symbols.Intern("is-" + constructorName)
			if e != nil {
				return e
			}
			constructor := ast.DatatypeConstructorDefinition{Name: constructorSymbol, Recognizer: recognizer}
			if parenthesized {
				for p.token != RightParenToken {
					if e = p.expect(LeftParenToken); e != nil {
						return e
					}
					fieldName, e := p.symbol()
					if e != nil {
						return e
					}
					fieldSymbol, e := m.Symbols.Intern(fieldName)
					if e != nil {
						return e
					}
					field := ast.DatatypeAccessorDefinition{Name: fieldSymbol}
					if p.token == SymbolToken {
						sortName := p.scanner.ID()
						if formal, found := localParams[sortName]; found {
							field.Range = formal
							if e = p.advance(); e != nil {
								return e
							}
						} else {
							pending = append(pending, datatypePendingField{len(definitions), len(definition.Constructors), len(constructor.Accessors), sortName})
							if e = p.advance(); e != nil {
								return e
							}
						}
					} else {
						field.Range, e = p.sort()
						if e != nil {
							return e
						}
					}
					constructor.Accessors = append(constructor.Accessors, field)
					if e = p.expect(RightParenToken); e != nil {
						return e
					}
				}
				if e = p.expect(RightParenToken); e != nil {
					return e
				}
			}
			definition.Constructors = append(definition.Constructors, constructor)
		}
		if len(definition.Constructors) == 0 {
			return fmt.Errorf("invalid datatype declaration, datatype must have at least one constructor")
		}
		definitions = append(definitions, definition)
		if e = p.expect(RightParenToken); e != nil {
			return e
		}
		if wrapped {
			return p.expect(RightParenToken)
		}
		return nil
	}
	if single {
		// A single declaration derives its arity from an optional par binder.
		if err = parseBody(headers[0], false, -1); err != nil {
			return err
		}
	} else {
		if err = p.expect(LeftParenToken); err != nil {
			return err
		}
		for p.token != RightParenToken {
			name := ""
			if modern {
				if len(definitions) >= len(headers) {
					return fmt.Errorf("invalid datatype declaration, too many data-type bodies defined")
				}
				name = headers[len(definitions)]
			}
			expected := int64(len(commonParameters))
			if modern {
				expected = int64(arities[len(definitions)])
			}
			if err = parseBody(name, !modern, expected); err != nil {
				return err
			}
		}
		if modern && len(definitions) != len(headers) {
			return fmt.Errorf("invalid datatype declaration, too few datatype definitions compared to declared sorts")
		}
		if err = p.expect(RightParenToken); err != nil {
			return err
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	indices := make(map[string]uint32)
	for i, d := range definitions {
		name, e := m.Symbols.String(d.Name)
		if e != nil {
			return e
		}
		if _, found := indices[name]; found {
			return fmt.Errorf("datatype sort already declared %s", name)
		}
		indices[name] = uint32(i)
	}
	for _, row := range pending {
		field := &definitions[row.datatype].Constructors[row.constructor].Accessors[row.accessor]
		if index, found := indices[row.name]; found {
			if p.datatypeHeaders[row.name] != 0 {
				return fmt.Errorf("sort constructor expects parameters")
			}
			field.Recursive = true
			field.RecursiveIndex = index
		} else {
			field.Range, err = p.context.LookupSort(row.name)
			if err != nil {
				return fmt.Errorf("invalid datatype declaration, unknown sort '%s'", row.name)
			}
		}
	}
	// Missing references precede duplicate-accessor checks in both retained
	// declaration forms, including mutually recursive blocks.
	for _, definition := range definitions {
		accessorNames := make(map[string]bool)
		for _, constructor := range definition.Constructors {
			for _, accessor := range constructor.Accessors {
				name, e := m.Symbols.String(accessor.Name)
				if e != nil {
					return e
				}
				if accessorNames[name] {
					return fmt.Errorf("invalid datatype declaration, repeated accessor identifier '%s'", name)
				}
				accessorNames[name] = true
			}
		}
	}
	return p.context.DeclareDatatypes(definitions)
}
