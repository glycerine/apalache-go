package fp

// Source: src/muz/rel/dl_compiler.cpp two-tail rule lowering and
// src/muz/rel/dl_instruction.cpp LOAD, JOIN_PROJECT, RENAME, UNION, and STORE.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

// externalIntervalDatalogCartesianRule lowers two positive tails with
// numerals, distinct variable columns and bound interpreted guards to a head of
// bound variables and numerals. It also covers the original Cartesian rule as
// the no-key/no-projection case.
type externalIntervalDatalogCartesianRule struct {
	head          ast.ASTHandle
	left          ast.ASTHandle
	right         ast.ASTHandle
	leftColumns   []uint32
	rightColumns  []uint32
	identical     [][]uint32
	removed       []uint32
	filterRemoved []uint32
	headRemoved   []uint32
	constants     []externalIntervalCartesianConstant
	headConstants []externalIntervalCartesianHeadConstant
	headUnbound   []externalIntervalCartesianHeadVariable
	headAdditions []externalIntervalCartesianHeadAddition
	conditionAdds []int
	duplicates    []uint32
	renames       [][]uint32
	condition     *externalIntervalUnaryCondition
}

type externalIntervalCartesianArgument struct {
	variable uint32
	numeral  ast.ASTHandle
}

type externalIntervalCartesianConstant struct {
	column uint32
	value  ast.ASTHandle
	root   ast.ASTRef
}

type externalIntervalCartesianHeadConstant struct {
	sort, value ast.ASTHandle
	root        ast.ASTRef
}

type externalIntervalCartesianHeadVariable struct{ sort ast.ASTHandle }

type externalIntervalCartesianHeadAddition struct {
	total bool
	index int
}

func (p *externalIntervalDatalogCartesianRule) headAssemblyAdditions() []externalIntervalCartesianHeadAddition {
	if len(p.headAdditions) != 0 || len(p.headConstants) == 0 {
		return p.headAdditions
	}
	additions := make([]externalIntervalCartesianHeadAddition, len(p.headConstants))
	for index := range additions {
		additions[index].index = index
	}
	return additions
}

