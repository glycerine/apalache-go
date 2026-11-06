package fp

// Source: dl_compiler.cpp positive rule lowering specialized to
// bound_relation operations. Bound widening equals ordinary bound union.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (r *boundDatalogRuntime) checkpoint() error {
	canceled, err := r.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return z3exception.NewDefaultException(r.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func appendBoundFullColumn(runtime *boundDatalogRuntime, relation *rel.BoundRelation, sorts []ast.ASTHandle, sortHandle ast.ASTHandle) (*rel.BoundRelation, []ast.ASTHandle, uint32, error) {
	if relation == nil || sortHandle.IsNull() {
		return nil, nil, 0, ast.ErrASTDomain
	}
	column := uint32(len(sorts))
	full, err := runtime.newRelation([]ast.ASTHandle{sortHandle}, true)
	if err != nil {
		return nil, nil, 0, err
	}
	joined, err := runtime.join(relation, sorts, full, []ast.ASTHandle{sortHandle}, nil, nil)
	if err != nil {
		return nil, nil, 0, err
	}
	return joined, append(sorts, sortHandle), column, nil
}

// duplicateBoundColumn follows compiler::make_duplicate_column: project the
// selected source column, then join it back using an equality join.
func duplicateBoundColumn(relation *rel.BoundRelation, sorts []ast.ASTHandle, column uint32) (*rel.BoundRelation, []ast.ASTHandle, uint32, error) {
	if relation == nil || uint64(column) >= uint64(len(sorts)) {
		return nil, nil, 0, ast.ErrASTDomain
	}
	remove := make([]uint32, 0, len(sorts)-1)
	for i := range sorts {
		if uint32(i) != column {
			remove = append(remove, uint32(i))
		}
	}
	single, err := relation.Project(remove)
	if err != nil {
		return nil, nil, 0, err
	}
	duplicate := uint32(len(sorts))
	joined, err := rel.JoinBoundRelations(relation, single, []uint32{column}, []uint32{0})
	if err != nil {
		return nil, nil, 0, err
	}
	return joined, append(sorts, sorts[column]), duplicate, nil
}

func duplicateBoundColumnChecked(runtime *boundDatalogRuntime, relation *rel.BoundRelation, sorts []ast.ASTHandle, column uint32) (*rel.BoundRelation, []ast.ASTHandle, uint32, error) {
	if relation == nil || uint64(column) >= uint64(len(sorts)) {
		return nil, nil, 0, ast.ErrASTDomain
	}
	remove := make([]uint32, 0, len(sorts)-1)
	for i := range sorts {
		if uint32(i) != column {
			remove = append(remove, uint32(i))
		}
	}
	single, singleSorts, err := runtime.project(relation, sorts, remove)
	if err != nil {
		return nil, nil, 0, err
	}
	duplicate := uint32(len(sorts))
	joined, err := runtime.join(relation, sorts, single, singleSorts, []uint32{column}, []uint32{0})
	if err != nil {
		return nil, nil, 0, err
	}
	return joined, append(sorts, sorts[column]), duplicate, nil
}

func ensureBoundConditionVariables(runtime *boundDatalogRuntime, condition ast.ASTHandle, variables map[uint32]uint32, relation *rel.BoundRelation, sorts []ast.ASTHandle) (*rel.BoundRelation, []ast.ASTHandle, error) {
	manager := runtime.manager
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
		var column uint32
		relation, sorts, column, err = appendBoundFullColumn(runtime, relation, sorts, sortHandle)
		if err != nil {
			return nil, nil, err
		}
		variables[variable] = column
	}
	return relation, sorts, nil
}

