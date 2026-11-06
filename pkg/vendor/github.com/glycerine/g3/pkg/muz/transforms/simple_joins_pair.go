package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp join_planner::get_normalizer and
// join_planner::get_key.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type normalizedPairKey struct {
	left  ast.ASTHandle
	right ast.ASTHandle
}

type normalizedPair struct {
	key               normalizedPairKey
	terms             [2]ast.ASTHandle
	actualToCanonical map[uint32]uint32
	canonicalToActual []uint32
	canonicalSorts    []ast.ASTHandle
}

func (p *SimpleJoinsPlugin) normalizePair(first, second ast.ASTHandle) (normalizedPair, error) {
	terms := [2]ast.ASTHandle{first, second}
	declarations := [2]ast.ASTHandle{}
	var err error
	declarations[0], err = p.manager.GetDecl(terms[0])
	if err != nil {
		return normalizedPair{}, err
	}
	declarations[1], err = p.manager.GetDecl(terms[1])
	if err != nil {
		return normalizedPair{}, err
	}
	firstID, err := p.manager.GetASTID(declarations[0])
	if err != nil {
		return normalizedPair{}, err
	}
	secondID, err := p.manager.GetASTID(declarations[1])
	if err != nil {
		return normalizedPair{}, err
	}
	if declarations[0] != declarations[1] {
		if firstID < secondID {
			terms[0], terms[1] = terms[1], terms[0]
			declarations[0], declarations[1] = declarations[1], declarations[0]
		}
	} else {
		leftCount, x := p.manager.GetNumArgs(terms[0])
		if x != nil {
			return normalizedPair{}, x
		}
		rightCount, x := p.manager.GetNumArgs(terms[1])
		if x != nil {
			return normalizedPair{}, x
		}
		if leftCount != rightCount {
			return normalizedPair{}, fmt.Errorf("muz/transforms: same-predicate pair has different arities")
		}
		leftPattern := make(map[uint32]int)
		rightPattern := make(map[uint32]int)
		swap := false
		for index := uint32(0); index < leftCount; index++ {
			leftArg, x := p.manager.GetArg(terms[0], index)
			if x != nil {
				return normalizedPair{}, x
			}
			rightArg, x := p.manager.GetArg(terms[1], index)
			if x != nil {
				return normalizedPair{}, x
			}
			leftSort, x := p.manager.GetSort(leftArg)
			if x != nil {
				return normalizedPair{}, x
			}
			rightSort, x := p.manager.GetSort(rightArg)
			if x != nil {
				return normalizedPair{}, x
			}
			leftSortID, x := p.manager.GetASTID(leftSort)
			if x != nil {
				return normalizedPair{}, x
			}
			rightSortID, x := p.manager.GetASTID(rightSort)
			if x != nil {
				return normalizedPair{}, x
			}
			if leftSort != rightSort {
				swap = leftSortID < rightSortID
				break
			}
			leftVar, x := p.manager.GetVarIndex(leftArg)
			if x != nil {
				return normalizedPair{}, x
			}
			rightVar, x := p.manager.GetVarIndex(rightArg)
			if x != nil {
				return normalizedPair{}, x
			}
			leftSeenInRight, leftSeen := rightPattern[leftVar]
			rightSeenInLeft, rightSeen := leftPattern[rightVar]
			leftPosition, rightPosition := -1, -1
			if leftSeen {
				leftPosition = leftSeenInRight
			}
			if rightSeen {
				rightPosition = rightSeenInLeft
			}
			if leftPosition != rightPosition {
				swap = leftPosition < rightPosition
				break
			}
			leftPattern[leftVar] = int(index)
			rightPattern[rightVar] = int(index)
		}
		if swap {
			terms[0], terms[1] = terms[1], terms[0]
		}
	}

	result := normalizedPair{actualToCanonical: make(map[uint32]uint32)}
	for _, term := range terms {
		count, x := p.manager.GetNumArgs(term)
		if x != nil {
			return normalizedPair{}, x
		}
		for index := uint32(0); index < count; index++ {
			argument, y := p.manager.GetArg(term, index)
			if y != nil {
				return normalizedPair{}, y
			}
			variable, y := p.manager.GetVarIndex(argument)
			if y != nil {
				return normalizedPair{}, fmt.Errorf("muz/transforms: pair normalizer requires variable arguments: %w", y)
			}
			if _, found := result.actualToCanonical[variable]; found {
				continue
			}
			canonical := uint32(len(result.canonicalToActual))
			result.actualToCanonical[variable] = canonical
			result.canonicalToActual = append(result.canonicalToActual, variable)
			sort, y := p.manager.GetSort(argument)
			if y != nil {
				return normalizedPair{}, y
			}
			result.canonicalSorts = append(result.canonicalSorts, sort)
		}
	}
	for termIndex, term := range terms {
		declaration, x := p.manager.GetDecl(term)
		if x != nil {
			return normalizedPair{}, x
		}
		count, x := p.manager.GetNumArgs(term)
		if x != nil {
			return normalizedPair{}, x
		}
		arguments := make([]ast.ASTHandle, count)
		for index := uint32(0); index < count; index++ {
			argument, y := p.manager.GetArg(term, index)
			if y != nil {
				return normalizedPair{}, y
			}
			variable, y := p.manager.GetVarIndex(argument)
			if y != nil {
				return normalizedPair{}, y
			}
			canonical := result.actualToCanonical[variable]
			arguments[index], y = p.manager.MkVar(canonical, result.canonicalSorts[canonical])
			if y != nil {
				return normalizedPair{}, y
			}
		}
		result.terms[termIndex], x = p.manager.MkApp(declaration, arguments)
		if x != nil {
			return normalizedPair{}, x
		}
	}
	leftID, err := p.manager.GetASTID(result.terms[0])
	if err != nil {
		return normalizedPair{}, err
	}
	rightID, err := p.manager.GetASTID(result.terms[1])
	if err != nil {
		return normalizedPair{}, err
	}
	if leftID > rightID {
		result.terms[0], result.terms[1] = result.terms[1], result.terms[0]
	}
	result.key = normalizedPairKey{left: result.terms[0], right: result.terms[1]}
	return result, nil
}
