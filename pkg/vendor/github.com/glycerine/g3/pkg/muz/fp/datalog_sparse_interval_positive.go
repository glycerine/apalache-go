package fp

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func appendSparseIntervalFullColumn(product *rel.SparseIntervalSieveProduct, sorts []ast.ASTHandle, sortHandle ast.ASTHandle) (*rel.SparseIntervalSieveProduct, []ast.ASTHandle, int, error) {
	if product == nil || sortHandle.IsNull() {
		return nil, nil, 0, ErrSparseIntervalDatalogFragment
	}
	next, err := product.AppendFullColumn(sortHandle)
	if err != nil {
		return nil, nil, 0, err
	}
	if err = product.Close(); err != nil {
		return nil, nil, 0, errors.Join(err, next.Close())
	}
	return next, append(sorts, sortHandle), len(sorts), nil
}

func duplicateSparseIntervalColumn(product *rel.SparseIntervalSieveProduct, sorts []ast.ASTHandle, column int) (*rel.SparseIntervalSieveProduct, []ast.ASTHandle, int, error) {
	if product == nil || column < 0 || column >= len(sorts) {
		return nil, nil, 0, ErrSparseIntervalDatalogFragment
	}
	next, err := product.DuplicateColumn(column)
	if err != nil {
		return nil, nil, 0, err
	}
	if err = product.Close(); err != nil {
		return nil, nil, 0, errors.Join(err, next.Close())
	}
	return next, append(sorts, sorts[column]), len(sorts), nil
}

func ensureSparseIntervalConditionVariables(manager *ast.ASTManager, condition ast.ASTHandle, variables map[uint32]int, product *rel.SparseIntervalSieveProduct, sorts []ast.ASTHandle) (*rel.SparseIntervalSieveProduct, []ast.ASTHandle, error) {
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
		var column int
		product, sorts, column, err = appendSparseIntervalFullColumn(product, sorts, sortHandle)
		if err != nil {
			return nil, nil, err
		}
		variables[variable] = column
	}
	return product, sorts, nil
}

