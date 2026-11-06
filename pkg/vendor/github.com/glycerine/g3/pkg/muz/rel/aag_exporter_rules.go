package rel

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (x *AAGExporter) lowerRule(rule *base.Rule) (uint32, error) {
	positive, e := rule.PositiveTailSize()
	if e != nil {
		return 0, e
	}
	uninterpreted, e := rule.UninterpretedTailSize()
	if e != nil {
		return 0, e
	}
	if positive > 1 {
		return 0, fmt.Errorf("%w: non-linear clauses not supported", ErrAAG)
	}
	if positive != uninterpreted {
		return 0, fmt.Errorf("%w: negation of queries not supported", ErrAAG)
	}
	head, e := rule.Head()
	if e != nil {
		return 0, e
	}
	headDecl, e := x.manager.GetDecl(head)
	if e != nil {
		return 0, e
	}
	terms, e := x.predicateTerms(headDecl, x.next)
	if e != nil {
		return 0, e
	}
	var tail ast.ASTHandle
	if positive == 1 {
		tail, _, e = rule.Tail(0)
		if e != nil {
			return 0, e
		}
		d, e := x.manager.GetDecl(tail)
		if e != nil {
			return 0, e
		}
		before, e := x.predicateTerms(d, x.current)
		if e != nil {
			return 0, e
		}
		terms = append(before, terms...)
	} else {
		before, e := x.predicateTerms(ast.ASTHandle{}, x.current)
		if e != nil {
			return 0, e
		}
		terms = append(before, terms...)
	}
	bindings := map[uint32]ast.ASTHandle{}
	if positive == 1 {
		if e = x.collectArguments(tail, false, bindings, &terms); e != nil {
			return 0, e
		}
	}
	if e = x.collectArguments(head, true, bindings, &terms); e != nil {
		return 0, e
	}
	size, e := rule.TailSize()
	if e != nil {
		return 0, e
	}
	for i := uninterpreted; i < size; i++ {
		condition, _, e := rule.Tail(i)
		if e != nil {
			return 0, e
		}
		condition, e = x.substitute(condition, bindings)
		if e != nil {
			return 0, e
		}
		terms = append(terms, condition)
	}
	formula, e := x.andExpressions(terms)
	if e != nil {
		return 0, e
	}
	return x.builder.Expression(formula)
}
func (x *AAGExporter) andExpressions(terms []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(terms) == 0 {
		return x.manager.MkTrue()
	}
	r := terms[0]
	var e error
	for _, term := range terms[1:] {
		r, e = x.manager.MkAnd(r, term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return r, nil
}
