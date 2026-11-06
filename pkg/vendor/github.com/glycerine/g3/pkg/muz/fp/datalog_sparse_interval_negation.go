package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// applySparseIntervalFiniteNegation implements the retained exact table path
// from dl_compiler.cpp. Mixed products and raw arithmetic relations remain
// unsupported because their source plugins do not provide this operation.
func (e *SparseIntervalDatalogEvaluator) applySparseIntervalFiniteNegation(rule *base.Rule, first, last int, working *rel.SparseIntervalSieveProduct, workingSorts []ast.ASTHandle, variables map[uint32]int) (*rel.SparseIntervalSieveProduct, []ast.ASTHandle, error) {
	for tailIndex := first; tailIndex < last; tailIndex++ {
		tail, negated, err := rule.Tail(tailIndex)
		if err != nil || !negated {
			return working, workingSorts, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
		declaration, err := e.manager.GetDecl(tail)
		negative := e.products[declaration]
		negativeSorts := e.sorts[declaration]
		if err != nil || negative == nil {
			return working, workingSorts, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
		count, err := e.manager.GetNumArgs(tail)
		if err != nil || int(count) != len(negativeSorts) {
			return working, workingSorts, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
		targetColumns := make([]int, 0, count)
		negativeColumns := make([]int, 0, count)
		for column := 0; column < int(count); column++ {
			argument, cause := e.manager.GetArg(tail, uint32(column))
			if cause != nil {
				return working, workingSorts, cause
			}
			kind, cause := e.manager.GetKind(argument)
			if cause != nil {
				return working, workingSorts, cause
			}
			var targetColumn int
			switch kind {
			case ast.ASTVar:
				variable, cause := e.manager.GetVarIndex(argument)
				if cause != nil {
					return working, workingSorts, cause
				}
				var found bool
				targetColumn, found = variables[variable]
				if !found {
					working, workingSorts, targetColumn, cause = appendSparseIntervalFullColumn(working, workingSorts, negativeSorts[column])
					if cause != nil {
						return working, workingSorts, cause
					}
					variables[variable] = targetColumn
				}
			case ast.ASTApp:
				working, workingSorts, targetColumn, cause = appendSparseIntervalFullColumn(working, workingSorts, negativeSorts[column])
				if cause == nil {
					cause = working.FilterEqual(targetColumn, argument)
				}
				if cause != nil {
					return working, workingSorts, cause
				}
			default:
				return working, workingSorts, ErrSparseIntervalDatalogFragment
			}
			targetColumns = append(targetColumns, targetColumn)
			negativeColumns = append(negativeColumns, column)
		}
		if _, err = working.FilterFiniteNegation(negative, targetColumns, negativeColumns); err != nil {
			return working, workingSorts, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
	}
	return working, workingSorts, nil
}