func (e *SparseIntervalDatalogEvaluator) evaluatePositiveRule(rule *base.Rule, widen bool) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	if err := e.checkpoint(); err != nil {
		return false, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted < positive || uninterpreted == 0 {
		return false, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	var working *rel.SparseIntervalSieveProduct
	defer func() {
		if working != nil {
			resultErr = errors.Join(resultErr, working.Close())
		}
	}()
	var workingSorts []ast.ASTHandle
	variables := make(map[uint32]int)
	if positive == 0 {
		// dl_compiler.cpp starts a negation-only rule with a full nullary
		// relation, then materializes each unbound negative variable as a full
		// column before applying the exact table difference.
		working, err = rel.NewSparseIntervalSieveProductAuxiliary(e.relations, nil, true)
		if err != nil {
			return false, err
		}
	}
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return false, errors.Join(cause, ErrSparseIntervalDatalogFragment)
		}
		declaration, cause := e.manager.GetDecl(tail)
		if cause != nil || e.products[declaration] == nil {
			return false, errors.Join(cause, fmt.Errorf("%w: undeclared tail", ErrSparseIntervalDatalogFragment))
		}
		tailProduct, cause := e.products[declaration].Clone()
		if cause != nil {
			return false, cause
		}
		tailOwned := true
		defer func() {
			if tailOwned {
				resultErr = errors.Join(resultErr, tailProduct.Close())
			}
		}()
		tailSorts := e.sorts[declaration]
		local := make(map[uint32]int)
		arguments, cause := e.manager.GetNumArgs(tail)
		if cause != nil || int(arguments) != len(tailSorts) {
			return false, errors.Join(cause, ErrSparseIntervalDatalogFragment)
		}
		for column := 0; column < int(arguments); column++ {
			argument, cause := e.manager.GetArg(tail, uint32(column))
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
				if prior, found := local[variable]; found {
					if cause = tailProduct.FilterIdentical([]int{prior, column}); cause != nil {
						return false, cause
					}
				} else {
					local[variable] = column
				}
			} else if cause = tailProduct.FilterEqual(column, argument); cause != nil {
				return false, errors.Join(cause, ErrSparseIntervalDatalogFragment)
			}
		}
		if working == nil {
			working, tailOwned = tailProduct, false
			workingSorts = append(workingSorts, tailSorts...)
			for variable, column := range local {
				variables[variable] = column
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
		left, right := make([]int, len(shared)), make([]int, len(shared))
		for i, variable := range shared {
			left[i], right[i] = variables[variable], local[variable]
		}
		offset := len(workingSorts)
		joined, cause := rel.JoinSparseIntervalSieveProducts(working, tailProduct, left, right)
		if cause != nil {
			return false, cause
		}
		if cause = errors.Join(working.Close(), tailProduct.Close()); cause != nil {
			_ = joined.Close()
			return false, cause
		}
		working, tailOwned = joined, false
		workingSorts = append(workingSorts, tailSorts...)
		for variable, column := range local {
			if _, found := variables[variable]; !found {
				variables[variable] = offset + column
			}
		}
	}
	working, workingSorts, err = e.applySparseIntervalFiniteNegation(rule, positive, uninterpreted, working, workingSorts, variables)
	if err != nil {
		return false, err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	intervalHelper := &IntervalDatalogEvaluator{manager: e.manager}
	conditions := make([]ast.ASTHandle, 0, tailSize-uninterpreted)
	for i := uninterpreted; i < tailSize; i++ {
		condition, negated, cause := rule.Tail(i)
		if cause != nil || negated {
			return false, errors.Join(cause, ErrSparseIntervalDatalogFragment)
		}
		conditions = append(conditions, condition)
	}
	if len(conditions) != 0 {
		condition := conditions[0]
		if len(conditions) != 1 {
			var cause error
			condition, cause = e.manager.MkAndArgs(conditions)
			if cause != nil {
				return false, cause
			}
		}
		conditionRef, cause := e.manager.NewRef(condition)
		if cause != nil {
			return false, cause
		}
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&conditionRef)) }()
		working, workingSorts, cause = ensureSparseIntervalConditionVariables(e.manager, condition, variables, working, workingSorts)
		if cause != nil {
			return false, cause
		}
		mappedVariables := make(map[uint32]uint32, len(variables))
		for variable, column := range variables {
			mappedVariables[variable] = uint32(column)
		}
		mapped, cause := intervalHelper.remapIntervalCondition(condition, mappedVariables, workingSorts)
		if cause != nil {
			return false, errors.Join(cause, ErrSparseIntervalDatalogFragment)
		}
		mappedHandle, cause := e.manager.RefGet(mapped)
		if cause == nil {
			cause = working.FilterInterpreted(mappedHandle, e.guard)
		}
		cause = errors.Join(cause, e.manager.DelRef(&mapped))
		if cause != nil {
			return false, cause
		}
	}
	head, formula, err := intervalHelper.ruleHead(rule)
	if formula != (ast.ASTRef{}) {
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&formula)) }()
	}
	if err != nil {
		return false, err
	}
	headDeclaration, err := e.manager.GetDecl(head)
	if err != nil || e.products[headDeclaration] == nil {
		return false, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	headCount, err := e.manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	desired := make([]int, headCount)
	usedColumns := make(map[int]bool, headCount)
	headSorts := e.sorts[headDeclaration]
	if int(headCount) != len(headSorts) {
		return false, ErrSparseIntervalDatalogFragment
	}
	for i := 0; i < int(headCount); i++ {
		argument, cause := e.manager.GetArg(head, uint32(i))
		if cause != nil {
			return false, cause
		}
		kind, cause := e.manager.GetKind(argument)
		if cause != nil {
			return false, cause
		}
		var column int
		if kind == ast.ASTVar {
			variable, cause := e.manager.GetVarIndex(argument)
			if cause != nil {
				return false, cause
			}
			var found bool
			column, found = variables[variable]
			if !found {
				working, workingSorts, column, cause = appendSparseIntervalFullColumn(working, workingSorts, headSorts[i])
				if cause != nil {
					return false, cause
				}
				variables[variable] = column
			}
		} else {
			valueSort, sortErr := e.manager.GetSort(argument)
			_, _, numeral, numeralErr := e.manager.ArithmeticNumeralText(argument)
			if !rel.CanHandleIntervalSorts(e.manager, []ast.ASTHandle{headSorts[i]}) {
				// Finite constants are not arithmetic numerals, but FilterEqual
				// validates and encodes them through the sparse component.
				numeral, numeralErr = true, nil
			}
			cause = errors.Join(sortErr, numeralErr)
			if cause != nil || !numeral || valueSort != headSorts[i] {
				return false, errors.Join(cause, fmt.Errorf("%w: ill-sorted head constant", ErrSparseIntervalDatalogFragment))
			}
			working, workingSorts, column, cause = appendSparseIntervalFullColumn(working, workingSorts, headSorts[i])
			if cause == nil {
				cause = working.FilterEqual(column, argument)
			}
			if cause != nil {
				return false, cause
			}
		}
		if usedColumns[column] {
			working, workingSorts, column, cause = duplicateSparseIntervalColumn(working, workingSorts, column)
			if cause != nil {
				return false, cause
			}
		}
		usedColumns[column], desired[i] = true, column
	}
	selected := append([]int(nil), desired...)
	sort.Ints(selected)
	remove := make([]int, 0, len(workingSorts)-len(selected))
	for column, selectedIndex := 0, 0; column < len(workingSorts); column++ {
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
		_ = projected.Close()
		return false, err
	}
	working = projected
	current := append([]int(nil), selected...)
	for destination, want := range desired {
		position := destination
		for position < len(current) && current[position] != want {
			position++
		}
		if position == len(current) {
			return false, ErrSparseIntervalDatalogFragment
		}
		if position != destination {
			renamed, cause := working.Rename([]int{position, destination})
			if cause != nil {
				return false, cause
			}
			if cause = working.Close(); cause != nil {
				_ = renamed.Close()
				return false, cause
			}
			working = renamed
			current[position], current[destination] = current[destination], current[position]
		}
	}
	empty, err := working.Empty()
	if err != nil || empty {
		return false, err
	}
	target := e.products[headDeclaration]
	targetEmpty, cause := target.Empty()
	if cause != nil {
		return false, cause
	}
	if targetEmpty {
		if cause = target.AdoptPrecisionShape(working); cause != nil {
			return false, cause
		}
	}
	return target.Union(working, widen)
}

