package rel

// Source: src/muz/rel/check_relation.cpp mk_join, verify_permutation and
// verify_filter_by_negation. These symbolic postconditions do not inspect a
// table representation; input references are borrowed and outputs are owned.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (r *relationFormulaChecker) joinFormulaRefs(left, right ast.ASTRef, leftSorts, rightSorts []ast.ASTHandle, columns, rightColumns []uint32) (result ast.ASTRef, resultErr error) {
	if r == nil || r.manager == nil || len(columns) != len(rightColumns) || uint64(len(leftSorts))+uint64(len(rightSorts)) > math.MaxUint32 {
		return result, ast.ErrASTDomain
	}
	for i, column := range columns {
		other := rightColumns[i]
		if uint64(column) >= uint64(len(leftSorts)) || uint64(other) >= uint64(len(rightSorts)) || leftSorts[column] != rightSorts[other] {
			return result, ast.ErrASTDomain
		}
	}
	m := r.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr != nil {
			result = ast.ASTRef{}
		} else {
			result, owner.result = owner.result, ast.ASTRef{}
		}
	}()
	first, err := m.RefGet(left)
	if err != nil {
		return result, err
	}
	second, err := m.RefGet(right)
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, len(rightSorts))
	for i, sort := range rightSorts {
		bindings[i], err = owner.hold(m.MkVar(uint32(len(leftSorts)+i), sort))
		if err != nil {
			return result, err
		}
	}
	shifted, err := r.substitute(second, bindings)
	if shifted != (ast.ASTRef{}) {
		owner.roots = append(owner.roots, shifted)
	}
	if err != nil {
		return result, err
	}
	second, err = m.RefGet(shifted)
	if err != nil {
		return result, err
	}
	formula, err := owner.hold(m.MkAndArgs([]ast.ASTHandle{first, second}))
	if err != nil {
		return result, err
	}
	for i, column := range columns {
		other := rightColumns[i]
		v1, err := owner.hold(m.MkVar(column, leftSorts[column]))
		if err != nil {
			return result, err
		}
		v2, err := owner.hold(m.MkVar(uint32(len(leftSorts))+other, rightSorts[other]))
		if err != nil {
			return result, err
		}
		equality, err := owner.hold(m.MkEq(v1, v2))
		if err != nil {
			return result, err
		}
		formula, err = owner.hold(m.MkAndArgs([]ast.ASTHandle{equality, formula}))
		if err != nil {
			return result, err
		}
	}
	owner.result, resultErr = m.NewRef(formula)
	return owner.result, resultErr
}

func (r *relationFormulaChecker) renameFormulaRef(source ast.ASTRef, oldSorts, newSorts []ast.ASTHandle, cycle []uint32) (result ast.ASTRef, resultErr error) {
	if r == nil || r.manager == nil || len(oldSorts) != len(newSorts) || uint64(len(oldSorts)) > math.MaxUint32 {
		return result, ast.ErrASTDomain
	}
	permutation := make([]uint32, len(oldSorts))
	for i := range permutation {
		permutation[i] = uint32(i)
	}
	seen := make([]bool, len(oldSorts))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(oldSorts)) || seen[column] {
			return result, ast.ErrASTDomain
		}
		seen[column] = true
	}
	// Source permutate_by_cycle moves the next column into the current
	// column. Substitution therefore maps old cycle[next] to cycle[current].
	for i, column := range cycle {
		permutation[cycle[(i+1)%len(cycle)]] = column
	}
	for i, destination := range permutation {
		if newSorts[destination] != oldSorts[i] {
			return result, ast.ErrASTDomain
		}
	}
	m := r.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr != nil {
			result = ast.ASTRef{}
		} else {
			result, owner.result = owner.result, ast.ASTRef{}
		}
	}()
	formula, err := m.RefGet(source)
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, len(permutation))
	for i, destination := range permutation {
		bindings[i], err = owner.hold(m.MkVar(destination, oldSorts[i]))
		if err != nil {
			return result, err
		}
	}
	owner.result, resultErr = r.substitute(formula, bindings)
	return owner.result, resultErr
}

func (r *relationFormulaChecker) negationFormulaRefs(target, negative ast.ASTRef, targetSorts, negativeSorts []ast.ASTHandle, columns, negativeColumns []uint32) (result ast.ASTRef, resultErr error) {
	if r == nil || r.manager == nil || len(columns) != len(negativeColumns) || uint64(len(targetSorts))+uint64(len(negativeSorts)) > math.MaxUint32 {
		return result, ast.ErrASTDomain
	}
	for i, column := range columns {
		other := negativeColumns[i]
		if uint64(column) >= uint64(len(targetSorts)) || uint64(other) >= uint64(len(negativeSorts)) || targetSorts[column] != negativeSorts[other] {
			return result, ast.ErrASTDomain
		}
	}
	m := r.manager
	owner := &typedUDocFormulaOwner{manager: m}
	defer func() {
		resultErr = finishTypedUDocFormula(owner, resultErr)
		if resultErr != nil {
			result = ast.ASTRef{}
		} else {
			result, owner.result = owner.result, ast.ASTRef{}
		}
	}()
	old, err := m.RefGet(target)
	if err != nil {
		return result, err
	}
	neg, err := m.RefGet(negative)
	if err != nil {
		return result, err
	}
	terms := []ast.ASTHandle{neg}
	for i, column := range columns {
		other := negativeColumns[i]
		v1, err := owner.hold(m.MkVar(uint32(len(negativeSorts))+column, targetSorts[column]))
		if err != nil {
			return result, err
		}
		v2, err := owner.hold(m.MkVar(other, negativeSorts[other]))
		if err != nil {
			return result, err
		}
		equality, err := owner.hold(m.MkEq(v1, v2))
		if err != nil {
			return result, err
		}
		terms = append(terms, equality)
	}
	body, err := owner.hold(m.MkAndArray(terms))
	if err != nil {
		return result, err
	}
	exists := body
	if len(negativeSorts) != 0 {
		bound := make([]ast.ASTHandle, len(negativeSorts))
		names := make([]symbol.Symbol, len(negativeSorts))
		for i, sort := range negativeSorts {
			bound[len(bound)-1-i] = sort
			names[i] = symbol.Numerical(uint32(i))
		}
		exists, err = owner.hold(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: body}))
		if err != nil {
			return result, err
		}
	}
	negated, err := owner.hold(m.MkNot(exists))
	if err != nil {
		return result, err
	}
	formula, err := owner.hold(m.MkAndArgs([]ast.ASTHandle{old, negated}))
	if err != nil {
		return result, err
	}
	owner.result, resultErr = m.NewRef(formula)
	return owner.result, resultErr
}