func remapBoundCondition(manager *ast.ASTManager, condition ast.ASTHandle, variables map[uint32]uint32, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(condition); err != nil {
		return result, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		if !used.Contains(variable).IsNull() {
			if _, found := variables[variable]; !found {
				return result, fmt.Errorf("%w: unbound interpreted variable", ErrBoundDatalogFragment)
			}
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
			resultErr = errors.Join(resultErr, manager.DelRef(&roots[i]))
		}
	}()
	for variable, column := range variables {
		binding, err := manager.MkVar(column, sorts[column])
		if err != nil {
			return result, err
		}
		root, err := manager.NewRef(binding)
		if err != nil {
			return result, err
		}
		roots = append(roots, root)
		bindings[variable] = binding
	}
	substitution, err := astrewriter.NewVarSubstitution(manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(condition, bindings, result); err != nil {
		resultErr = errors.Join(err, manager.DelRef(&result))
		result = ast.ASTRef{}
	}
	return result, resultErr
}

func (r *boundDatalogRuntime) boundRuleHead(rule *base.Rule) (head ast.ASTHandle, formula ast.ASTRef, resultErr error) {
	formula, err := rule.ToFormula()
	if err != nil {
		return head, formula, err
	}
	head, err = r.manager.RefGet(formula)
	if err != nil {
		return head, formula, err
	}
	if kind, cause := r.manager.GetKind(head); cause != nil {
		return head, formula, cause
	} else if kind == ast.ASTQuantifier {
		head, err = r.manager.GetQuantifierBody(head)
		if err != nil {
			return head, formula, err
		}
	}
	if op, cause := r.manager.GetDeclKind(head); cause == nil && op == ast.ASTOpImplies {
		head, err = r.manager.GetArg(head, 1)
	} else if cause != nil {
		err = cause
	}
	return head, formula, err
}

func (r *boundDatalogRuntime) evaluatePositiveBoundRule(rule *base.Rule, relations map[ast.ASTHandle]*rel.BoundRelation, widen bool) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	if err != nil || positive == 0 {
		return false, errors.Join(err, ErrBoundDatalogFragment)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return false, errors.Join(err, fmt.Errorf("%w: negated predicate tail", ErrBoundDatalogFragment))
	}
	var working *rel.BoundRelation
	var workingSorts []ast.ASTHandle
	variables := make(map[uint32]uint32)
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return false, errors.Join(cause, ErrBoundDatalogFragment)
		}
		declaration, cause := r.manager.GetDecl(tail)
		if cause != nil || relations[declaration] == nil {
			return false, errors.Join(cause, ErrBoundDatalogFragment)
		}
		tailRelation := relations[declaration].Clone()
		tailSorts := r.entries[declaration].sorts
		local := make(map[uint32]uint32)
		arguments, cause := r.manager.GetNumArgs(tail)
		if cause != nil || int(arguments) != len(tailSorts) {
			return false, errors.Join(cause, ErrBoundDatalogFragment)
		}
		for column := uint32(0); column < arguments; column++ {
			argument, cause := r.manager.GetArg(tail, column)
			if cause != nil {
				return false, cause
			}
			kind, cause := r.manager.GetKind(argument)
			if cause != nil {
				return false, cause
			}
			if kind == ast.ASTVar {
				variable, cause := r.manager.GetVarIndex(argument)
				if cause != nil {
					return false, cause
				}
				if prior, found := local[variable]; found {
					if cause = r.filterIdentical(tailRelation, tailSorts, []uint32{prior, column}); cause != nil {
						return false, cause
					}
				} else {
					local[variable] = column
				}
				continue
			}
			_, _, numeral, cause := r.manager.ArithmeticNumeralText(argument)
			if cause != nil || !numeral {
				return false, errors.Join(cause, fmt.Errorf("%w: non-numeral tail constant", ErrBoundDatalogFragment))
			}
			if cause = r.filterEqual(tailRelation, tailSorts, column, argument); cause != nil {
				return false, cause
			}
		}
		if working == nil {
			working, workingSorts = tailRelation, append(workingSorts, tailSorts...)
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
		left, right := make([]uint32, len(shared)), make([]uint32, len(shared))
		for i, variable := range shared {
			left[i], right[i] = variables[variable], local[variable]
		}
		offset := uint32(len(workingSorts))
		working, cause = r.join(working, workingSorts, tailRelation, tailSorts, left, right)
		if cause != nil {
			return false, cause
		}
		workingSorts = append(workingSorts, tailSorts...)
		for variable, column := range local {
			if _, found := variables[variable]; !found {
				variables[variable] = offset + column
			}
		}
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	// dl_compiler combines interpreted tails into one conjunction before
	// invoking the relation plugin. BoundRelation recognizes atomic order
	// formulas but deliberately treats a conjunction as not applicable.
	// Therefore two or more interpreted tails are a single coarse no-op.
	interpretedCount := tailSize - uninterpreted
	for i := uninterpreted; interpretedCount == 1 && i < tailSize; i++ {
		condition, negated, cause := rule.Tail(i)
		if cause != nil || negated {
			return false, errors.Join(cause, ErrBoundDatalogFragment)
		}
		working, workingSorts, cause = ensureBoundConditionVariables(r, condition, variables, working, workingSorts)
		if cause != nil {
			return false, cause
		}
		mapped, cause := remapBoundCondition(r.manager, condition, variables, workingSorts)
		if cause != nil {
			return false, cause
		}
		handle, cause := r.manager.RefGet(mapped)
		if cause == nil {
			cause = r.filterInterpreted(working, workingSorts, handle)
		}
		cause = errors.Join(cause, r.manager.DelRef(&mapped))
		if cause != nil {
			return false, cause
		}
	}
	head, formula, err := r.boundRuleHead(rule)
	if formula != (ast.ASTRef{}) {
		defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&formula)) }()
	}
	if err != nil {
		return false, err
	}
	headDeclaration, err := r.manager.GetDecl(head)
	if err != nil || relations[headDeclaration] == nil {
		return false, errors.Join(err, ErrBoundDatalogFragment)
	}
	headCount, err := r.manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	desired := make([]uint32, headCount)
	usedColumns := make(map[uint32]bool, headCount)
	headSorts := r.entries[headDeclaration].sorts
	if int(headCount) != len(headSorts) {
		return false, ErrBoundDatalogFragment
	}
	for i := uint32(0); i < headCount; i++ {
		argument, cause := r.manager.GetArg(head, i)
		if cause != nil {
			return false, cause
		}
		kind, cause := r.manager.GetKind(argument)
		if cause != nil {
			return false, cause
		}
		var column uint32
		if kind == ast.ASTVar {
			variable, cause := r.manager.GetVarIndex(argument)
			if cause != nil {
				return false, cause
			}
			var found bool
			column, found = variables[variable]
			if !found {
				working, workingSorts, column, cause = appendBoundFullColumn(r, working, workingSorts, headSorts[i])
				if cause != nil {
					return false, cause
				}
				variables[variable] = column
			}
		} else {
			valueSort, sortErr := r.manager.GetSort(argument)
			_, _, numeral, numeralErr := r.manager.ArithmeticNumeralText(argument)
			cause = errors.Join(sortErr, numeralErr)
			if cause != nil || !numeral || valueSort != headSorts[i] {
				return false, errors.Join(cause, fmt.Errorf("%w: non-numeral or ill-sorted head constant", ErrBoundDatalogFragment))
			}
			working, workingSorts, column, cause = appendBoundFullColumn(r, working, workingSorts, headSorts[i])
			if cause != nil {
				return false, cause
			}
		}
		if usedColumns[column] {
			working, workingSorts, column, cause = duplicateBoundColumnChecked(r, working, workingSorts, column)
			if cause != nil {
				return false, cause
			}
		}
		usedColumns[column], desired[i] = true, column
	}
	selected := append([]uint32(nil), desired...)
	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	remove := make([]uint32, 0, len(workingSorts)-len(selected))
	for column, at := uint32(0), 0; int(column) < len(workingSorts); column++ {
		if at < len(selected) && selected[at] == column {
			at++
		} else {
			remove = append(remove, column)
		}
	}
	working, workingSorts, err = r.project(working, workingSorts, remove)
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
			return false, ErrBoundDatalogFragment
		}
		if position != destination {
			working, workingSorts, err = r.rename(working, workingSorts, []uint32{uint32(position), uint32(destination)})
			if err != nil {
				return false, err
			}
			current[position], current[destination] = current[destination], current[position]
		}
	}
	if widen {
		return r.union(relations[headDeclaration], working, headSorts, true)
	}
	return r.union(relations[headDeclaration], working, headSorts, false)
}

