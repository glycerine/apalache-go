package core

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type pbPreprocessOccurrences struct{ positive, negative []uint32 }

func (t *PBPreprocessTactic) literalBase(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if child, yes, e := t.negated(h); e != nil {
		return ast.ASTHandle{}, false, e
	} else if yes {
		ok, x := t.isConstant(child)
		return child, !ok, x
	}
	ok, e := t.isConstant(h)
	return h, ok, e
}

func (t *PBPreprocessTactic) classify(g *tactic.Goal) (map[ast.ASTHandle]*pbPreprocessOccurrences, map[uint32]*pbPreprocessConstraint, error) {
	variables := make(map[ast.ASTHandle]*pbPreprocessOccurrences)
	constraints := make(map[uint32]*pbPreprocessConstraint)
	n, e := g.Size()
	if e != nil {
		return nil, nil, e
	}
	for i := uint32(0); i < n; i++ {
		h, x := g.Form(i)
		if x != nil {
			return nil, nil, x
		}
		parsed, x := t.parse(h)
		if x != nil {
			return nil, nil, x
		}
		if parsed == nil {
			continue
		}
		constraints[i] = parsed
		for _, term := range parsed.terms {
			base, positive, x := t.literalBase(term.expression)
			if x != nil {
				return nil, nil, x
			}
			if t.manager.IsTrue(base) || t.manager.IsFalse(base) {
				continue
			}
			entry := variables[base]
			if entry == nil {
				entry = &pbPreprocessOccurrences{}
				variables[base] = entry
			}
			if positive {
				entry.positive = append(entry.positive, i)
			} else {
				entry.negative = append(entry.negative, i)
			}
		}
	}
	// Any occurrence outside the classified PB/or/constant fragment disqualifies
	// the variable, exactly as the source declassifier traversal.
	for i := uint32(0); i < n; i++ {
		if constraints[i] != nil {
			continue
		}
		h, x := g.Form(i)
		if x != nil {
			return nil, nil, x
		}
		for variable := range variables {
			contains, x := t.contains(h, variable)
			if x != nil {
				return nil, nil, x
			}
			if contains {
				delete(variables, variable)
			}
		}
	}
	return variables, constraints, nil
}

func (t *PBPreprocessTactic) contains(root, wanted ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		i := len(stack) - 1
		h := stack[i]
		stack = stack[:i]
		if h == wanted {
			return true, nil
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		k, e := t.manager.GetKind(h)
		if e != nil {
			return false, e
		}
		if k != ast.ASTApp {
			continue
		}
		n, e := t.manager.GetNumArgs(h)
		if e != nil {
			return false, e
		}
		for j := uint32(0); j < n; j++ {
			a, x := t.manager.GetArg(h, j)
			if x != nil {
				return false, x
			}
			stack = append(stack, a)
		}
	}
	return false, nil
}
