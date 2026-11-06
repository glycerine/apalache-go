package fp

// Source: dl_compiler.cpp rule evaluation, specialized to positive
// interval-relation tails and the source interval plugin operations.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func ensureIntervalConditionVariables(e *IntervalDatalogEvaluator, condition ast.ASTHandle, variables map[uint32]uint32, relation *rel.IntervalRelation, sorts []ast.ASTHandle) (*rel.IntervalRelation, []ast.ASTHandle, error) {
	manager := e.manager
	if variables == nil {
		return nil, nil, ErrIntervalDatalogFragment
	}
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, nil, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sortHandle := used.Contains(variable)
		if sortHandle.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		column := uint32(len(sorts))
		relation, sorts, err = e.appendFullColumn(relation, sorts, sortHandle)
		if err != nil {
			return nil, nil, err
		}
		variables[variable] = column
	}
	return relation, sorts, nil
}

func (e *IntervalDatalogEvaluator) ruleHead(rule *base.Rule) (head ast.ASTHandle, formula ast.ASTRef, resultErr error) {
	formula, err := rule.ToFormula()
	if err != nil {
		return head, formula, err
	}
	head, err = e.manager.RefGet(formula)
	if err != nil {
		return head, formula, err
	}
	if kind, cause := e.manager.GetKind(head); cause != nil {
		return head, formula, cause
	} else if kind == ast.ASTQuantifier {
		head, err = e.manager.GetQuantifierBody(head)
		if err != nil {
			return head, formula, err
		}
	}
	if op, cause := e.manager.GetDeclKind(head); cause == nil && op == ast.ASTOpImplies {
		head, err = e.manager.GetArg(head, 1)
	} else if cause != nil {
		err = cause
	}
	return head, formula, err
}

func (e *IntervalDatalogEvaluator) remapIntervalCondition(condition ast.ASTHandle, variables map[uint32]uint32, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	used, err := ast.NewASTUsedVars(e.manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(condition); err != nil {
		return result, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		if used.Contains(variable).IsNull() {
			continue
		}
		if _, found := variables[variable]; !found {
			return result, fmt.Errorf("%w: interpreted condition has an unbound variable", ErrIntervalDatalogFragment)
		}
	}
	max := uint32(0)
	for variable := range variables {
		if variable >= max {
			max = variable + 1
		}
	}
	bindings := make([]ast.ASTHandle, max)
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&roots[i]))
		}
	}()
	for variable, column := range variables {
		h, err := e.manager.MkVar(column, sorts[column])
		if err != nil {
			return result, err
		}
		root, err := e.manager.NewRef(h)
		if err != nil {
			return result, err
		}
		roots = append(roots, root)
		bindings[variable] = h
	}
	sub, err := astrewriter.NewVarSubstitution(e.manager, false)
	if sub != nil {
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = sub.Substitute(condition, bindings, result); err != nil {
		resultErr = errors.Join(err, e.manager.DelRef(&result))
		result = ast.ASTRef{}
		return result, resultErr
	}
	return result, nil
}

