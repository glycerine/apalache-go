package fp

// Source: dl_compiler.cpp one-tail binding, FILTER_IDENTICAL and head assembly.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogUnaryTransform struct {
	constants       []externalIntervalDatalogUnaryConstant
	headConstants   []externalIntervalCartesianHeadConstant
	headUnbound     []externalIntervalCartesianHeadVariable
	headAdditions   []externalIntervalCartesianHeadAddition
	conditionAdds   []int
	duplicates      []uint32
	identical       [][]uint32
	filterRemoved   []uint32
	removed         []uint32
	renames         [][]uint32
	condition       *externalIntervalUnaryCondition
	application     ast.ASTHandle
	applicationRoot ast.ASTRef
	headApplication ast.ASTHandle
	headRoot        ast.ASTRef
}

type externalIntervalDatalogUnaryConstant struct {
	column uint32
	value  ast.ASTHandle
}

func (p *externalIntervalDatalogIdentityRule) pin(manager *ast.ASTManager) error {
	if p == nil || p.transform == nil {
		return nil
	}
	if len(p.transform.constants) != 0 && p.transform.applicationRoot == (ast.ASTRef{}) {
		root, err := manager.NewRef(p.transform.application)
		if err != nil {
			return err
		}
		p.transform.applicationRoot = root
	}
	if len(p.transform.headConstants) != 0 && p.transform.headRoot == (ast.ASTRef{}) {
		root, err := manager.NewRef(p.transform.headApplication)
		if err != nil {
			return err
		}
		p.transform.headRoot = root
	}
	return p.transform.condition.prepare(manager)
}

func (p *externalIntervalDatalogIdentityRule) close(manager *ast.ASTManager) error {
	if p == nil || p.transform == nil {
		return nil
	}
	if err := p.transform.condition.close(manager); err != nil {
		return err
	}
	if p.transform.applicationRoot == (ast.ASTRef{}) {
		if p.transform.headRoot == (ast.ASTRef{}) {
			return nil
		}
	} else if err := manager.DelRef(&p.transform.applicationRoot); err != nil {
		return err
	}
	if p.transform.headRoot != (ast.ASTRef{}) {
		return manager.DelRef(&p.transform.headRoot)
	}
	return nil
}

func sameExternalIntervalUnaryTransform(left, right *externalIntervalDatalogUnaryTransform) bool {
	if left == nil || right == nil {
		return left == right
	}
	if !sameExternalIntervalUnaryCondition(left.condition, right.condition) {
		return false
	}
	if left.headApplication != right.headApplication || !externalIntervalColumnsEqual(left.filterRemoved, right.filterRemoved) || !externalIntervalColumnsEqual(left.removed, right.removed) || len(left.constants) != len(right.constants) || len(left.headConstants) != len(right.headConstants) || len(left.headUnbound) != len(right.headUnbound) || len(left.headAdditions) != len(right.headAdditions) || len(left.conditionAdds) != len(right.conditionAdds) || !externalIntervalColumnsEqual(left.duplicates, right.duplicates) || len(left.identical) != len(right.identical) || len(left.renames) != len(right.renames) {
		return false
	}
	for i := range left.constants {
		if left.constants[i] != right.constants[i] {
			return false
		}
	}
	for i := range left.headConstants {
		if left.headConstants[i].sort != right.headConstants[i].sort || left.headConstants[i].value != right.headConstants[i].value {
			return false
		}
	}
	for i := range left.headUnbound {
		if left.headUnbound[i] != right.headUnbound[i] {
			return false
		}
	}
	for i := range left.headAdditions {
		if left.headAdditions[i] != right.headAdditions[i] {
			return false
		}
	}
	for i := range left.conditionAdds {
		if left.conditionAdds[i] != right.conditionAdds[i] {
			return false
		}
	}
	for i := range left.identical {
		if !externalIntervalColumnsEqual(left.identical[i], right.identical[i]) {
			return false
		}
	}
	for i := range left.renames {
		if !externalIntervalColumnsEqual(left.renames[i], right.renames[i]) {
			return false
		}
	}
	return true
}

func sameExternalIntervalUnaryCondition(left, right *externalIntervalUnaryCondition) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(left.source) != len(right.source) || len(left.bindings) != len(right.bindings) {
		return false
	}
	for i := range left.source {
		if left.source[i] != right.source[i] {
			return false
		}
	}
	for i := range left.bindings {
		if left.bindings[i] != right.bindings[i] {
			return false
		}
	}
	return true
}

