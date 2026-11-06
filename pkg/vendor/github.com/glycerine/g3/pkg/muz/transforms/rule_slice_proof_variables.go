package transforms

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// derivedVariables matches a sliced derived rule to its reconstructed
// original rule, following the recorded predicate-column projection. A false
// result means the correspondence is ambiguous or outside this matcher.
func (c *SliceProofConverter) derivedVariables(proof ast.ASTHandle, original *base.Rule) (mapping []uint32, matched bool, resultErr error) {
	m := c.manager
	fact, err := m.GetProofFact(proof)
	if err != nil {
		return nil, false, err
	}
	body := fact
	quantified, err := m.IsForall(fact)
	if err != nil {
		return nil, false, err
	}
	if quantified {
		body, err = m.GetQuantifierBody(fact)
		if err != nil {
			return nil, false, err
		}
	}
	before, err := ast.NewASTUsedVars(m)
	if err != nil {
		return nil, false, err
	}
	if err = before.Collect(body); err != nil {
		return nil, false, err
	}

	sliced, err := c.rules.NewRuleFromFormula(fact, "")
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, sliced.Close()) }()
	used, err := sliced.UsedVariables()
	if err != nil {
		return nil, false, err
	}
	mapping = make([]uint32, used.MaxFoundVarIndexPlusOne())
	found := make([]bool, len(mapping))
	inverse := make(map[uint32]uint32)
	var matchTerm func(ast.ASTHandle, ast.ASTHandle) (bool, error)
	matchTerm = func(small, large ast.ASTHandle) (bool, error) {
		sk, e := m.GetKind(small)
		if e != nil {
			return false, e
		}
		lk, e := m.GetKind(large)
		if e != nil {
			return false, e
		}
		if sk == ast.ASTVar {
			if lk != ast.ASTVar {
				return false, nil
			}
			si, e := m.GetVarIndex(small)
			if e != nil {
				return false, e
			}
			li, e := m.GetVarIndex(large)
			if e != nil {
				return false, e
			}
			ss, e := m.GetSort(small)
			if e != nil {
				return false, e
			}
			ls, e := m.GetSort(large)
			if e != nil {
				return false, e
			}
			if int(si) >= len(mapping) || ss != ls || (found[si] && mapping[si] != li) {
				return false, nil
			}
			if previous, exists := inverse[li]; exists && previous != si {
				return false, nil
			}
			mapping[si], found[si] = li, true
			inverse[li] = si
			return true, nil
		}
		if sk != ast.ASTApp || lk != ast.ASTApp {
			return small == large, nil
		}
		sd, e := m.GetDecl(small)
		if e != nil {
			return false, e
		}
		ld, e := m.GetDecl(large)
		if e != nil {
			return false, e
		}
		size, e := m.GetNumArgs(small)
		if e != nil {
			return false, e
		}
		largeSize, e := m.GetNumArgs(large)
		if e != nil {
			return false, e
		}
		var removed []bool
		if origin, ok := c.predicates[sd]; ok {
			if ld != origin.declaration || uint32(len(origin.removed)) != largeSize {
				return false, nil
			}
			removed = origin.removed
		} else if sd != ld || size != largeSize {
			return false, nil
		}
		var j uint32
		for i := uint32(0); i < largeSize; i++ {
			if len(removed) != 0 && removed[i] {
				continue
			}
			if j >= size {
				return false, nil
			}
			sa, x := m.GetArg(small, j)
			if x != nil {
				return false, x
			}
			la, x := m.GetArg(large, i)
			if x != nil {
				return false, x
			}
			ok, x := matchTerm(sa, la)
			if x != nil || !ok {
				return false, x
			}
			j++
		}
		return j == size, nil
	}
	smallHead, err := sliced.Head()
	if err != nil {
		return nil, false, err
	}
	largeHead, err := original.Head()
	if err != nil {
		return nil, false, err
	}
	ok, err := matchTerm(smallHead, largeHead)
	if err != nil || !ok {
		return nil, false, err
	}
	smallPredicates, err := sliced.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	largePredicates, err := original.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	if smallPredicates != largePredicates {
		return nil, false, nil
	}
	for i := 0; i < smallPredicates; i++ {
		a, an, e := sliced.Tail(i)
		if e != nil {
			return nil, false, e
		}
		b, bn, e := original.Tail(i)
		if e != nil {
			return nil, false, e
		}
		if an != bn {
			return nil, false, nil
		}
		ok, e := matchTerm(a, b)
		if e != nil || !ok {
			return nil, false, e
		}
	}
	// Resolution can leave variables that occur only in theory constraints.
	// Predicate columns cannot identify those variables; use the unchanged
	// interpreted tails when the predicate correspondence is incomplete.
	complete := true
	for _, present := range found {
		complete = complete && present
	}
	if !complete {
		smallSize, e := sliced.TailSize()
		if e != nil {
			return nil, false, e
		}
		largeSize, e := original.TailSize()
		if e != nil {
			return nil, false, e
		}
		if smallSize-smallPredicates != largeSize-largePredicates {
			return nil, false, nil
		}
		for i := smallPredicates; i < smallSize; i++ {
			a, an, e := sliced.Tail(i)
			if e != nil {
				return nil, false, e
			}
			b, bn, e := original.Tail(i)
			if e != nil {
				return nil, false, e
			}
			if an != bn {
				return nil, false, nil
			}
			ok, e := matchTerm(a, b)
			if e != nil || !ok {
				return nil, false, e
			}
		}
	}
	for i := range mapping {
		if !found[i] {
			return nil, false, nil
		}
	}
	bankSize := before.MaxFoundVarIndexPlusOne()
	if quantified {
		count, e := m.GetNumDeclarations(fact)
		if e != nil {
			return nil, false, e
		}
		if count > bankSize {
			bankSize = count
		}
	}
	if !before.UsesAllVars(bankSize) {
		expanded := make([]uint32, bankSize)
		next := 0
		for i := range expanded {
			expanded[i] = ^uint32(0)
			if !before.Contains(uint32(i)).IsNull() {
				if next >= len(mapping) {
					return nil, false, nil
				}
				expanded[i] = mapping[next]
				next++
			}
		}
		if next != len(mapping) {
			return nil, false, nil
		}
		mapping = expanded
	}
	return mapping, true, nil
}