func (e *IntervalDatalogEvaluator) evaluatePositiveRule(rule *base.Rule, widen bool) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	if err != nil || positive == 0 {
		return false, errors.Join(err, fmt.Errorf("%w: expected a positive tail", ErrIntervalDatalogFragment))
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return false, errors.Join(err, fmt.Errorf("%w: negated predicate tail", ErrIntervalDatalogFragment))
	}
	var working *rel.IntervalRelation
	var workingSorts []ast.ASTHandle
	variables := make(map[uint32]uint32)
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return false, errors.Join(cause, fmt.Errorf("%w: invalid positive tail", ErrIntervalDatalogFragment))
		}
		tailDeclaration, cause := e.manager.GetDecl(tail)
		if cause != nil || e.relations[tailDeclaration] == nil {
			return false, errors.Join(cause, fmt.Errorf("%w: undeclared tail", ErrIntervalDatalogFragment))
		}
		tailRelation, cause := e.relations[tailDeclaration].Clone()
		if cause != nil {
			return false, cause
		}
		tailSorts := e.sorts[tailDeclaration]
		localVariables := make(map[uint32]uint32)
		arguments, cause := e.manager.GetNumArgs(tail)
		if cause != nil || int(arguments) != len(tailSorts) {
			return false, errors.Join(cause, ErrIntervalDatalogFragment)
		}
		for column := uint32(0); column < arguments; column++ {
			argument, cause := e.manager.GetArg(tail, column)
			if cause != nil {
				return false, cause
			}
			kind, cause := e.manager.GetKind(argument)
			if cause != nil {
				return false, cause
			}
			if kind == ast.ASTVar {
				variable, cause := e.manager.GetVarIndex(argument)
				if cause != nil {
					return false, cause
				}
				if prior, found := localVariables[variable]; found {
					if cause = e.filterIdentical(tailRelation, tailSorts, []uint32{prior, column}); cause != nil {
						return false, cause
					}
				} else {
					localVariables[variable] = column
				}
				continue
			}
			_, _, numeral, cause := e.manager.ArithmeticNumeralText(argument)
			if cause != nil || !numeral {
				return false, errors.Join(cause, fmt.Errorf("%w: tail argument is not a variable or numeral", ErrIntervalDatalogFragment))
			}
			if cause = e.filterEqual(tailRelation, tailSorts, column, argument); cause != nil {
				return false, cause
			}
		}
		if working == nil {
			working = tailRelation
			workingSorts = append(workingSorts, tailSorts...)
			for variable, column := range localVariables {
				variables[variable] = column
			}
			continue
		}
		sharedVariables := make([]uint32, 0, len(localVariables))
		for variable := range localVariables {
			if _, shared := variables[variable]; shared {
				sharedVariables = append(sharedVariables, variable)
			}
		}
		sort.Slice(sharedVariables, func(i, j int) bool { return sharedVariables[i] < sharedVariables[j] })
		leftColumns := make([]uint32, 0, len(sharedVariables))
		rightColumns := make([]uint32, 0, len(sharedVariables))
		for _, variable := range sharedVariables {
			right := localVariables[variable]
			if left, shared := variables[variable]; shared {
				leftColumns = append(leftColumns, left)
				rightColumns = append(rightColumns, right)
			}
		}
		offset := uint32(len(workingSorts))
		working, workingSorts, cause = e.join(working, workingSorts, tailRelation, tailSorts, leftColumns, rightColumns)
		if cause != nil {
			return false, cause
		}
		for variable, column := range localVariables {
			if _, shared := variables[variable]; !shared {
				variables[variable] = offset + column
			}
		}
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	conditionCount := tailSize - uninterpreted
	for i := uninterpreted; i < tailSize; i++ {
		condition, conditionNegated, cause := rule.Tail(i)
		if cause != nil || conditionNegated {
			return false, errors.Join(cause, ErrIntervalDatalogFragment)
		}
		// This audited slice materializes interpreted-only variables for the
		// source compiler's single-condition, one-or-two-positive-tail lowering.
		// Wider rule shapes retain their existing requirement that every
		// interpreted variable be bound by a predicate tail.
		if conditionCount == 1 && positive <= 2 {
			working, workingSorts, cause = ensureIntervalConditionVariables(e, condition, variables, working, workingSorts)
			if cause != nil {
				return false, cause
			}
		}
		mapped, cause := e.remapIntervalCondition(condition, variables, workingSorts)
		if cause != nil {
			return false, cause
		}
		mappedHandle, cause := e.manager.RefGet(mapped)
		if cause == nil {
			cause = e.filterInterpreted(working, workingSorts, mappedHandle)
		}
		cause = errors.Join(cause, e.manager.DelRef(&mapped))
		if cause != nil {
			return false, cause
		}
	}
	head, formula, err := e.ruleHead(rule)
	if formula != (ast.ASTRef{}) {
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&formula)) }()
	}
	if err != nil {
		return false, err
	}
	headDeclaration, err := e.manager.GetDecl(head)
	if err != nil || e.relations[headDeclaration] == nil {
		return false, errors.Join(err, ErrIntervalDatalogFragment)
	}
	headCount, err := e.manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	desired := make([]uint32, headCount)
	usedColumns := make(map[uint32]bool, headCount)
	headSorts := e.sorts[headDeclaration]
	if int(headCount) != len(headSorts) {
		return false, ErrIntervalDatalogFragment
	}
	for i := uint32(0); i < headCount; i++ {
		argument, cause := e.manager.GetArg(head, i)
		if cause != nil {
			return false, cause
		}
		kind, cause := e.manager.GetKind(argument)
		if cause != nil {
			return false, cause
		}
		var column uint32
		if kind == ast.ASTVar {
			variable, cause := e.manager.GetVarIndex(argument)
			if cause != nil {
				return false, cause
			}
			var found bool
			column, found = variables[variable]
			if !found {
				column = uint32(len(workingSorts))
				working, workingSorts, cause = e.appendFullColumn(working, workingSorts, headSorts[i])
				if cause != nil {
					return false, cause
				}
				variables[variable] = column
			}
		} else {
			valueSort, sortErr := e.manager.GetSort(argument)
			_, _, numeral, numeralErr := e.manager.ArithmeticNumeralText(argument)
			cause = errors.Join(sortErr, numeralErr)
			if cause != nil || !numeral || valueSort != headSorts[i] {
				return false, errors.Join(cause, fmt.Errorf("%w: non-numeral or ill-sorted head constant", ErrIntervalDatalogFragment))
			}
			column = uint32(len(workingSorts))
			working, workingSorts, cause = e.appendConstantColumn(working, workingSorts, headSorts[i], argument)
			if cause != nil {
				return false, cause
			}
		}
		if usedColumns[column] {
			working, workingSorts, cause = e.duplicateColumn(working, workingSorts, column)
			if cause != nil {
				return false, cause
			}
			column = uint32(len(workingSorts) - 1)
		}
		usedColumns[column], desired[i] = true, column
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
	working, workingSorts, err = e.project(working, workingSorts, remove)
	if err != nil {
		return false, err
	}
	current := append([]uint32(nil), selected...)
	for destination, want := range desired {
		position := destination
		for position < len(current) && current[position] != want {
			position++
		}
		if position == len(current) {
			return false, ErrIntervalDatalogFragment
		}
		if position != destination {
			working, workingSorts, err = e.rename(working, workingSorts, []uint32{uint32(position), uint32(destination)})
			if err != nil {
				return false, err
			}
			current[position], current[destination] = current[destination], current[position]
		}
	}
	return e.union(e.relations[headDeclaration], headSorts, working, widen)
}

