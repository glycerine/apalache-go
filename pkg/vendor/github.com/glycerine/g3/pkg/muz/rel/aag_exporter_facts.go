package rel

import "github.com/glycerine/g3/pkg/ast"

func (x *AAGExporter) lowerFact(f AAGFact) (uint32, error) {
	terms, e := x.predicateTerms(ast.ASTHandle{}, x.current)
	if e != nil {
		return 0, e
	}
	after, e := x.predicateTerms(f.Declaration, x.next)
	if e != nil {
		return 0, e
	}
	terms = append(terms, after...)
	if e = x.ensureArgumentLatches(len(f.Values)); e != nil {
		return 0, e
	}
	for i, v := range f.Values {
		eq, e := x.manager.MkEq(x.argNext[i], v)
		if e != nil {
			return 0, e
		}
		terms = append(terms, eq)
	}
	formula, e := x.andExpressions(terms)
	if e != nil {
		return 0, e
	}
	return x.builder.Expression(formula)
}
