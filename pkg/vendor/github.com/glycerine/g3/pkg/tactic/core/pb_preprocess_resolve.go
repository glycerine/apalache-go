package core

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *PBPreprocessTactic) complements(a, b ast.ASTHandle) (bool, error) {
	ac, an, e := t.negated(a)
	if e != nil {
		return false, e
	}
	bc, bn, e := t.negated(b)
	if e != nil {
		return false, e
	}
	if an {
		return ac == b, nil
	}
	if bn {
		return bc == a, nil
	}
	return false, nil
}

func (t *PBPreprocessTactic) simplifyPB(h ast.ASTHandle) (_ ast.ASTHandle, result error) {
	r, e := rewriter.NewTheoryRewriter(t.manager, t.services.Context, t.services.Globals, nil)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	defer func() { result = errors.Join(result, r.Close()) }()
	out, e := t.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	defer func() { result = errors.Join(result, t.manager.DelRef(&out)) }()
	if e = r.Rewrite(h, out); e != nil {
		return ast.ASTHandle{}, e
	}
	return t.manager.RefGet(out)
}

func (t *PBPreprocessTactic) recordBoolean(converter *tactic.ExtensionModelConverter, literal ast.ASTHandle, value bool) error {
	base, negated, e := t.negated(literal)
	if e != nil {
		return e
	}
	if !negated {
		base = literal
	} else {
		value = !value
	}
	decl, e := t.manager.GetDecl(base)
	if e != nil {
		return e
	}
	v, e := t.manager.MkFalse()
	if value {
		v, e = t.manager.MkTrue()
	}
	if e != nil {
		return e
	}
	return converter.Insert(decl, v)
}

// resolveOne implements the retained, deliberately narrow subsumption-
// resolution case and performs at most one resolution per outer pass.
func (t *PBPreprocessTactic) resolveOne(g *tactic.Goal, converter *tactic.ExtensionModelConverter, variables map[ast.ASTHandle]*pbPreprocessOccurrences, constraints map[uint32]*pbPreprocessConstraint) (bool, error) {
	for _, occ := range variables {
		var firstIndex, secondIndex uint32
		if len(occ.positive) == 1 && len(occ.negative) == 1 {
			firstIndex, secondIndex = occ.positive[0], occ.negative[0]
		} else {
			continue
		}
		first, second := constraints[firstIndex], constraints[secondIndex]
		if first == nil || second == nil || first.bound.Cmp(big.NewRat(1, 1)) != 0 {
			continue
		}
		indices := make([]int, 0, len(first.terms))
		min := -1
		valid := true
		for _, term := range first.terms {
			base, _, e := t.literalBase(term.expression)
			if e != nil {
				return false, e
			}
			o := variables[base]
			if o == nil || len(o.positive) != 1 || len(o.negative) != 1 {
				valid = false
				break
			}
			found := -1
			for j, candidate := range second.terms {
				yes, e := t.complements(term.expression, candidate.expression)
				if e != nil {
					return false, e
				}
				if yes {
					found = j
					break
				}
			}
			if found < 0 {
				valid = false
				break
			}
			indices = append(indices, found)
			if min < 0 || second.terms[found].coefficient.Cmp(second.terms[min].coefficient) < 0 {
				min = found
			}
		}
		if !valid || min < 0 {
			continue
		}
		nextTerms := make([]pbPreprocessTerm, len(second.terms))
		for i, term := range second.terms {
			nextTerms[i] = pbPreprocessTerm{expression: term.expression, coefficient: new(big.Rat).Set(term.coefficient)}
		}
		truth, e := t.manager.MkTrue()
		if e != nil {
			return false, e
		}
		falsehood, e := t.manager.MkFalse()
		if e != nil {
			return false, e
		}
		for _, index := range indices {
			value := index != min
			if value {
				nextTerms[index].expression = truth
			} else {
				nextTerms[index].expression = falsehood
			}
			if e = t.recordBoolean(converter, second.terms[index].expression, value); e != nil {
				return false, e
			}
		}
		raw, e := t.makeGE(nextTerms, second.bound)
		if e != nil {
			return false, e
		}
		rewritten, e := t.simplifyPB(raw)
		if e != nil {
			return false, e
		}
		pr, e := g.Proof(secondIndex)
		if e != nil {
			return false, e
		}
		leftDep, e := g.Dependency(firstIndex)
		if e != nil {
			return false, e
		}
		rightDep, e := g.Dependency(secondIndex)
		if e != nil {
			return false, e
		}
		joined, e := t.manager.DependencyJoin(leftDep, rightDep)
		if e != nil {
			return false, e
		}
		if e = g.Update(secondIndex, rewritten, pr, joined); e != nil {
			_ = t.manager.DependencyDec(joined)
			return false, e
		}
		if e = t.manager.DependencyDec(joined); e != nil {
			return false, e
		}
		if e = g.Update(firstIndex, truth, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return false, e
		}
		return true, nil
	}
	return false, nil
}
