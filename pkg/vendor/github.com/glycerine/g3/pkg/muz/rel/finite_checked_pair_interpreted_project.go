package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func checkedFiniteInterpretedProject(table FiniteTable, manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, removed []int) (_ FiniteTable, resultErr error) {
	clone, err := table.CloneFinite()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, clone.Close()) }()
	if err = filterCheckedPairInterpreted(clone, manager, relationSorts, theory, conditions, columns, sorts); err != nil {
		return nil, err
	}
	return finiteTableProject(clone, removed)
}

// FilterInterpretedAndProject mirrors the source check_table fused
// transformer: checker first, selected primary second, followed by an
// extensional comparison of the independently owned projected results.
func (p *FiniteCheckedPair) FilterInterpretedAndProject(manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle, removed []int) (*FiniteCheckedPair, error) {
	if err := p.verify("filter interpreted project input"); err != nil {
		return nil, err
	}
	checker, err := checkedFiniteInterpretedProject(p.checker, manager, relationSorts, theory, conditions, columns, sorts, removed)
	if err != nil {
		return nil, err
	}
	primary, err := checkedFiniteInterpretedProject(p.primary, manager, relationSorts, theory, conditions, columns, sorts, removed)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}