// intervalPredicateSCCs returns dependency-first SCC order. The source
// compiler completes lower strata before initializing and widening a
// recursive stratum.
func (e *IntervalDatalogEvaluator) intervalPredicateSCCs() (map[ast.ASTHandle]int, []int, error) {
	nodes := make([]ast.ASTHandle, 0, len(e.sorts))
	for declaration := range e.sorts {
		nodes = append(nodes, declaration)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Origin() != nodes[j].Origin() {
			return nodes[i].Origin() < nodes[j].Origin()
		}
		if nodes[i].Slot() != nodes[j].Slot() {
			return nodes[i].Slot() < nodes[j].Slot()
		}
		return nodes[i].Generation() < nodes[j].Generation()
	})
	graph := make(map[ast.ASTHandle][]ast.ASTHandle, len(nodes))
	count, err := e.rules.Len()
	if err != nil {
		return nil, nil, err
	}
	for i := 0; i < count; i++ {
		rule, cause := e.rules.Rule(i)
		if cause != nil {
			return nil, nil, cause
		}
		head, cause := rule.HeadDeclaration()
		positive, pErr := rule.PositiveTailSize()
		cause = errors.Join(cause, pErr)
		if _, declared := e.sorts[head]; !declared {
			cause = errors.Join(cause, fmt.Errorf("%w: undeclared rule head", ErrIntervalDatalogFragment))
		}
		seen := make(map[ast.ASTHandle]bool)
		for j := 0; cause == nil && j < positive; j++ {
			var tail ast.ASTHandle
			tail, cause = rule.TailDeclaration(j)
			if _, declared := e.sorts[tail]; !declared {
				cause = fmt.Errorf("%w: undeclared rule tail", ErrIntervalDatalogFragment)
			} else if !seen[tail] {
				seen[tail] = true
				graph[head] = append(graph[head], tail)
			}
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return nil, nil, cause
		}
	}
	for head := range graph {
		sort.Slice(graph[head], func(i, j int) bool {
			return graph[head][i].Slot() < graph[head][j].Slot()
		})
	}
	indices := make(map[ast.ASTHandle]int, len(nodes))
	low := make(map[ast.ASTHandle]int, len(nodes))
	onStack := make(map[ast.ASTHandle]bool, len(nodes))
	stack := make([]ast.ASTHandle, 0, len(nodes))
	component := make(map[ast.ASTHandle]int, len(nodes))
	nextIndex := 0
	componentCount := 0
	var connect func(ast.ASTHandle)
	connect = func(node ast.ASTHandle) {
		nextIndex++
		indices[node], low[node] = nextIndex, nextIndex
		stack = append(stack, node)
		onStack[node] = true
		for _, dependency := range graph[node] {
			if indices[dependency] == 0 {
				connect(dependency)
				if low[dependency] < low[node] {
					low[node] = low[dependency]
				}
			} else if onStack[dependency] && indices[dependency] < low[node] {
				low[node] = indices[dependency]
			}
		}
		if low[node] != indices[node] {
			return
		}
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component[last] = componentCount
			if last == node {
				break
			}
		}
		componentCount++
	}
	for _, node := range nodes {
		if indices[node] == 0 {
			connect(node)
		}
	}
	dependencies := make([]map[int]bool, componentCount)
	for head, tails := range graph {
		for _, tail := range tails {
			if component[head] != component[tail] {
				if dependencies[component[head]] == nil {
					dependencies[component[head]] = make(map[int]bool)
				}
				dependencies[component[head]][component[tail]] = true
			}
		}
	}
	visited := make([]bool, componentCount)
	order := make([]int, 0, componentCount)
	var visit func(int)
	visit = func(current int) {
		if visited[current] {
			return
		}
		visited[current] = true
		deps := make([]int, 0, len(dependencies[current]))
		for dependency := range dependencies[current] {
			deps = append(deps, dependency)
		}
		sort.Ints(deps)
		for _, dependency := range deps {
			visit(dependency)
		}
		order = append(order, current)
	}
	for current := 0; current < componentCount; current++ {
		visit(current)
	}
	return component, order, nil
}