func (e *SparseIntervalDatalogEvaluator) evaluateComponent(component map[ast.ASTHandle]int, current int, internal, widen bool) (changed bool, resultErr error) {
	count, err := e.rules.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		if err = e.checkpoint(); err != nil {
			return false, err
		}
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
		tails, cause := rule.TailSize()
		if cause != nil || tails == 0 {
			cause = errors.Join(cause, rule.Close())
			if cause != nil {
				return false, cause
			}
			continue
		}
		positive, cause := rule.PositiveTailSize()
		hasInternal := false
		for j := 0; cause == nil && j < positive; j++ {
			var tail ast.ASTHandle
			tail, cause = rule.TailDeclaration(j)
			hasInternal = hasInternal || component[tail] == current
		}
		if cause != nil || hasInternal != internal {
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

func closeSparseIntervalProducts(products map[ast.ASTHandle]*rel.SparseIntervalSieveProduct) (result error) {
	for declaration, product := range products {
		result = errors.Join(result, product.Close())
		delete(products, declaration)
	}
	return result
}

// SaturatePositive completes dependencies before recursive SCCs and applies
// widening only to internal recursive edges.
func (e *SparseIntervalDatalogEvaluator) SaturatePositive(widen bool) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	if err := e.checkpoint(); err != nil {
		return err
	}
	previous := make(map[ast.ASTHandle]*rel.SparseIntervalSieveProduct, len(e.products))
	for declaration, product := range e.products {
		clone, err := product.Clone()
		if err != nil {
			return errors.Join(err, closeSparseIntervalProducts(previous))
		}
		previous[declaration] = clone
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeSparseIntervalProducts(e.products))
			e.products = previous
		} else {
			resultErr = closeSparseIntervalProducts(previous)
		}
	}()
	count, err := e.rules.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err = e.checkpoint(); err != nil {
			return err
		}
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
	if err = e.checkpoint(); err != nil {
		return err
	}
	component, order, err := e.sparseIntervalPredicateStrata()
	if err != nil {
		return errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	for _, current := range order {
		if err = e.checkpoint(); err != nil {
			return err
		}
		if _, err = e.evaluateComponent(component, current, false, false); err != nil {
			return err
		}
		for {
			if err = e.checkpoint(); err != nil {
				return err
			}
			changed, cause := e.evaluateComponent(component, current, true, widen)
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
