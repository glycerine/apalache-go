package core

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func pbPreprocessSubsumes(a, b *pbPreprocessConstraint) bool {
	if a == nil || b == nil || b.bound.Cmp(a.bound) > 0 {
		return false
	}
	for _, left := range a.terms {
		found := false
		for _, right := range b.terms {
			if left.expression == right.expression {
				if left.coefficient.Cmp(right.coefficient) > 0 {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (t *PBPreprocessTactic) subsume(g *tactic.Goal, variables map[ast.ASTHandle]*pbPreprocessOccurrences, constraints map[uint32]*pbPreprocessConstraint) (bool, error) {
	progress := false
	for firstIndex, first := range constraints {
		if len(first.terms) == 0 {
			continue
		}
		base, positive, e := t.literalBase(first.terms[0].expression)
		if e != nil {
			return false, e
		}
		occurrences := variables[base]
		if occurrences == nil {
			continue
		}
		candidates := occurrences.positive
		if !positive {
			candidates = occurrences.negative
		}
		for _, secondIndex := range candidates {
			if secondIndex == firstIndex {
				continue
			}
			second := constraints[secondIndex]
			if !pbPreprocessSubsumes(first, second) {
				continue
			}
			truth, e := t.manager.MkTrue()
			if e != nil {
				return false, e
			}
			pr, e := g.Proof(secondIndex)
			if e != nil {
				return false, e
			}
			firstDep, e := g.Dependency(firstIndex)
			if e != nil {
				return false, e
			}
			secondDep, e := g.Dependency(secondIndex)
			if e != nil {
				return false, e
			}
			dep, e := t.manager.DependencyJoin(firstDep, secondDep)
			if e != nil {
				return false, e
			}
			if e = g.Update(secondIndex, truth, pr, dep); e != nil {
				_ = t.manager.DependencyDec(dep)
				return false, e
			}
			if e = t.manager.DependencyDec(dep); e != nil {
				return false, e
			}
			progress = true
		}
	}
	return progress, nil
}