func (e *IntervalDatalogEvaluator) evaluateIntervalComponent(component map[ast.ASTHandle]int, current int, internal, widen bool) (changed bool, resultErr error) {
	count, err := e.rules.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, cause := e.rules.Rule(i)
		if cause != nil {
			return false, cause
		}
		head, cause := rule.HeadDeclaration()
		if cause != nil || component[head] != current {
			cause = errors.Join(cause, rule.Close())
			if cause != nil {
				return false, cause
			}
			continue
		}
		tailSize, cause := rule.TailSize()
		if cause != nil || tailSize == 0 {
			cause = errors.Join(cause, rule.Close())
			if cause != nil {
				return false, cause
			}
			continue
		}
		positive, cause := rule.PositiveTailSize()
		hasInternalTail := false
		for j := 0; cause == nil && j < positive; j++ {
			var tail ast.ASTHandle
			tail, cause = rule.TailDeclaration(j)
			hasInternalTail = hasInternalTail || component[tail] == current
		}
		if cause != nil || hasInternalTail != internal {
			cause = errors.Join(cause, rule.Close())
			if cause != nil {
				return false, cause
			}
			continue
		}
		ruleChanged, cause := e.evaluatePositiveRule(rule, widen)
		if cause != nil {
			return false, cause
		}
		changed = changed || ruleChanged
	}
	return changed, nil
}

// SaturatePositive completes dependency SCCs in source order. External inputs
// initialize each component with ordinary union before internal recursive rules
// use source-requested widening.
func (e *IntervalDatalogEvaluator) SaturatePositive(widen bool) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	previous := make(map[ast.ASTHandle]*rel.IntervalRelation, len(e.relations))
	for declaration, relation := range e.relations {
		clone, err := relation.Clone()
		if err != nil {
			return err
		}
		previous[declaration] = clone
	}
	defer func() {
		if resultErr != nil {
			e.relations = previous
		}
	}()
	count, err := e.rules.Len()
	if err != nil {
		return err
	}
	// Seed every fact before scheduling dependent rules, independent of rule
	// insertion order. Fact insertion uses ordinary source union, never widen.
	for i := 0; i < count; i++ {
		rule, cause := e.rules.Rule(i)
		if cause != nil {
			return cause
		}
		tails, cause := rule.TailSize()
		if cause != nil {
			_ = rule.Close()
			return cause
		}
		if tails == 0 {
			if cause = e.applyFactRule(rule); cause != nil {
				return cause
			}
		} else if cause = rule.Close(); cause != nil {
			return cause
		}
	}
	component, order, err := e.intervalPredicateSCCs()
	if err != nil {
		return err
	}
	for _, current := range order {
		// Rules depending only on completed lower components form the ordinary
		// source initialization of this component.
		if _, err = e.evaluateIntervalComponent(component, current, false, false); err != nil {
			return err
		}
		for {
			changed, cause := e.evaluateIntervalComponent(component, current, true, widen)
			if cause != nil {
				return cause
			}
			if !changed {
				break
			}
		}
	}
	return nil
}
