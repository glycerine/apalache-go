package cmd_context

// Source: dparser::parse_decls, parse_rule, parse_body and add_rule.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseRules() error {
	for p.current.kind != dlEOF {
		switch p.current.kind {
		case dlNewline:
			if err := p.advance(); err != nil {
				return err
			}
		case dlInclude:
			if err := p.parseInclude(false); err != nil {
				return err
			}
		case dlIdentifier:
			if err := p.parseRule(); err != nil {
				return err
			}
		default:
			return p.expected("predicate or include")
		}
	}
	return nil
}

func (p *datalogParser) parseRule() error {
	p.variables, p.nextVar = make(map[string]ast.ASTHandle), 0
	name := p.current.text
	if err := p.advance(); err != nil {
		return err
	}
	head, declaration, err := p.predicate(name, true)
	if err != nil {
		return err
	}
	if declaration {
		for p.current.kind == dlIdentifier { // source pragmas
			if p.current.text == "printtuples" || p.current.text == "outputtuples" {
				engine, engineErr := p.context.fixedpointContext()
				if engineErr != nil {
					return engineErr
				}
				declaration, found := p.predicates[name]
				if !found {
					return p.expected("registered predicate")
				}
				if engineErr = engine.SetOutputPredicate(declaration); engineErr != nil {
					return engineErr
				}
			}
			if err = p.advance(); err != nil {
				return err
			}
		}
		if p.current.kind == dlPeriod {
			return p.expected("predicate declaration without period")
		}
		if p.current.kind == dlNewline {
			return p.advance()
		}
		if p.current.kind == dlEOF {
			return nil
		}
		if p.current.kind != dlLeftArrow {
			return p.expected("end of predicate declaration or :-")
		}
	}
	if p.current.kind == dlPeriod {
		if err = p.context.AddFixedpointRule(head, ""); err != nil {
			return err
		}
		return p.advance()
	}
	if p.current.kind != dlLeftArrow {
		return p.expected(":- or period")
	}
	if err = p.advance(); err != nil {
		return err
	}
	var tails []ast.ASTHandle
	var negated []bool
	for {
		negative := false
		if p.current.kind == dlNegation {
			negative = true
			if err = p.advance(); err != nil {
				return err
			}
		}
		var tail ast.ASTHandle
		if p.current.kind == dlIdentifier {
			text := p.current.text
			if _, variable := p.variables[text]; variable {
				tail, err = p.parseInfix()
			} else {
				if err = p.advance(); err == nil {
					tail, _, err = p.predicate(text, false)
				}
			}
		} else if p.current.kind == dlString || p.current.kind == dlNumber {
			tail, err = p.parseInfix()
		} else {
			return p.expected("predicate or infix relation")
		}
		if err != nil {
			return err
		}
		tails, negated = append(tails, tail), append(negated, negative)
		if p.current.kind == dlPeriod {
			break
		}
		if p.current.kind != dlComma {
			return p.expected("comma or period")
		}
		if err = p.advance(); err != nil {
			return err
		}
	}
	rule, err := p.fixedpointRule(head, tails, negated)
	if err != nil {
		return err
	}
	if err = p.context.AddFixedpointRule(rule, ""); err != nil {
		return err
	}
	return p.advance()
}

func (p *datalogParser) fixedpointRule(head ast.ASTHandle, tails []ast.ASTHandle, negated []bool) (ast.ASTHandle, error) {
	body := make([]ast.ASTHandle, len(tails))
	for i, tail := range tails {
		body[i] = tail
		if negated[i] {
			var err error
			body[i], err = p.manager.MkNot(tail)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	conjunction, err := p.manager.MkAndArray(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formula, err := p.manager.MkImplies(conjunction, head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pin(formula)
}
