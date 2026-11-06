package fp

// Source: src/muz/rel/dl_compiler.cpp, get_local_indexes_for_projection and
// compile_rule; src/muz/base/dl_util.h, variable_intersection::populate.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type boundIntervalJoinColumn struct {
	variable uint32
	constant *big.Rat
	sort     ast.ASTHandle
}

func (a *boundIntervalDatalogActivation) joinBoundIntervalPositiveTails(rule *base.Rule, inputs boundIntervalRuleInputs) (*rel.IntervalBoundProduct, []ast.ASTHandle, map[uint32]uint32, map[uint32][]uint32, error) {
	var tails [2]*rel.IntervalBoundProduct
	var columns []boundIntervalJoinColumn
	boundary := 0
	counts := make(map[uint32]int)
	for i := range tails {
		tail, negated, err := rule.Tail(i)
		if err != nil || negated {
			return nil, nil, nil, nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		declaration, err := a.manager.GetDecl(tail)
		if err != nil || inputs.relation(declaration, i) == nil || a.entries[declaration] == nil {
			return nil, nil, nil, nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		tails[i] = inputs.relation(declaration, i)
		arguments, err := a.manager.GetNumArgs(tail)
		sorts := a.entries[declaration].sorts
		if err != nil || int(arguments) != len(sorts) {
			return nil, nil, nil, nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		for column := uint32(0); column < arguments; column++ {
			argument, err := a.manager.GetArg(tail, column)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			kind, err := a.manager.GetKind(argument)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			entry := boundIntervalJoinColumn{sort: sorts[column]}
			if kind == ast.ASTVar {
				entry.variable, err = a.manager.GetVarIndex(argument)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				counts[entry.variable]++
			} else {
				actualSort, sortErr := a.manager.GetSort(argument)
				text, _, numeral, numeralErr := a.manager.ArithmeticNumeralText(argument)
				value, valid := new(big.Rat).SetString(text)
				if err = errors.Join(sortErr, numeralErr); err != nil || !numeral || !valid || actualSort != entry.sort {
					return nil, nil, nil, nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
				}
				entry.constant = value
			}
			columns = append(columns, entry)
		}
		if i == 0 {
			boundary = len(columns)
		}
	}

	// Head multiplicity determines how many copies survive the fused join.
	// A variable used only by a condition needs one copy until filtering.
	needed := make(map[uint32]int)
	head, err := rule.Head()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	headSize, err := a.manager.GetNumArgs(head)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for i := uint32(0); i < headSize; i++ {
		argument, cause := a.manager.GetArg(head, i)
		if cause != nil {
			return nil, nil, nil, nil, cause
		}
		kind, cause := a.manager.GetKind(argument)
		if cause != nil {
			return nil, nil, nil, nil, cause
		}
		if kind == ast.ASTVar {
			variable, cause := a.manager.GetVarIndex(argument)
			if cause != nil {
				return nil, nil, nil, nil, cause
			}
			needed[variable]++
		}
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for i := 2; i < tailSize; i++ {
		condition, _, cause := rule.Tail(i)
		if cause != nil {
			return nil, nil, nil, nil, cause
		}
		used, cause := ast.NewASTUsedVars(a.manager)
		if cause != nil {
			return nil, nil, nil, nil, cause
		}
		if cause = used.Collect(condition); cause != nil {
			return nil, nil, nil, nil, cause
		}
		for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
			if !used.Contains(variable).IsNull() && needed[variable] == 0 {
				needed[variable] = 1
			}
		}
	}

	// Preserve every cross-tail occurrence pair in left-major order. The
	// join establishes those equalities before any columns are projected.
	var leftKeys, rightKeys []uint32
	for i, left := range columns[:boundary] {
		for j, right := range columns[boundary:] {
			if left.constant != nil || right.constant != nil || left.variable != right.variable {
				continue
			}
			if left.sort != right.sort {
				return nil, nil, nil, nil, ErrBoundIntervalDatalogFragment
			}
			leftKeys = append(leftKeys, uint32(i))
			rightKeys = append(rightKeys, uint32(j))
		}
	}
	var removed []uint32
	var kept []boundIntervalJoinColumn
	var sorts []ast.ASTHandle
	keptBoundary := uint32(0)
	variables := make(map[uint32]uint32)
	occurrences := make(map[uint32][]uint32)
	var order []uint32
	for i, column := range columns {
		if column.constant == nil && counts[column.variable] > needed[column.variable] {
			counts[column.variable]--
			removed = append(removed, uint32(i))
			continue
		}
		index := uint32(len(kept))
		if column.constant == nil {
			if len(occurrences[column.variable]) == 0 {
				order = append(order, column.variable)
			}
			variables[column.variable] = index
			occurrences[column.variable] = append(occurrences[column.variable], index)
		}
		kept = append(kept, column)
		sorts = append(sorts, column.sort)
		if i < boundary {
			keptBoundary++
		}
	}
	var working *rel.IntervalBoundProduct
	if len(removed) == 0 {
		working, err = rel.JoinIntervalBoundProducts(tails[0], tails[1], leftKeys, rightKeys)
	} else {
		working, err = rel.JoinProjectIntervalBoundProducts(tails[0], tails[1], leftKeys, rightKeys, removed)
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for i, column := range kept {
		if column.constant != nil {
			if err = working.FilterEqual(uint32(i), column.constant); err != nil {
				return nil, nil, nil, nil, err
			}
		}
	}
	for _, variable := range order {
		indexes := occurrences[variable]
		if len(indexes) < 2 || indexes[0] < keptBoundary && indexes[len(indexes)-1] >= keptBoundary {
			continue // cross-tail equalities were already enforced by the join
		}
		for _, index := range indexes[1:] {
			if err = working.FilterIdentical(indexes[0], index); err != nil {
				return nil, nil, nil, nil, err
			}
		}
	}
	return working, sorts, variables, occurrences, nil
}