func (e *PropositionalEngine) externalIntervalUnaryPlan(rule *base.Rule) (headDecl, tailDecl ast.ASTHandle, transform *externalIntervalDatalogUnaryTransform, err error) {
	return e.externalIntervalCompiler().externalIntervalUnaryPlan(rule)
}

func (c *externalIntervalDatalogCompiler) externalIntervalUnaryPlan(rule *base.Rule) (headDecl, tailDecl ast.ASTHandle, transform *externalIntervalDatalogUnaryTransform, err error) {
	fail := func(cause error) (ast.ASTHandle, ast.ASTHandle, *externalIntervalDatalogUnaryTransform, error) {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, cause
	}
	total, err := rule.TailSize()
	if err != nil {
		return fail(err)
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return fail(err)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return fail(err)
	}
	if total < 1 || positive != 1 || uninterpreted != 1 {
		return fail(ErrExternalIntervalDatalogFragment)
	}
	head, err := rule.Head()
	if err != nil {
		return fail(err)
	}
	tail, negated, err := rule.Tail(0)
	if err != nil {
		return fail(err)
	}
	if negated {
		return fail(ErrExternalIntervalDatalogFragment)
	}
	variables := make([][]uint32, 2)
	sorts := make([][]ast.ASTHandle, 2)
	plan := &externalIntervalDatalogUnaryTransform{}
	headArguments := []externalIntervalCartesianArgument(nil)
	headSorts := []ast.ASTHandle(nil)
	for index, application := range []ast.ASTHandle{head, tail} {
		declaration, cause := c.manager.GetDecl(application)
		if cause != nil {
			return fail(cause)
		}
		state := c.predicates[declaration]
		if state == nil || c.queryAuxiliary[declaration] || !isExternalIntervalRepresentation(state.kinds) {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		count, cause := c.manager.GetNumArgs(application)
		if cause != nil {
			return fail(cause)
		}
		if int(count) != len(state.sorts) {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		if index == 0 {
			headDecl = declaration
			headArguments = make([]externalIntervalCartesianArgument, count)
			headSorts = append([]ast.ASTHandle(nil), state.sorts...)
			plan.headApplication = head
		} else {
			tailDecl = declaration
		}
		for column := uint32(0); column < count; column++ {
			argument, cause := c.manager.GetArg(application, column)
			if cause != nil {
				return fail(cause)
			}
			kind, cause := c.manager.GetKind(argument)
			if cause != nil {
				return fail(cause)
			}
			sort, cause := c.manager.GetSort(argument)
			if cause != nil {
				return fail(cause)
			}
			if sort != state.sorts[column] {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			if kind != ast.ASTVar {
				isValue, cause := c.manager.IsValue(argument)
				if cause != nil {
					return fail(cause)
				}
				if !isValue {
					return fail(ErrExternalIntervalDatalogFragment)
				}
				_, _, numeral, cause := c.manager.ArithmeticNumeralText(argument)
				if cause != nil {
					return fail(cause)
				}
				if !numeral {
					return fail(ErrExternalIntervalDatalogFragment)
				}
				if index == 0 {
					headArguments[column].numeral = argument
				} else {
					// Earlier constants have already been projected away; the
					// source instruction selects at the number of prior variables.
					plan.constants = append(plan.constants, externalIntervalDatalogUnaryConstant{column: uint32(len(variables[index])), value: argument})
					plan.application = tail
				}
				continue
			}
			variable, cause := c.manager.GetVarIndex(argument)
			if cause != nil {
				return fail(cause)
			}
			if index == 0 {
				headArguments[column].variable = variable
			} else {
				variables[index] = append(variables[index], variable)
				sorts[index] = append(sorts[index], sort)
			}
		}
	}
	columns := make(map[uint32][]uint32)
	for column, variable := range variables[1] {
		previous := columns[variable]
		if len(previous) != 0 && sorts[1][previous[0]] != sorts[1][column] {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		columns[variable] = append(previous, uint32(column))
	}
	order := make([]uint32, 0, len(columns))
	for variable := range columns {
		order = append(order, variable)
	}
	// Source norm_vars compacts these variable-only rules. The unsigned map
	// therefore visits dense variable IDs in numeric order, not column order.
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, variable := range order {
		if group := columns[variable]; len(group) > 1 {
			plan.identical = append(plan.identical, group)
		}
	}
	wanted := make(map[uint32]bool)
	unsafeSorts := make(map[uint32]ast.ASTHandle)
	for column, argument := range headArguments {
		if !argument.numeral.IsNull() {
			continue
		}
		variable := argument.variable
		wanted[variable] = true
		group := columns[variable]
		if len(group) == 0 {
			if sort, exists := unsafeSorts[variable]; exists && sort != headSorts[column] {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			unsafeSorts[variable] = headSorts[column]
			continue
		}
		if sorts[1][group[len(group)-1]] != headSorts[column] {
			return fail(ErrExternalIntervalDatalogFragment)
		}
	}
	if total > 1 {
		boolean, cause := c.manager.MkBoolSort()
		if cause != nil {
			return fail(cause)
		}
		used, cause := ast.NewASTUsedVars(c.manager)
		if cause != nil {
			return fail(cause)
		}
		condition := &externalIntervalUnaryCondition{}
		conditionOnly := make(map[uint32]bool)
		for index := 1; index < total; index++ {
			guard, negated, cause := rule.Tail(index)
			if cause != nil {
				return fail(cause)
			}
			if negated {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			kind, cause := c.manager.GetKind(guard)
			if cause != nil {
				return fail(cause)
			}
			guardSort, cause := c.manager.GetSort(guard)
			if cause != nil {
				return fail(cause)
			}
			if kind != ast.ASTApp || guardSort != boolean {
				return fail(ErrExternalIntervalDatalogFragment)
			}
			if cause := used.Collect(guard); cause != nil {
				return fail(cause)
			}
			for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
				variableSort := used.Contains(variable)
				if variableSort.IsNull() {
					continue
				}
				group := columns[variable]
				if len(group) == 0 {
					if headSort, headVariable := unsafeSorts[variable]; headVariable && headSort != variableSort {
						return fail(ErrExternalIntervalDatalogFragment)
					}
					if prior, found := unsafeSorts[variable]; found && prior != variableSort {
						return fail(ErrExternalIntervalDatalogFragment)
					}
					if !conditionOnly[variable] {
						column := uint32(len(variables[1]))
						index := len(plan.headUnbound)
						plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: variableSort})
						plan.conditionAdds = append(plan.conditionAdds, index)
						variables[1] = append(variables[1], variable)
						sorts[1] = append(sorts[1], variableSort)
						columns[variable] = []uint32{column}
						conditionOnly[variable] = true
						order = append(order, variable)
						sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
					}
				} else if sorts[1][group[len(group)-1]] != variableSort {
					return fail(ErrExternalIntervalDatalogFragment)
				}
			}
			condition.source = append(condition.source, guard)
		}
		for _, variable := range order {
			group := columns[variable]
			column := group[len(group)-1]
			condition.bindings = append(condition.bindings, externalIntervalUnaryConditionBinding{variable: variable, column: column, sort: sorts[1][column]})
		}
		plan.condition = condition
		// The fused filter removes every occurrence of variables absent
		// from the head. Its guard still uses the original column binding;
		// duplicate surviving columns are removed later by head assembly.
		var filtered []uint32
		for column, variable := range variables[1] {
			if !wanted[variable] {
				plan.filterRemoved = append(plan.filterRemoved, uint32(column))
			} else {
				filtered = append(filtered, variable)
			}
		}
		if len(plan.filterRemoved) != 0 {
			variables[1] = filtered
			columns = make(map[uint32][]uint32)
			for column, variable := range filtered {
				columns[variable] = append(columns[variable], uint32(column))
			}
		}
	}
	surviving := make(map[uint32]uint32)
	for column, variable := range variables[1] {
		group := columns[variable]
		if !wanted[variable] || group[len(group)-1] != uint32(column) {
			plan.removed = append(plan.removed, uint32(column))
		} else {
			surviving[variable] = uint32(len(surviving))
		}
	}
	headSources := make([]uint32, len(headArguments))
	width := uint32(len(surviving))
	unsafeSources := make(map[uint32]uint32, len(unsafeSorts))
	for column, argument := range headArguments {
		if !argument.numeral.IsNull() {
			index := len(plan.headConstants)
			plan.headConstants = append(plan.headConstants, externalIntervalCartesianHeadConstant{sort: headSorts[column], value: argument.numeral})
			plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{index: index})
			headSources[column] = width
			width++
			continue
		}
		if _, found := columns[argument.variable]; found {
			headSources[column] = surviving[argument.variable]
			continue
		}
		if source, found := unsafeSources[argument.variable]; found {
			headSources[column] = source
			continue
		}
		sort := unsafeSorts[argument.variable]
		if sort.IsNull() {
			return fail(ErrExternalIntervalDatalogFragment)
		}
		index := len(plan.headUnbound)
		plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: sort})
		plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{total: true, index: index})
		unsafeSources[argument.variable] = width
		headSources[column] = width
		width++
	}
	permutation := make([]uint32, len(headArguments))
	usedSources := make(map[uint32]bool)
	for column, source := range headSources {
		if usedSources[source] {
			plan.duplicates = append(plan.duplicates, source)
			permutation[column] = width
			width++
		} else {
			usedSources[source] = true
			permutation[column] = source
		}
	}
	for column := range permutation {
		if permutation[column] == uint32(column) {
			continue
		}
		var cycle []uint32
		next := uint32(column)
		for {
			cycle = append(cycle, next)
			previous := next
			next = permutation[previous]
			permutation[previous] = previous
			if next == uint32(column) {
				break
			}
		}
		// As in two-tail assembly, transpositions implement the compiler's
		// permutation with the external declaration's opposite cycle direction.
		for i := 0; i+1 < len(cycle); i++ {
			plan.renames = append(plan.renames, []uint32{cycle[i], cycle[i+1]})
		}
	}
	if len(plan.constants)+len(plan.headConstants)+len(plan.headUnbound)+len(plan.duplicates)+len(plan.identical)+len(plan.removed)+len(plan.renames) != 0 || plan.condition != nil {
		transform = plan
	}
	return headDecl, tailDecl, transform, nil
}

