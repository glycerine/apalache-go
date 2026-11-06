package fp

// Source: dparser::parse_rule, parse_body, parse_infix and add_rule.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseRule(name string, declaration ast.ASTHandle) (resultErr error) {
	defer func() { resultErr = joinErrors(resultErr, p.closeArguments()) }()
	p.vars = make(map[string]ast.ASTHandle)
	p.nextVar = 0
	arguments, err := p.parseArguments(declaration)
	if err != nil {
		return err
	}
	headRef, err := p.newArgumentRef()
	if err != nil {
		return err
	}
	defer func() { resultErr = joinErrors(resultErr, p.releaseArgumentRef(&headRef)) }()
	head, err := p.manager.MkApp(declaration, arguments)
	if err != nil {
		return err
	}
	if err = p.manager.SetRef(headRef, head); err != nil {
		return err
	}
	if p.token.kind == datalogPeriod {
		p.advance()
		return p.addRule(head, nil, nil)
	}
	return p.parseRuleBody(head)
}

func (p *datalogParser) parseRuleBody(head ast.ASTHandle) (resultErr error) {
	if err := p.expect(datalogLeftArrow, "expected '.' or ':-' after rule head"); err != nil {
		return err
	}
	var tail []ast.ASTHandle
	var negated []bool
	var tailRefs []ast.ASTRef
	defer func() {
		for i := len(tailRefs) - 1; i >= 0; i-- {
			resultErr = joinErrors(resultErr, p.releaseArgumentRef(&tailRefs[i]))
		}
	}()
	for {
		// The source loop returns EOS without publishing its partial rule.
		if p.token.kind == datalogEOF {
			return nil
		}
		// Source parse_body accepts a period at the start of each iteration,
		// including an empty body and a body ending in a comma.
		if p.token.kind == datalogPeriod {
			p.advance()
			break
		}
		negative := false
		if p.token.kind == datalogNegation {
			negative = true
			p.advance()
		}
		reference, err := p.newArgumentRef()
		if err != nil {
			return err
		}
		tailRefs = append(tailRefs, reference)
		expression, err := p.parseBodyExpression()
		if err != nil {
			return err
		}
		if err = p.manager.SetRef(reference, expression); err != nil {
			return err
		}
		tail = append(tail, expression)
		negated = append(negated, negative)
		if p.token.kind == datalogPeriod {
			p.advance()
			break
		}
		if err = p.expect(datalogComma, "expected comma or period in rule body"); err != nil {
			return err
		}
	}
	return p.addRule(head, tail, negated)
}

func (p *datalogParser) parseBodyExpression() (ast.ASTHandle, error) {
	first := p.token
	if first.kind != datalogIdentifier && first.kind != datalogString && first.kind != datalogNumber {
		return ast.ASTHandle{}, p.fail("expected predicate in rule body")
	}
	p.advance()
	_, knownVariable := p.vars[first.text]
	if first.kind != datalogIdentifier || knownVariable {
		return p.parseInfix(first)
	}
	name, err := p.symbol(first.text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, exists, err := p.context.Predicate(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !exists {
		return ast.ASTHandle{}, p.fail("predicate must be declared before use")
	}
	arguments, err := p.parseArguments(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.manager.MkApp(declaration, arguments)
}

func (p *datalogParser) parseInfix(first datalogToken) (ast.ASTHandle, error) {
	operator := p.token.kind
	if operator != datalogEqual && operator != datalogNotEqual && operator != datalogLess && operator != datalogGreater {
		return ast.ASTHandle{}, p.fail("expected built-in infix operator")
	}
	p.advance()
	second := p.token
	secondVariable, secondIsVariable := p.vars[second.text]
	firstVariable, firstIsVariable := p.vars[first.text]
	secondIsVariable = secondIsVariable && second.kind == datalogIdentifier
	firstIsVariable = firstIsVariable && first.kind == datalogIdentifier
	if second.kind != datalogString && second.kind != datalogNumber && !secondIsVariable {
		return ast.ASTHandle{}, p.fail("expected constant or previously typed variable")
	}
	if !firstIsVariable && !secondIsVariable {
		return ast.ASTHandle{}, p.fail("at least one infix operand must be a variable")
	}
	variable := firstVariable
	if !firstIsVariable {
		variable = secondVariable
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := p.ownedArgument(first, sort, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.ownedArgument(second, sort, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.advance()
	switch operator {
	case datalogEqual:
		return p.manager.MkEq(left, right)
	case datalogNotEqual:
		reference, e := p.newArgumentRef()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equal, e := p.manager.MkEq(left, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = p.manager.SetRef(reference, equal); e != nil {
			return ast.ASTHandle{}, e
		}
		return p.manager.MkNot(equal)
	case datalogLess, datalogGreater:
		if operator == datalogGreater {
			left, right = right, left
		}
		finite, e := p.manager.IsDatalogFiniteSort(sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if finite {
			return p.manager.MkDatalogLT(left, right)
		}
		// The original legacy command rejects arithmetic sorts here, but the
		// established Go parser accepts its live integer-domain extension.
		return p.manager.MkArithmeticLT(left, right)
	}
	return ast.ASTHandle{}, p.fail("unsupported infix operator")
}

func (p *datalogParser) addRule(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool) (resultErr error) {
	manager, err := p.context.RuleManager()
	if err != nil {
		return err
	}
	rule, err := manager.NewRule(head, tail, negated, "")
	if err != nil {
		return err
	}
	defer func() { resultErr = joinErrors(resultErr, rule.Close()) }()
	return p.context.AddRule(rule)
}
