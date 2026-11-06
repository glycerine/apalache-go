package arith

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// lia2cardDependencies owns the returned dependency root. Rewriting a term
// using a 0/1 indicator depends on both bounds that justify that indicator.
func (t *LIA2CardTactic) lia2cardDependencies(original ast.ASTHandle, variables []ast.ASTHandle, zeroOne map[ast.ASTHandle]bool, originalDep ast.ASTDependency) (result ast.ASTDependency, err error) {
	m := t.manager
	result = originalDep
	if err = m.DependencyInc(result); err != nil {
		return ast.ASTDependency{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, m.DependencyDec(result))
			result = ast.ASTDependency{}
		}
	}()
	for _, variable := range variables {
		if !zeroOne[variable] {
			continue
		}
		used, e := m.OccursExpr(variable, original)
		if e != nil {
			return result, e
		}
		if !used {
			continue
		}
		lower, e := t.bounds.LowerDependency(variable)
		if e != nil {
			return result, e
		}
		upper, e := t.bounds.UpperDependency(variable)
		if e != nil {
			return result, e
		}
		for _, dependency := range []ast.ASTDependency{lower, upper} {
			joined, e := m.DependencyJoin(result, dependency)
			if e != nil {
				return result, e
			}
			if e = m.DependencyInc(joined); e != nil {
				return result, e
			}
			if e = m.DependencyDec(result); e != nil {
				return result, errors.Join(e, m.DependencyDec(joined))
			}
			result = joined
		}
	}
	return result, nil
}
