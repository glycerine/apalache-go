package fp

// Source: dparser::parse_pred and parse_args declaration branch.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (p *datalogParser) parseDeclarationOrRule() error {
	name := p.token.text
	nameSymbol, err := p.symbol(name)
	if err != nil {
		return err
	}
	p.advance()
	declaration, exists, err := p.context.Predicate(nameSymbol)
	if err != nil {
		return err
	}
	if !exists {
		return p.parseDeclaration(nameSymbol)
	}
	return p.parseRule(name, declaration)
}

func (p *datalogParser) parseDeclaration(name usymbol.Symbol) (resultErr error) {
	defer func() { resultErr = joinErrors(resultErr, p.closeArguments()) }()
	var domain []ast.ASTHandle
	var names []usymbol.Symbol
	if p.token.kind == datalogLeftParen {
		p.advance()
		if p.token.kind == datalogRightParen {
			return p.fail("expected argument name in predicate declaration")
		}
		if p.token.kind != datalogRightParen {
			for p.token.kind != datalogEOF {
				if p.token.kind != datalogIdentifier {
					return p.fail("expected argument name in predicate declaration")
				}
				argument, err := p.symbol(p.token.text)
				if err != nil {
					return err
				}
				p.advance()
				if err = p.expect(datalogColon, "expected ':' in predicate declaration"); err != nil {
					return err
				}
				if p.token.kind != datalogIdentifier {
					return p.fail("expected argument sort")
				}
				sortName, ok := domainBaseName(p.token.text)
				if !ok || p.sorts[sortName].IsNull() {
					return p.fail("unknown argument sort")
				}
				domain = append(domain, p.sorts[sortName])
				names = append(names, argument)
				p.advance()
				if p.token.kind == datalogRightParen {
					break
				}
				if p.token.kind == datalogComma {
					p.advance()
				}
			}
		}
		if p.token.kind == datalogRightParen {
			p.advance()
		}
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	declaration, err := p.manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return err
	}
	if err = p.context.RegisterPredicate(declaration, true); err != nil {
		return err
	}
	for p.token.kind == datalogIdentifier {
		if p.token.text == "printtuples" || p.token.text == "outputtuples" {
			if err = p.context.SetOutputPredicate(declaration); err != nil {
				return err
			}
		}
		p.advance()
	}
	if err = p.context.SetArgumentNames(declaration, names); err != nil {
		return err
	}
	if p.token.kind == datalogLeftArrow {
		// parse_pred creates an application even for a new declaration, and
		// parse_rule accepts that application as a rule head before ':-'.
		p.vars = make(map[string]ast.ASTHandle)
		p.nextVar = 0
		args := make([]ast.ASTHandle, len(domain))
		for i, sort := range domain {
			reference, cause := p.newArgumentRef()
			if cause != nil {
				return cause
			}
			args[i], err = p.manager.MkVar(p.nextVar, sort)
			if err != nil {
				return err
			}
			if err = p.manager.SetRef(reference, args[i]); err != nil {
				return err
			}
		}
		headRef, err := p.newArgumentRef()
		if err != nil {
			return err
		}
		defer func() { resultErr = joinErrors(resultErr, p.releaseArgumentRef(&headRef)) }()
		head, err := p.manager.MkApp(declaration, args)
		if err != nil {
			return err
		}
		if err = p.manager.SetRef(headRef, head); err != nil {
			return err
		}
		return p.parseRuleBody(head)
	}
	if p.token.kind != datalogNewline && p.token.kind != datalogEOF {
		return p.fail("predicate declaration should not end with '.'")
	}
	return nil
}
