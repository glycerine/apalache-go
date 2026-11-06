package fp

// Source: src/muz/rel/dl_compiler.cpp positive rule lowering, specialized
// to the concrete udoc relation operations.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *datalogUDocContext) checkpoint() error {
	canceled, err := c.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(c.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (c *datalogUDocContext) appendConstantColumn(working *rel.TypedUDocRelation, sorts []ast.ASTHandle, sortHandle, value ast.ASTHandle) (*rel.TypedUDocRelation, []ast.ASTHandle, uint32, error) {
	column, err := c.factory.NewFull([]ast.ASTHandle{sortHandle})
	if err != nil {
		return nil, nil, 0, err
	}
	if err = column.FilterEqual(0, value); err != nil {
		return nil, nil, 0, errors.Join(err, column.Close())
	}
	if working == nil {
		return column, []ast.ASTHandle{sortHandle}, 0, nil
	}
	joined, err := working.Join(column, nil, nil)
	if err != nil {
		return nil, nil, 0, errors.Join(err, column.Close())
	}
	if err = errors.Join(working.Close(), column.Close()); err != nil {
		return nil, nil, 0, errors.Join(err, joined.Close())
	}
	return joined, append(sorts, sortHandle), uint32(len(sorts)), nil
}

func (c *datalogUDocContext) duplicateColumn(working *rel.TypedUDocRelation, sorts []ast.ASTHandle, source uint32) (*rel.TypedUDocRelation, []ast.ASTHandle, uint32, error) {
	if len(sorts) == 1 {
		joined, err := working.Join(working, []uint32{source}, []uint32{0})
		if err != nil {
			return nil, nil, 0, err
		}
		if err = working.Close(); err != nil {
			return nil, nil, 0, errors.Join(err, joined.Close())
		}
		return joined, append(sorts, sorts[source]), 1, nil
	}
	remove := make([]uint32, 0, len(sorts)-1)
	for i := range sorts {
		if uint32(i) != source {
			remove = append(remove, uint32(i))
		}
	}
	column, err := working.Project(remove)
	if err != nil {
		return nil, nil, 0, err
	}
	joined, err := working.Join(column, []uint32{source}, []uint32{0})
	if err != nil {
		return nil, nil, 0, errors.Join(err, column.Close())
	}
	if err = errors.Join(working.Close(), column.Close()); err != nil {
		return nil, nil, 0, errors.Join(err, joined.Close())
	}
	return joined, append(sorts, sorts[source]), uint32(len(sorts)), nil
}

func (c *datalogUDocContext) evaluateRule(rule *base.Rule) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	finiteHead, finiteSource, err := classifyUDocFiniteForwarding(c.manager, rule, c.finiteInputs)
	if err != nil {
		return false, err
	}
	if !finiteSource.IsNull() {
		return c.evaluateFiniteForwarding(rule, finiteHead, finiteSource)
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	tails, err := rule.TailSize()
	if err != nil {
		return false, errors.Join(err, ErrUDocDatalogFragment)
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	headDeclaration, err := c.manager.GetDecl(head)
	if err != nil || c.result[headDeclaration] == nil {
		return false, errors.Join(err, fmt.Errorf("%w: undeclared head", ErrUDocDatalogFragment))
	}
	if tails == 0 {
		count, cause := c.manager.GetNumArgs(head)
		if cause != nil {
			return false, cause
		}
		values := make([]ast.ASTHandle, count)
		ground := true
		for i := range values {
			values[i], cause = c.manager.GetArg(head, uint32(i))
			if cause != nil {
				return false, cause
			}
			kind, cause := c.manager.GetKind(values[i])
			if cause != nil {
				return false, cause
			}
			if kind == ast.ASTVar {
				ground = false
				continue
			}
			value, valueErr := c.manager.IsValue(values[i])
			if valueErr != nil || !value {
				return false, errors.Join(valueErr, ErrUDocDatalogFragment)
			}
		}
		if ground {
			return c.result[headDeclaration].AddFact(values)
		}
	}

	var working *rel.TypedUDocRelation
	defer func() {
		if working != nil {
			resultErr = errors.Join(resultErr, working.Close())
		}
	}()
	var workingSorts []ast.ASTHandle
	variables := make(map[uint32]uint32)
	columnVariables := make(map[uint32]uint32)
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return false, errors.Join(cause, ErrUDocDatalogFragment)
		}
		declaration, cause := c.manager.GetDecl(tail)
		if present, finite := c.nullaryResult[declaration]; finite && cause == nil {
			if !present {
				return false, nil
			}
			continue
		}
		source := c.result[declaration]
		if cause != nil || source == nil {
			return false, errors.Join(cause, ErrUDocDatalogFragment)
		}
		next, cause := source.Clone()
		if cause != nil {
			return false, cause
		}
		nextOwned := true
		defer func() {
			if nextOwned {
				resultErr = errors.Join(resultErr, next.Close())
			}
		}()
		empty, cause := next.Empty()
		if cause != nil || empty {
			return false, cause
		}
		local := make(map[uint32]uint32)
		localColumns := make(map[uint32]uint32)
		arguments, cause := c.manager.GetNumArgs(tail)
		if cause != nil || int(arguments) != len(next.Sorts()) {
			return false, errors.Join(cause, ErrUDocDatalogFragment)
		}
		for column := uint32(0); column < arguments; column++ {
			argument, getErr := c.manager.GetArg(tail, column)
			if getErr != nil {
				return false, getErr
			}
			kind, getErr := c.manager.GetKind(argument)
			if getErr != nil {
				return false, getErr
			}
			if kind != ast.ASTVar {
				if getErr = next.FilterEqual(column, argument); getErr != nil {
					return false, errors.Join(getErr, ErrUDocDatalogFragment)
				}
				continue
			}
			variable, getErr := c.manager.GetVarIndex(argument)
			if getErr != nil {
				return false, getErr
			}
			localColumns[column] = variable
			if prior, found := local[variable]; found {
				if getErr = next.FilterIdentical([]uint32{prior, column}); getErr != nil {
					return false, getErr
				}
			} else {
				local[variable] = column
			}
		}
		if working == nil {
			working, nextOwned = next, false
			workingSorts = next.Sorts()
			for variable, column := range local {
				variables[variable] = column
			}
			for column, variable := range localColumns {
				columnVariables[column] = variable
			}
			continue
		}
		shared := make([]uint32, 0, len(local))
		for variable := range local {
			if _, found := variables[variable]; found {
				shared = append(shared, variable)
			}
		}
		sort.Slice(shared, func(i, j int) bool { return shared[i] < shared[j] })
		left, right := make([]uint32, len(shared)), make([]uint32, len(shared))
		for i, variable := range shared {
			left[i], right[i] = variables[variable], local[variable]
		}
		offset := uint32(len(workingSorts))
		removed := make([]uint32, len(right))
		for i, column := range right {
			removed[i] = offset + column
		}
		sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
		nextSorts := next.Sorts()
		joined, getErr := working.JoinProject(next, left, right, removed)
		if getErr != nil {
			return false, getErr
		}
		if getErr = errors.Join(working.Close(), next.Close()); getErr != nil {
			_ = joined.Close()
			return false, getErr
		}
		working, nextOwned = joined, false
		removedLocal := make(map[uint32]bool, len(right))
		for _, column := range right {
			removedLocal[column] = true
		}
		for column, sortHandle := range nextSorts {
			if !removedLocal[uint32(column)] {
				workingSorts = append(workingSorts, sortHandle)
			}
		}
		for variable, column := range local {
			if _, found := variables[variable]; !found {
				shift := uint32(0)
				for removedColumn := range removedLocal {
					if removedColumn < column {
						shift++
					}
				}
				variables[variable] = offset + column - shift
			}
		}
		for column, variable := range localColumns {
			if removedLocal[column] {
				continue
			}
			var shift uint32
			for removedColumn := range removedLocal {
				if removedColumn < column {
					shift++
				}
			}
			columnVariables[offset+column-shift] = variable
		}
	}

	negativeTails, err := c.negativeRuleTails(rule)
	if err != nil {
		return false, err
	}
	if err = c.addUDocNegativeColumns(&working, &workingSorts, negativeTails, variables); err != nil {
		return false, err
	}
	if err = c.addUDocConditionColumns(&working, rule, uninterpreted, tails, variables); err != nil {
		return false, err
	}
	if working != nil {
		workingSorts = working.Sorts()
	}
	for variable, column := range variables {
		columnVariables[column] = variable
	}
	if err = c.filterNegativeTails(&working, &workingSorts, negativeTails, variables); err != nil {
		return false, err
	}
	filtered, err := c.filterInterpretedTailsAndProject(working, rule, head, uninterpreted, tails, variables, columnVariables)
	if err != nil {
		return false, err
	}
	if filtered != working {
		if working != nil {
			err = working.Close()
		}
		if err != nil {
			return false, errors.Join(err, filtered.Close())
		}
		working = filtered
	}
	if working != nil {
		workingSorts = working.Sorts()
	}

	headSorts := c.result[headDeclaration].Sorts()
	count, err := c.manager.GetNumArgs(head)
	if err != nil || int(count) != len(headSorts) {
		return false, errors.Join(err, ErrUDocDatalogFragment)
	}
	projectedInputs, projectedSorts, err := c.projectUDocHeadInputs(working, workingSorts, head, variables)
	if err != nil {
		return false, err
	}
	working, workingSorts = projectedInputs, projectedSorts
	desired := make([]uint32, count)
	used := make(map[uint32]bool)
	for i := uint32(0); i < count; i++ {
		argument, cause := c.manager.GetArg(head, i)
		if cause != nil {
			return false, cause
		}
		kind, cause := c.manager.GetKind(argument)
		if cause != nil {
			return false, cause
		}
		var column uint32
		if kind == ast.ASTVar {
			variable, getErr := c.manager.GetVarIndex(argument)
			if getErr != nil {
				return false, getErr
			}
			var found bool
			column, found = variables[variable]
			if !found {
				appended, appendedSorts, appendedColumn, appendErr := c.appendFullColumn(working, workingSorts, headSorts[i])
				if appendErr != nil {
					return false, appendErr
				}
				working, workingSorts, column = appended, appendedSorts, appendedColumn
				variables[variable] = column
			}
		} else {
			appended, appendedSorts, appendedColumn, appendErr := c.appendConstantColumn(working, workingSorts, headSorts[i], argument)
			if appendErr != nil {
				return false, appendErr
			}
			working, workingSorts, column = appended, appendedSorts, appendedColumn
		}
		if used[column] {
			duplicated, duplicatedSorts, duplicatedColumn, duplicateErr := c.duplicateColumn(working, workingSorts, column)
			if duplicateErr != nil {
				return false, duplicateErr
			}
			working, workingSorts, column = duplicated, duplicatedSorts, duplicatedColumn
		}
		used[column], desired[i] = true, column
	}
	if working == nil {
		if err = c.ensureFullNullary(&working); err != nil {
			return false, err
		}
		workingSorts = working.Sorts()
	}
	selected := append([]uint32(nil), desired...)
	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	remove := make([]uint32, 0, len(workingSorts)-len(selected))
	for column, selectedIndex := uint32(0), 0; int(column) < len(workingSorts); column++ {
		if selectedIndex < len(selected) && selected[selectedIndex] == column {
			selectedIndex++
		} else {
			remove = append(remove, column)
		}
	}
	projected, err := working.Project(remove)
	if err != nil {
		return false, err
	}
	if err = working.Close(); err != nil {
		return false, errors.Join(err, projected.Close())
	}
	working = projected
	current := append([]uint32(nil), selected...)
	for destination, want := range desired {
		position := destination
		for position < len(current) && current[position] != want {
			position++
		}
		if position == len(current) {
			return false, ErrUDocDatalogFragment
		}
		if position != destination {
			renamed, cause := working.Rename([]uint32{uint32(position), uint32(destination)})
			if cause != nil {
				return false, cause
			}
			if cause = working.Close(); cause != nil {
				return false, errors.Join(cause, renamed.Close())
			}
			working = renamed
			current[position], current[destination] = current[destination], current[position]
		}
	}
	delta, err := c.factory.NewEmpty(headSorts)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, delta.Close()) }()
	if err = c.result[headDeclaration].UnionFrom(working, delta); err != nil {
		return false, err
	}
	empty, err := delta.Empty()
	return !empty, err
}

func (c *datalogUDocContext) saturate(set *base.RuleSet) error {
	if c == nil || c.manager == nil || c.factory == nil || c.result == nil || set == nil {
		return ErrUDocDatalogFragment
	}
	if c.products != nil {
		return c.saturateProducts(set)
	}
	return c.saturateStratified(set)
}