// saturatePositive uses a global fixpoint. This is source-equivalent for the
// bound plugin because its widening operation is exactly its ordinary union.
func (r *boundDatalogRuntime) saturatePositive(widen bool) error {
	if err := r.open(); err != nil {
		return err
	}
	r.evaluated = nil
	if err := r.checkpoint(); err != nil {
		return err
	}
	next := make(map[ast.ASTHandle]*rel.BoundRelation, len(r.entries))
	for declaration, entry := range r.entries {
		next[declaration] = entry.seed.Clone()
	}
	count, err := r.rules.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err = r.checkpoint(); err != nil {
			return err
		}
		rule, cause := r.rules.Rule(i)
		if cause != nil {
			return cause
		}
		tails, cause := rule.TailSize()
		if cause != nil {
			_ = rule.Close()
			return cause
		}
		if tails == 0 {
			declaration, cause := rule.HeadDeclaration()
			head, headErr := rule.Head()
			var values []ast.ASTHandle
			if headErr == nil {
				var arity uint32
				arity, headErr = r.manager.GetNumArgs(head)
				if headErr == nil {
					values = make([]ast.ASTHandle, arity)
					for j := range values {
						values[j], headErr = r.manager.GetArg(head, uint32(j))
						if headErr != nil {
							break
						}
					}
				}
			}
			cause = errors.Join(cause, headErr, rule.Close())
			entry := r.entries[declaration]
			if cause != nil || entry == nil || len(values) != len(entry.sorts) {
				return errors.Join(cause, ErrBoundDatalogFragment)
			}
			if cause = r.addFactTo(next[declaration], entry.sorts, values); cause != nil {
				return cause
			}
		} else if cause = rule.Close(); cause != nil {
			return cause
		}
	}
	for {
		if err = r.checkpoint(); err != nil {
			return err
		}
		changed := false
		for i := 0; i < count; i++ {
			if err = r.checkpoint(); err != nil {
				return err
			}
			rule, cause := r.rules.Rule(i)
			if cause != nil {
				return cause
			}
			tails, cause := rule.TailSize()
			if cause != nil || tails == 0 {
				cause = errors.Join(cause, rule.Close())
				if cause != nil {
					return cause
				}
				continue
			}
			ruleChanged, cause := r.evaluatePositiveBoundRule(rule, next, widen)
			if cause != nil {
				return cause
			}
			changed = changed || ruleChanged
		}
		if !changed {
			break
		}
	}
	r.evaluated = next
	return nil
}