func sameExternalIntervalCartesianRule(left, right *externalIntervalDatalogCartesianRule) bool {
	if left == nil || right == nil || left.head != right.head || left.left != right.left || left.right != right.right ||
		!externalIntervalColumnsEqual(left.leftColumns, right.leftColumns) ||
		!externalIntervalColumnsEqual(left.rightColumns, right.rightColumns) ||
		!sameExternalIntervalColumnGroups(left.identical, right.identical) ||
		!externalIntervalColumnsEqual(left.removed, right.removed) ||
		!externalIntervalColumnsEqual(left.filterRemoved, right.filterRemoved) ||
		!externalIntervalColumnsEqual(left.headRemoved, right.headRemoved) || len(left.constants) != len(right.constants) ||
		len(left.headConstants) != len(right.headConstants) || len(left.headUnbound) != len(right.headUnbound) ||
		len(left.headAdditions) != len(right.headAdditions) || len(left.conditionAdds) != len(right.conditionAdds) ||
		!externalIntervalColumnsEqual(left.duplicates, right.duplicates) ||
		!sameExternalIntervalUnaryCondition(left.condition, right.condition) || len(left.renames) != len(right.renames) {
		return false
	}
	for i, constant := range left.constants {
		other := right.constants[i]
		if constant.column != other.column || constant.value != other.value {
			return false
		}
	}
	for i, constant := range left.headConstants {
		other := right.headConstants[i]
		if constant.sort != other.sort || constant.value != other.value {
			return false
		}
	}
	for i, variable := range left.headUnbound {
		if variable.sort != right.headUnbound[i].sort {
			return false
		}
	}
	for i, addition := range left.headAdditions {
		if addition != right.headAdditions[i] {
			return false
		}
	}
	for i, addition := range left.conditionAdds {
		if addition != right.conditionAdds[i] {
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

func sameExternalIntervalColumnGroups(left, right [][]uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !externalIntervalColumnsEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func externalIntervalColumnsEqual(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for i, column := range left {
		if column != right[i] {
			return false
		}
	}
	return true
}

func (e *PropositionalEngine) externalIntervalCartesianRule(rule *base.Rule) (*externalIntervalDatalogCartesianRule, error) {
	return e.externalIntervalCompiler().externalIntervalCartesianRule(rule)
}

func (c *externalIntervalDatalogCompiler) externalIntervalCartesianRule(rule *base.Rule) (*externalIntervalDatalogCartesianRule, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	if positive != 2 || uninterpreted != 2 || total < 2 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	// The source compiler consumes normalized rule tail order directly.
	left, leftNegated, err := rule.Tail(0)
	if err != nil {
		return nil, err
	}
	right, rightNegated, err := rule.Tail(1)
	if err != nil {
		return nil, err
	}
	if leftNegated || rightNegated {
		return nil, ErrExternalIntervalDatalogFragment
	}
	plan := &externalIntervalDatalogCartesianRule{}
	applications := []ast.ASTHandle{head, left, right}
	declarations := []*ast.ASTHandle{&plan.head, &plan.left, &plan.right}
	arguments := make([][]externalIntervalCartesianArgument, 3)
	sorts := make([][]ast.ASTHandle, 3)
	for index, application := range applications {
		declaration, cause := c.manager.GetDecl(application)
		if cause != nil {
			return nil, cause
		}
		state := c.predicates[declaration]
		if state == nil || c.queryAuxiliary[declaration] || !isExternalIntervalRepresentation(state.kinds) {
			return nil, ErrExternalIntervalDatalogFragment
		}
		count, cause := c.manager.GetNumArgs(application)
		if cause != nil {
			return nil, cause
		}
		if int(count) != len(state.sorts) || index != 0 && count == 0 {
			return nil, ErrExternalIntervalDatalogFragment
		}
		*declarations[index] = declaration
		sorts[index] = state.sorts
		arguments[index] = make([]externalIntervalCartesianArgument, count)
		seen := make(map[uint32][]uint32)
		groupIndex := make(map[uint32]int)
		for column := uint32(0); column < count; column++ {
			argument, cause := c.manager.GetArg(application, column)
			if cause != nil {
				return nil, cause
			}
			kind, cause := c.manager.GetKind(argument)
			if cause != nil {
				return nil, cause
			}
			sort, cause := c.manager.GetSort(argument)
			if cause != nil {
				return nil, cause
			}
			if sort != state.sorts[column] {
				return nil, ErrExternalIntervalDatalogFragment
			}
			if kind != ast.ASTVar {
				isValue, cause := c.manager.IsValue(argument)
				if cause != nil {
					return nil, cause
				}
				if !isValue {
					return nil, ErrExternalIntervalDatalogFragment
				}
				_, _, numeral, cause := c.manager.ArithmeticNumeralText(argument)
				if cause != nil {
					return nil, cause
				}
				if !numeral {
					return nil, ErrExternalIntervalDatalogFragment
				}
				arguments[index][column].numeral = argument
				continue
			}
			variable, cause := c.manager.GetVarIndex(argument)
			if cause != nil {
				return nil, cause
			}
			if index != 0 {
				if prior := seen[variable]; len(prior) > 0 {
					group, found := groupIndex[variable]
					if !found {
						group = len(plan.identical)
						groupIndex[variable] = group
						columns := append([]uint32(nil), prior...)
						if index == 2 {
							for i := range columns {
								columns[i] += uint32(len(arguments[1]))
							}
						}
						plan.identical = append(plan.identical, columns)
					}
					columnIndex := column
					if index == 2 {
						columnIndex += uint32(len(arguments[1]))
					}
					plan.identical[group] = append(plan.identical[group], columnIndex)
				}
				seen[variable] = append(seen[variable], column)
			}
			if index == 0 || len(seen[variable]) == 0 {
				seen[variable] = []uint32{column}
			}
			arguments[index][column].variable = variable
		}
	}
	leftArguments, rightArguments := arguments[1], arguments[2]
	for first, argument := range leftArguments {
		if !argument.numeral.IsNull() {
			continue
		}
		for second, other := range rightArguments {
			if other.numeral.IsNull() && argument.variable == other.variable {
				if sorts[1][first] != sorts[2][second] {
					return nil, ErrExternalIntervalDatalogFragment
				}
				plan.leftColumns = append(plan.leftColumns, uint32(first))
				plan.rightColumns = append(plan.rightColumns, uint32(second))
			}
		}
	}
	joinedArguments := append(append([]externalIntervalCartesianArgument(nil), leftArguments...), rightArguments...)
	joinedSorts := append(append([]ast.ASTHandle(nil), sorts[1]...), sorts[2]...)
	last := make(map[uint32]uint32)
	remaining := make(map[uint32]uint32)
	for column, argument := range joinedArguments {
		if argument.numeral.IsNull() {
			last[argument.variable] = uint32(column)
			remaining[argument.variable]++
		}
	}
	wanted := make(map[uint32]bool)
	needed := make(map[uint32]uint32)
	unsafeSorts := make(map[uint32]ast.ASTHandle)
	conditionOnlySorts := make(map[uint32]ast.ASTHandle)
	for column, argument := range arguments[0] {
		if !argument.numeral.IsNull() {
			continue
		}
		variable := argument.variable
		wanted[variable] = true
		source, found := last[variable]
		if !found {
			if sort, exists := unsafeSorts[variable]; exists && sort != sorts[0][column] {
				return nil, ErrExternalIntervalDatalogFragment
			}
			unsafeSorts[variable] = sorts[0][column]
			continue
		}
		if joinedSorts[source] != sorts[0][column] {
			return nil, ErrExternalIntervalDatalogFragment
		}
		needed[variable]++
	}
	if total > 2 {
		boolean, cause := c.manager.MkBoolSort()
		if cause != nil {
			return nil, cause
		}
		used, cause := ast.NewASTUsedVars(c.manager)
		if cause != nil {
			return nil, cause
		}
		condition := &externalIntervalUnaryCondition{}
		for index := 2; index < total; index++ {
			guard, negated, cause := rule.Tail(index)
			if cause != nil {
				return nil, cause
			}
			if negated {
				return nil, ErrExternalIntervalDatalogFragment
			}
			kind, cause := c.manager.GetKind(guard)
			if cause != nil {
				return nil, cause
			}
			sort, cause := c.manager.GetSort(guard)
			if cause != nil {
				return nil, cause
			}
			if kind != ast.ASTApp || sort != boolean {
				return nil, ErrExternalIntervalDatalogFragment
			}
			if cause := used.Collect(guard); cause != nil {
				return nil, cause
			}
			for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
				sort := used.Contains(variable)
				if sort.IsNull() {
					continue
				}
				column, bound := last[variable]
				if bound {
					if joinedSorts[column] != sort {
						return nil, ErrExternalIntervalDatalogFragment
					}
					if needed[variable] == 0 {
						needed[variable] = 1
					}
				} else if prior, found := unsafeSorts[variable]; found {
					if prior != sort {
						return nil, ErrExternalIntervalDatalogFragment
					}
					conditionOnlySorts[variable] = sort
				} else if prior, found := conditionOnlySorts[variable]; found {
					if prior != sort {
						return nil, ErrExternalIntervalDatalogFragment
					}
				} else {
					conditionOnlySorts[variable] = sort
				}
			}
			condition.source = append(condition.source, guard)
		}
		plan.condition = condition
	}
	// Source get_local_indexes_for_projection discards unused variables and
	// the earliest surplus copies, preserving the head's required multiplicity
	// and at least one copy for each guard variable.
	surviving := make(map[uint32]uint32)
	var projectedArguments []externalIntervalCartesianArgument
	for column, argument := range joinedArguments {
		variable := argument.variable
		if !argument.numeral.IsNull() {
			// Constants survive JOIN_PROJECT and are filtered in place at
			// fixed physical columns before any interpreted guard runs.
			plan.constants = append(plan.constants, externalIntervalCartesianConstant{
				column: uint32(len(projectedArguments)), value: argument.numeral,
			})
			projectedArguments = append(projectedArguments, argument)
		} else if remaining[variable] > needed[variable] {
			plan.removed = append(plan.removed, uint32(column))
			remaining[variable]--
		} else {
			surviving[variable] = uint32(len(projectedArguments))
			projectedArguments = append(projectedArguments, argument)
		}
	}
	var conditionOnlyVariables []uint32
	for variable := range conditionOnlySorts {
		conditionOnlyVariables = append(conditionOnlyVariables, variable)
	}
	sort.Slice(conditionOnlyVariables, func(i, j int) bool { return conditionOnlyVariables[i] < conditionOnlyVariables[j] })
	for _, variable := range conditionOnlyVariables {
		column := uint32(len(projectedArguments))
		surviving[variable] = column
		index := len(plan.headUnbound)
		plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: conditionOnlySorts[variable]})
		plan.conditionAdds = append(plan.conditionAdds, index)
		projectedArguments = append(projectedArguments, externalIntervalCartesianArgument{variable: variable})
	}
	filteredArguments := projectedArguments
	if plan.condition != nil {
		for column, argument := range projectedArguments {
			if argument.numeral.IsNull() && surviving[argument.variable] == uint32(column) {
				variable := argument.variable
				variableSort := conditionOnlySorts[variable]
				if variableSort.IsNull() {
					variableSort = joinedSorts[last[variable]]
				}
				plan.condition.bindings = append(plan.condition.bindings, externalIntervalUnaryConditionBinding{
					variable: variable, column: surviving[variable], sort: variableSort,
				})
			}
		}
		// Guard-only variables survive the initial join projection, then
		// leave through the fused filter/project. Constants stay until the
		// separate head projection, so guard bindings count their columns.
		filteredArguments = nil
		for column, argument := range projectedArguments {
			if argument.numeral.IsNull() && !wanted[argument.variable] {
				plan.filterRemoved = append(plan.filterRemoved, uint32(column))
			} else {
				filteredArguments = append(filteredArguments, argument)
			}
		}
	}
	occurrences := make(map[uint32][]uint32)
	for column, argument := range filteredArguments {
		if argument.numeral.IsNull() {
			occurrences[argument.variable] = append(occurrences[argument.variable], uint32(column))
		}
	}
	// Source head assembly consumes available copies from the back, keeping
	// the last remaining column available for later explicit duplication.
	headSources := make([]uint32, len(arguments[0]))
	headBound := make([]bool, len(arguments[0]))
	referenced := make(map[uint32]bool)
	for column, argument := range arguments[0] {
		if !argument.numeral.IsNull() {
			continue
		}
		copies := occurrences[argument.variable]
		if len(copies) == 0 {
			if _, unsafe := unsafeSorts[argument.variable]; !unsafe {
				return nil, ErrExternalIntervalDatalogFragment
			}
			continue
		}
		source := copies[len(copies)-1]
		headSources[column] = source
		headBound[column] = true
		referenced[source] = true
		if len(copies) > 1 {
			occurrences[argument.variable] = copies[:len(copies)-1]
		}
	}
	projectedColumns := make(map[uint32]uint32)
	for column := range filteredArguments {
		if !referenced[uint32(column)] {
			plan.headRemoved = append(plan.headRemoved, uint32(column))
		} else {
			projectedColumns[uint32(column)] = uint32(len(projectedColumns))
		}
	}
	width := uint32(len(projectedColumns))
	unsafeSources := make(map[uint32]uint32, len(unsafeSorts))
	// Source assembly appends constants and total columns in head order before
	// duplicating repeated variables, even when those head positions interleave.
	for column, argument := range arguments[0] {
		if argument.numeral.IsNull() {
			if headBound[column] {
				headSources[column] = projectedColumns[headSources[column]]
			} else if source, found := unsafeSources[argument.variable]; found {
				headSources[column] = source
			} else {
				sort := unsafeSorts[argument.variable]
				if sort.IsNull() {
					return nil, ErrExternalIntervalDatalogFragment
				}
				index := len(plan.headUnbound)
				plan.headUnbound = append(plan.headUnbound, externalIntervalCartesianHeadVariable{sort: sort})
				plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{total: true, index: index})
				unsafeSources[argument.variable] = width
				headSources[column] = width
				width++
			}
		} else {
			index := len(plan.headConstants)
			plan.headConstants = append(plan.headConstants, externalIntervalCartesianHeadConstant{
				sort: sorts[0][column], value: argument.numeral,
			})
			plan.headAdditions = append(plan.headAdditions, externalIntervalCartesianHeadAddition{index: index})
			headSources[column] = width
			width++
		}
	}
	permutation := make([]uint32, len(headSources))
	usedColumns := make(map[uint32]bool)
	for column, source := range headSources {
		if usedColumns[source] {
			plan.duplicates = append(plan.duplicates, source)
			permutation[column] = width
			width++
		} else {
			usedColumns[source] = true
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
		// The compiler's source permutation rotates left, while the source
		// datalog declaration and vector-relation rename rotate right. Those
		// disagree for cycles longer than two. Lower the compiler permutation
		// to adjacent transpositions, whose signatures, external callbacks and
		// interval values agree, instead of changing either shared primitive.
		for i := 0; i+1 < len(cycle); i++ {
			plan.renames = append(plan.renames, []uint32{cycle[i], cycle[i+1]})
		}
	}
	return plan, nil
}

// externalIntervalDatalogCartesianContinuation owns every temporary created
// after LOAD. A committed UNION and its temporary are cleaned before STORE.
// Every frontier is retryable without replaying JOIN or UNION.
type externalIntervalDatalogCartesianContinuation struct {
	registers *externalIntervalDatalogRegisters
	temporary *rel.ExternalIntervalProduct
	retiring  *rel.ExternalIntervalProduct
	auxiliary *rel.ExternalIntervalProduct
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
	publish   bool
}

func (c *externalIntervalDatalogCartesianContinuation) resume() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if err := c.closeProducts(); err != nil {
		return err
	}
	if c.publish && c.registers != nil {
		if err := c.registers.store(); err != nil {
			var owner *externalIntervalDatalogRegisterFailure
			if errors.As(err, &owner) && owner.owner == c.registers {
				c.nested = owner
				c.registers = nil
			}
			return err
		}
		c.registers = nil
	}
	if !c.publish && c.registers != nil {
		if err := c.registers.Close(); err != nil {
			return err
		}
		c.registers = nil
	}
	return nil
}