func (a *externalIntervalDatalogActivation) setUnaryPending(failure memory.ConstructionFailure, plan *externalIntervalDatalogIdentityRule, completes bool) {
	a.pending, a.pendingCompletesRule = failure, completes
	a.pendingIdentity, a.pendingGround, a.pendingCartesian = nil, nil, nil
	if completes {
		a.pendingIdentity = plan
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalUnary(plan *externalIntervalDatalogIdentityRule, transform *externalIntervalDatalogUnaryTransform) error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{plan.head, plan.tail})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.setUnaryPending(pending, plan, false)
		}
		return err
	}
	owner := &externalIntervalDatalogCartesianContinuation{registers: registers, runtime: registers.runtime}
	rollback := func(cause error) error {
		failure := finishExternalIntervalDatalogCartesianFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(failure, &pending) {
			a.setUnaryPending(pending, plan, false)
		}
		return failure
	}
	finish := func() error {
		owner.publish = true
		if err := owner.resume(); err != nil {
			failure := newExternalIntervalDatalogCartesianFailure(err, owner)
			a.setUnaryPending(failure, plan, true)
			return failure
		}
		return nil
	}
	finishEmpty := func() error {
		if len(transform.headAdditions) == 0 {
			for _, index := range transform.conditionAdds {
				if _, err := registers.totalRelation(plan.head, transform.headUnbound[index].sort); err != nil {
					return rollback(err)
				}
			}
			return finish()
		}
		if err := owner.closeProducts(); err != nil {
			return rollback(err)
		}
		if err := registers.initializeHeadAssembly(plan.head, transform.headAdditions, transform.headConstants, transform.headUnbound); err != nil {
			return rollback(err)
		}
		for _, index := range transform.conditionAdds {
			if _, err := registers.totalRelation(plan.head, transform.headUnbound[index].sort); err != nil {
				return rollback(err)
			}
		}
		return finish()
	}
	current, err := registers.relation(plan.tail)
	if err != nil {
		return rollback(err)
	}
	if current == nil {
		return finishEmpty()
	}
	for _, constant := range transform.constants {
		next, err := selectExternalIntervalUnary(current, constant.column, constant.value, registers.runtime)
		if err != nil {
			return rollback(err)
		}
		if err := owner.replaceTemporary(next); err != nil {
			return rollback(err)
		}
		current = next
		// The default source operation always projects after filtering.
		// Only the completed SELECT_EQUAL_AND_PROJECT tests emptiness.
		empty, err := current.Empty()
		if err != nil {
			return rollback(err)
		}
		if empty {
			return finishEmpty()
		}
	}
	if len(transform.identical) != 0 {
		if owner.temporary == nil {
			owner.temporary, err = current.Clone()
			if err != nil {
				return rollback(err)
			}
			current = owner.temporary
		}
		for _, columns := range transform.identical {
			if err := current.FilterIdentical(columns); err != nil {
				return rollback(err)
			}
			empty, err := current.Empty()
			if err != nil {
				return rollback(err)
			}
			if empty {
				return finishEmpty()
			}
		}
	}
	if len(transform.conditionAdds) != 0 {
		if owner.temporary == nil {
			owner.temporary, err = current.Clone()
			if err != nil {
				return rollback(err)
			}
			current = owner.temporary
		}
		for _, index := range transform.conditionAdds {
			empty, cause := owner.appendHeadTotal(plan.head, transform.headUnbound[index])
			if cause != nil {
				return rollback(cause)
			}
			current = owner.temporary
			if empty {
				return finishEmpty()
			}
		}
	}
	if transform.condition != nil {
		condition, err := transform.condition.handle(a.manager)
		if err != nil {
			return rollback(err)
		}
		if len(transform.filterRemoved) != 0 {
			next, err := filterProjectExternalIntervalUnary(current, condition, transform.filterRemoved, registers.runtime)
			if err != nil {
				return rollback(err)
			}
			if err := owner.replaceTemporary(next); err != nil {
				return rollback(err)
			}
			current = next
		} else {
			if owner.temporary == nil {
				owner.temporary, err = current.Clone()
				if err != nil {
					return rollback(err)
				}
				current = owner.temporary
			}
			if err := current.FilterInterpreted(condition); err != nil {
				return rollback(err)
			}
		}
		empty, err := current.Empty()
		if err != nil {
			return rollback(err)
		}
		if empty {
			return finishEmpty()
		}
	}
	if len(transform.removed) != 0 {
		next, err := current.Project(transform.removed)
		if err != nil {
			return rollback(err)
		}
		if err := owner.replaceTemporary(next); err != nil {
			return rollback(err)
		}
		current = next
	}
	if len(transform.headAdditions) != 0 {
		if owner.temporary == nil {
			owner.temporary, err = current.Clone()
			if err != nil {
				return rollback(err)
			}
			current = owner.temporary
		}
		for _, addition := range transform.headAdditions {
			var empty bool
			if addition.total {
				empty, err = owner.appendHeadTotal(plan.head, transform.headUnbound[addition.index])
			} else {
				empty, err = owner.appendHeadConstant(plan.head, transform.headConstants[addition.index])
			}
			if err != nil {
				return rollback(err)
			}
			current = owner.temporary
			if empty {
				return finishEmpty()
			}
		}
	}
	if len(transform.duplicates) != 0 {
		if owner.temporary == nil {
			owner.temporary, err = current.Clone()
			if err != nil {
				return rollback(err)
			}
			current = owner.temporary
		}
		for _, column := range transform.duplicates {
			empty, cause := owner.duplicateColumn(column)
			if cause != nil {
				return rollback(cause)
			}
			current = owner.temporary
			if empty {
				return finishEmpty()
			}
		}
	}
	for _, cycle := range transform.renames {
		next, err := current.Rename(cycle)
		if err != nil {
			return rollback(err)
		}
		if err := owner.replaceTemporary(next); err != nil {
			return rollback(err)
		}
		current = next
	}
	target, err := registers.ensureRelation(plan.head)
	if err != nil {
		return rollback(err)
	}
	if err := target.Union(current, nil, false); err != nil {
		if !rel.ExternalAssignmentCommitted(err) {
			return rollback(err)
		}
		var nested memory.ConstructionFailure
		if !errors.As(err, &nested) {
			return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
		}
		owner.nested, owner.publish = nested, true
		failure := newExternalIntervalDatalogCartesianFailure(err, owner)
		a.setUnaryPending(failure, plan, true)
		return failure
	}
	return finish()
}
