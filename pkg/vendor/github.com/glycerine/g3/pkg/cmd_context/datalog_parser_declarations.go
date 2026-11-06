package cmd_context

// Source: dparser::parse_pred and declaration branch of parse_args.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (p *datalogParser) predicate(name string, declarationAllowed bool) (ast.ASTHandle, bool, error) {
	declaration, found := p.predicates[name]
	if !found {
		if !declarationAllowed {
			return ast.ASTHandle{}, false, fmt.Errorf("datalog:%d: predicate %s must be declared before use", p.current.line, name)
		}
		domain, names, err := p.declarationArguments()
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		declaration, err = p.context.DeclareRelation(name, domain)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		p.predicates[name] = declaration
		engine, err := p.context.fixedpointContext()
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if err = engine.SetArgumentNames(declaration, names); err != nil {
			return ast.ASTHandle{}, false, err
		}
		arguments := make([]ast.ASTHandle, len(domain))
		for i, sort := range domain {
			arguments[i], err = p.manager.MkVar(uint32(i), sort)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
		}
		application, err := p.manager.MkApp(declaration, arguments)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		application, err = p.pin(application)
		return application, true, err
	}
	arguments, err := p.applicationArguments(declaration)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	application, err := p.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	application, err = p.pin(application)
	return application, false, err
}

func (p *datalogParser) declarationArguments() ([]ast.ASTHandle, []usymbol.Symbol, error) {
	if p.current.kind != dlLeftParen {
		return nil, nil, nil
	}
	if err := p.advance(); err != nil {
		return nil, nil, err
	}
	var domain []ast.ASTHandle
	var names []usymbol.Symbol
	for {
		if err := p.require(dlIdentifier, "argument name"); err != nil {
			return nil, nil, err
		}
		argumentName, err := p.manager.Symbols.Intern(p.current.text)
		if err != nil {
			return nil, nil, err
		}
		if err := p.advance(); err != nil {
			return nil, nil, err
		}
		if err := p.require(dlColon, "colon in predicate declaration"); err != nil {
			return nil, nil, err
		}
		if err := p.advance(); err != nil {
			return nil, nil, err
		}
		if err := p.require(dlIdentifier, "sort name"); err != nil {
			return nil, nil, err
		}
		name, err := datalogDomainName(p.current.text)
		if err != nil {
			return nil, nil, err
		}
		sort, found := p.sorts[name]
		if !found {
			return nil, nil, fmt.Errorf("datalog:%d: unknown sort %s", p.current.line, name)
		}
		domain = append(domain, sort)
		names = append(names, argumentName)
		if err = p.advance(); err != nil {
			return nil, nil, err
		}
		if p.current.kind == dlRightParen {
			return domain, names, p.advance()
		}
		if p.current.kind != dlComma {
			return nil, nil, p.expected("comma or right parenthesis")
		}
		if err = p.advance(); err != nil {
			return nil, nil, err
		}
	}
}

func (p *datalogParser) applicationArguments(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	if arity == 0 && p.current.kind != dlLeftParen {
		return nil, nil
	}
	if err = p.require(dlLeftParen, "left parenthesis"); err != nil {
		return nil, err
	}
	if err = p.advance(); err != nil {
		return nil, err
	}
	arguments := make([]ast.ASTHandle, 0, arity)
	for uint32(len(arguments)) < arity {
		sort, e := p.manager.GetDomain(declaration, uint32(len(arguments)))
		if e != nil {
			return nil, e
		}
		argument, e := p.parseArgument(sort)
		if e != nil {
			return nil, e
		}
		arguments = append(arguments, argument)
		if uint32(len(arguments)) == arity {
			break
		}
		if p.current.kind != dlComma {
			return nil, p.expected("comma")
		}
		if e = p.advance(); e != nil {
			return nil, e
		}
	}
	if p.current.kind != dlRightParen {
		return nil, p.expected("right parenthesis")
	}
	return arguments, p.advance()
}