func (c *externalIntervalDatalogCartesianContinuation) replaceTemporary(next *rel.ExternalIntervalProduct) error {
	c.retiring, c.temporary = c.temporary, next
	if c.retiring != nil {
		if err := c.retiring.Close(); err != nil {
			return err
		}
		c.retiring = nil
	}
	return nil
}

type externalIntervalDatalogCartesianFailure struct {
	original, cleanup error
	reported          error
	text              string
	owner             *externalIntervalDatalogCartesianContinuation
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogCartesianFailure)(nil)

func (f *externalIntervalDatalogCartesianFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogCartesianFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogCartesianFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogCartesianFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogCartesianFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogCartesianFailure) Cleanup() error {
	f.cleanup = f.owner.resume()
	return f.cleanup
}

func newExternalIntervalDatalogCartesianFailure(cause error, owner *externalIntervalDatalogCartesianContinuation) *externalIntervalDatalogCartesianFailure {
	return &externalIntervalDatalogCartesianFailure{
		original: cause, cleanup: cause, reported: cause, text: cause.Error(), owner: owner,
	}
}

func finishExternalIntervalDatalogCartesianFailure(cause error, owner *externalIntervalDatalogCartesianContinuation) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.resume()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogCartesianFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

func (a *externalIntervalDatalogActivation) setCartesianPending(failure memory.ConstructionFailure, completes bool) {
	a.pending = failure
	a.pendingCompletesRule = completes
	a.pendingIdentity = nil
	a.pendingGround = nil
	a.pendingCartesian = nil
	if completes {
		a.pendingCartesian = a.cartesian
	}
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalCartesian() error {
	registers, err := a.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{
		a.cartesian.head, a.cartesian.left, a.cartesian.right,
	})
	if err != nil {
		var pending memory.ConstructionFailure
		if errors.As(err, &pending) {
			a.setCartesianPending(pending, false)
		}
		return err
	}
	owner := &externalIntervalDatalogCartesianContinuation{
		registers: registers,
		runtime:   registers.runtime,
	}
	rollback := func(cause error) error {
		finished := finishExternalIntervalDatalogCartesianFailure(cause, owner)
		var pending memory.ConstructionFailure
		if errors.As(finished, &pending) {
			a.setCartesianPending(pending, false)
		}
		owner = nil
		return finished
	}
	finish := func() error {
		owner.publish = true
		if cause := owner.resume(); cause != nil {
			failure := newExternalIntervalDatalogCartesianFailure(cause, owner)
			a.setCartesianPending(failure, true)
			return failure
		}
		return nil
	}
	finishEmpty := func() error {
		if len(a.cartesian.headAssemblyAdditions()) == 0 {
			for _, index := range a.cartesian.conditionAdds {
				if _, cause := registers.totalRelation(a.cartesian.head, a.cartesian.headUnbound[index].sort); cause != nil {
					return rollback(cause)
				}
			}
			return finish()
		}
		// Source make_empty retires the current relation before later
		// unconditional singleton instructions execute. An empty producer
		// still determines first-use cache ownership for later rules.
		if cause := owner.closeProducts(); cause != nil {
			return rollback(cause)
		}
		if cause := registers.initializeCartesianHeadConstants(a.cartesian); cause != nil {
			return rollback(cause)
		}
		for _, index := range a.cartesian.conditionAdds {
			if _, cause := registers.totalRelation(a.cartesian.head, a.cartesian.headUnbound[index].sort); cause != nil {
				return rollback(cause)
			}
		}
		return finish()
	}

	left, err := registers.relationAt(a.cartesian.left, 0)
	if err != nil {
		return rollback(err)
	}
	right, err := registers.relationAt(a.cartesian.right, 1)
	if err != nil {
		return rollback(err)
	}
	if left == nil || right == nil {
		return finishEmpty()
	}
	if left != nil && right != nil {
		joined, cause := rel.JoinExternalIntervalProducts(left, right, a.cartesian.leftColumns, a.cartesian.rightColumns)
		if cause != nil {
			return rollback(cause)
		}
		owner.temporary = joined
		for _, columns := range a.cartesian.identical {
			if cause = owner.temporary.FilterIdentical(columns); cause != nil {
				return rollback(cause)
			}
		}
		if len(a.cartesian.removed) != 0 {
			projected, cause := owner.temporary.Project(a.cartesian.removed)
			if cause != nil {
				return rollback(cause)
			}
			if cause = owner.replaceTemporary(projected); cause != nil {
				return rollback(cause)
			}
		}
		// instr_join_project canonicalizes an empty result before any head
		// assembly. In particular, disjoint interval keys must not reach UNION.
		empty, cause := owner.temporary.Empty()
		if cause != nil {
			return rollback(cause)
		}
		if empty {
			return finishEmpty()
		}
		for _, constant := range a.cartesian.constants {
			if cause = owner.temporary.FilterEqual(constant.column, constant.value); cause != nil {
				return rollback(cause)
			}
			empty, cause = owner.temporary.Empty()
			if cause != nil {
				return rollback(cause)
			}
			if empty {
				return finishEmpty()
			}
		}
		for _, index := range a.cartesian.conditionAdds {
			empty, cause = owner.appendHeadTotal(a.cartesian.head, a.cartesian.headUnbound[index])
			if cause != nil {
				return rollback(cause)
			}
			if empty {
				return finishEmpty()
			}
		}
		if a.cartesian.condition != nil {
			condition, cause := a.cartesian.condition.handle(a.manager)
			if cause != nil {
				return rollback(cause)
			}
			if len(a.cartesian.filterRemoved) != 0 {
				projected, cause := filterProjectExternalIntervalUnary(owner.temporary, condition, a.cartesian.filterRemoved, registers.runtime)
				if cause != nil {
					return rollback(cause)
				}
				if cause = owner.replaceTemporary(projected); cause != nil {
					return rollback(cause)
				}
			} else {
				// JOIN already owns its result, so this source branch filters
				// directly without a clone before head assembly.
				if cause = owner.temporary.FilterInterpreted(condition); cause != nil {
					return rollback(cause)
				}
			}
			empty, cause = owner.temporary.Empty()
			if cause != nil {
				return rollback(cause)
			}
			if empty {
				return finishEmpty()
			}
		}
		if len(a.cartesian.headRemoved) != 0 {
			projected, cause := owner.temporary.Project(a.cartesian.headRemoved)
			if cause != nil {
				return rollback(cause)
			}
			if cause = owner.replaceTemporary(projected); cause != nil {
				return rollback(cause)
			}
		}
		for _, addition := range a.cartesian.headAssemblyAdditions() {
			var empty bool
			if addition.total {
				empty, cause = owner.appendHeadTotal(a.cartesian.head, a.cartesian.headUnbound[addition.index])
			} else {
				empty, cause = owner.appendHeadConstant(a.cartesian.head, a.cartesian.headConstants[addition.index])
			}
			if cause != nil {
				return rollback(cause)
			}
			if empty {
				return finishEmpty()
			}
		}
		for _, column := range a.cartesian.duplicates {
			empty, cause := owner.duplicateColumn(column)
			if cause != nil {
				return rollback(cause)
			}
			if empty {
				return finishEmpty()
			}
		}
		for _, cycle := range a.cartesian.renames {
			renamed, cause := owner.temporary.Rename(cycle)
			if cause != nil {
				return rollback(cause)
			}
			if cause = owner.replaceTemporary(renamed); cause != nil {
				return rollback(cause)
			}
		}
		cause = registers.union(a.cartesian.head, owner.temporary)
		if cause != nil {
			if rel.ExternalAssignmentCommitted(cause) {
				var nested memory.ConstructionFailure
				if !errors.As(cause, &nested) {
					return rollback(fmt.Errorf("%w: committed union has no cleanup owner", ErrExternalIntervalDatalogFragment))
				}
				owner.nested = nested
				owner.publish = true
				failure := newExternalIntervalDatalogCartesianFailure(cause, owner)
				a.setCartesianPending(failure, true)
				return failure
			}
			return rollback(cause)
		}
	}
	return finish()
}
