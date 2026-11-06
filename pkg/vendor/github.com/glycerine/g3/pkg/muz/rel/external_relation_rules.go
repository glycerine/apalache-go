package rel

// Source: src/muz/rel/dl_compiler.cpp fact and single-tail rule compilation,
// specialized for src/muz/rel/dl_external_relation.cpp operations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

type externalFactRule struct {
	head        ast.ASTHandle
	tails       []ast.ASTHandle
	values      []ast.ASTHandle
	joinLeft    [][]uint32
	joinRight   [][]uint32
	headColumns []externalHeadColumn
	filters     []externalTailFilter
	interpreted []ast.ASTHandle
	columns     map[uint32]uint32
	negative    []externalNegativeTail
	seeds       []externalRuleColumnSeed
	headSeeds   []externalRuleColumnSeed
	ground      bool
	stratum     uint32
}

type externalRuleColumnSeed struct {
	sort, constant ast.ASTHandle
}

type externalNegativeTail struct {
	predicate                      ast.ASTHandle
	targetColumns, negativeColumns []uint32
}

type externalTailFilter struct {
	equalColumns []uint32
	equalValues  []ast.ASTHandle
	identical    [][]uint32
}

func prepareExternalRuleTail(source *ExternalRelation, filter externalTailFilter) (_ *ExternalRelation, resultErr error) {
	value, err := source.Clone()
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, value.Close())
		}
	}()
	for index, column := range filter.equalColumns {
		if err = value.FilterEqual(column, filter.equalValues[index]); err != nil {
			return nil, err
		}
	}
	for _, columns := range filter.identical {
		if err = value.FilterIdentical(columns); err != nil {
			return nil, err
		}
	}
	published = true
	return value, nil
}

func classifyExternalFactRule(manager *ast.ASTManager, rule *base.Rule) (externalFactRule, error) {
	var result externalFactRule
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	result.head, err = manager.GetDecl(head)
	if err != nil {
		return result, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return result, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	tails, err := rule.TailSize()
	if err != nil || positive < 0 || uninterpreted < positive || tails < uninterpreted {
		return result, errors.Join(err, ErrCompilerUnsupported)
	}
	headCount, err := manager.GetNumArgs(head)
	if err != nil {
		return result, err
	}
	headArgs := make([]ast.ASTHandle, headCount)
	for index := uint32(0); index < headCount; index++ {
		headArgs[index], err = manager.GetArg(head, index)
		if err != nil {
			return result, err
		}
	}
	if positive == 0 && tails == 0 {
		allValues := true
		for _, value := range headArgs {
			valid, cause := manager.IsValue(value)
			if cause != nil {
				return result, cause
			}
			allValues = allValues && valid
		}
		if allValues {
			result.values = headArgs
			result.ground = true
			return result, nil
		}
	}
	positions := make(map[ast.ASTHandle]uint32)
	columns := make([]ast.ASTHandle, 0)
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		declaration, cause := manager.GetDecl(tail)
		if cause != nil {
			return result, cause
		}
		result.tails = append(result.tails, declaration)
		result.filters = append(result.filters, externalTailFilter{})
		if tailIndex != 0 {
			result.joinLeft = append(result.joinLeft, nil)
			result.joinRight = append(result.joinRight, nil)
		}
		count, cause := manager.GetNumArgs(tail)
		if cause != nil {
			return result, cause
		}
		local := make(map[ast.ASTHandle][]uint32, count)
		localOrder := make([]ast.ASTHandle, 0, count)
		for index := uint32(0); index < count; index++ {
			argument, argErr := manager.GetArg(tail, index)
			if argErr != nil {
				return result, argErr
			}
			combined := uint32(len(columns))
			columns = append(columns, argument)
			kind, argErr := manager.GetKind(argument)
			if argErr != nil {
				return result, argErr
			}
			if kind != ast.ASTVar {
				value, valueErr := manager.IsValue(argument)
				if valueErr != nil || !value {
					return result, errors.Join(valueErr, ErrCompilerUnsupported)
				}
				result.filters[tailIndex].equalColumns = append(result.filters[tailIndex].equalColumns, index)
				result.filters[tailIndex].equalValues = append(result.filters[tailIndex].equalValues, argument)
				continue
			}
			local[argument] = append(local[argument], index)
			if len(local[argument]) == 1 {
				localOrder = append(localOrder, argument)
				if tailIndex != 0 {
					if left, shared := positions[argument]; shared {
						result.joinLeft[tailIndex-1] = append(result.joinLeft[tailIndex-1], left)
						result.joinRight[tailIndex-1] = append(result.joinRight[tailIndex-1], index)
					}
				}
				if _, found := positions[argument]; !found {
					positions[argument] = combined
				}
			}
		}
		for _, variable := range localOrder {
			group := local[variable]
			if len(group) > 1 {
				result.filters[tailIndex].identical = append(result.filters[tailIndex].identical, group)
			}
		}
	}
	negativeApplications := make([]ast.ASTHandle, 0, uninterpreted-positive)
	negativePredicates := make([]ast.ASTHandle, 0, uninterpreted-positive)
	for tailIndex := positive; tailIndex < uninterpreted; tailIndex++ {
		negative, negated, cause := rule.Tail(tailIndex)
		if cause != nil || !negated {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		predicate, cause := manager.GetDecl(negative)
		if cause != nil {
			return result, cause
		}
		negativeApplications = append(negativeApplications, negative)
		negativePredicates = append(negativePredicates, predicate)
		count, cause := manager.GetNumArgs(negative)
		if cause != nil {
			return result, cause
		}
		for column := uint32(0); column < count; column++ {
			argument, argErr := manager.GetArg(negative, column)
			if argErr != nil {
				return result, argErr
			}
			kind, argErr := manager.GetKind(argument)
			if argErr != nil {
				return result, argErr
			}
			if kind == ast.ASTVar {
				_, found := positions[argument]
				if !found {
					sort, sortErr := manager.GetSort(argument)
					if sortErr != nil {
						return result, sortErr
					}
					positions[argument] = uint32(len(columns) + len(result.seeds))
					result.seeds = append(result.seeds, externalRuleColumnSeed{sort: sort})
				}
			} else {
				value, valueErr := manager.IsValue(argument)
				if valueErr != nil || !value {
					return result, errors.Join(valueErr, ErrCompilerUnsupported)
				}
			}
		}
	}
	result.columns = make(map[uint32]uint32, len(positions))
	for variable, column := range positions {
		index, cause := manager.GetVarIndex(variable)
		if cause != nil {
			return result, cause
		}
		result.columns[index] = column
	}
	for tailIndex := uninterpreted; tailIndex < tails; tailIndex++ {
		condition, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		result.interpreted = append(result.interpreted, condition)
	}
	if len(result.interpreted) != 0 {
		free, cause := astrewriter.NewExprFreeVars(manager)
		if cause != nil {
			return result, cause
		}
		for _, condition := range result.interpreted {
			if cause = free.Accumulate(condition); cause != nil {
				return result, cause
			}
		}
		for variable := uint32(0); variable < free.Size(); variable++ {
			sort, sortErr := free.Get(variable)
			if sortErr != nil {
				return result, sortErr
			}
			if sort.IsNull() {
				continue
			}
			if _, found := result.columns[variable]; !found {
				result.columns[variable] = uint32(len(columns) + len(result.seeds))
				result.seeds = append(result.seeds, externalRuleColumnSeed{sort: sort})
			}
		}
	}
	for tailIndex, negative := range negativeApplications {
		entry := externalNegativeTail{predicate: negativePredicates[tailIndex]}
		count, cause := manager.GetNumArgs(negative)
		if cause != nil {
			return result, cause
		}
		for column := uint32(0); column < count; column++ {
			argument, argErr := manager.GetArg(negative, column)
			if argErr != nil {
				return result, argErr
			}
			kind, argErr := manager.GetKind(argument)
			if argErr != nil {
				return result, argErr
			}
			var target uint32
			if kind == ast.ASTVar {
				target = positions[argument]
			} else {
				target = uint32(len(columns) + len(result.seeds))
				result.seeds = append(result.seeds, externalRuleColumnSeed{constant: argument})
			}
			entry.targetColumns = append(entry.targetColumns, target)
			entry.negativeColumns = append(entry.negativeColumns, column)
		}
		result.negative = append(result.negative, entry)
	}
	if len(result.interpreted) != 0 {
		if err = validateExternalInterpretedColumns(manager, result.interpreted, result.columns); err != nil {
			return result, err
		}
	}
	result.headColumns = make([]externalHeadColumn, 0, len(headArgs))
	for _, argument := range headArgs {
		kind, cause := manager.GetKind(argument)
		if cause != nil {
			return result, cause
		}
		if kind == ast.ASTVar {
			index, indexErr := manager.GetVarIndex(argument)
			if indexErr != nil {
				return result, indexErr
			}
			position, found := result.columns[index]
			if !found {
				sort, sortErr := manager.GetSort(argument)
				if sortErr != nil {
					return result, sortErr
				}
				position = uint32(len(columns) + len(result.seeds) + len(result.headSeeds))
				result.columns[index] = position
				result.headSeeds = append(result.headSeeds, externalRuleColumnSeed{sort: sort})
			}
			result.headColumns = append(result.headColumns, externalSourceHeadColumn(position))
			continue
		}
		value, cause := manager.IsValue(argument)
		if cause != nil || !value {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		result.headColumns = append(result.headColumns, externalConstantHeadColumn(argument))
	}
	return result, nil
}

// ExecuteExternalFactRules runs the concrete source-reached external relation
// slice: ground facts and ordered positive-tail equality joins, stratified
// negation with full or singleton columns, and bound interpreted filters,
// followed by source head assembly. It validates the whole set before mutation.
func ExecuteExternalFactRules(plugin *ExternalRelationPlugin, set *base.RuleSet) (resultErr error) {
	return ExecuteExternalFactRulesWithOptions(plugin, set, ExternalRuleExecutionOptions{})
}

// ExecuteExternalFactRulesWithOptions preserves the source compiler's
// dependency-first SCC schedule and recursive widening callback selection.
func ExecuteExternalFactRulesWithOptions(plugin *ExternalRelationPlugin, set *base.RuleSet, options ExternalRuleExecutionOptions) (resultErr error) {
	if plugin == nil || plugin.closed || plugin.ast == nil || set == nil {
		return ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	compiled := make([]externalFactRule, 0, count)
	for index := 0; index < count; index++ {
		rule, cause := set.Rule(index)
		if cause != nil {
			return cause
		}
		entry, cause := classifyExternalFactRule(plugin.ast, rule)
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return cause
		}
		compiled = append(compiled, entry)
	}
	components, err := planExternalRuleComponents(set, compiled)
	if err != nil {
		return err
	}
	validated := make(map[ast.ASTHandle]bool)
	predicateOrder := make([]ast.ASTHandle, 0)
	for _, entry := range compiled {
		predicates := make([]ast.ASTHandle, 0, 1+len(entry.tails)+len(entry.negative))
		predicates = append(predicates, entry.head)
		predicates = append(predicates, entry.tails...)
		for _, negative := range entry.negative {
			predicates = append(predicates, negative.predicate)
		}
		for _, predicate := range predicates {
			if predicate.IsNull() || validated[predicate] {
				continue
			}
			if _, err = externalPredicateSorts(plugin.ast, predicate); err != nil {
				return errors.Join(err, ErrCompilerUnsupported)
			}
			validated[predicate] = true
			predicateOrder = append(predicateOrder, predicate)
		}
	}
	// Relation allocation belongs to setup, not logical saturation. Complete
	// every source manager lookup and OP_RA_EMPTY callback before the first fact
	// is published so a later constructor failure cannot leave a rule prefix.
	for _, predicate := range predicateOrder {
		if _, err = plugin.PredicateRelation(predicate); err != nil {
			return err
		}
	}
	// Source make_add_constant_column caches unary singleton tables in top-level
	// setup. Construct every required singleton before publishing ground facts,
	// preserving whole-program preflight on callback failure.
	constantRelations := make(map[ast.ASTHandle]*ExternalRelation)
	preflightOwner := &externalRulePreflightOwner{plugin: plugin}
	defer func() { resultErr = finishExternalRulePreflight(preflightOwner, resultErr) }()
	makeConstant := func(constant ast.ASTHandle) error {
		if constant.IsNull() || constantRelations[constant] != nil {
			return nil
		}
		sort, cause := plugin.ast.GetSort(constant)
		if cause != nil {
			return cause
		}
		singleton, cause := plugin.NewEmpty([]ast.ASTHandle{sort})
		if cause != nil {
			return cause
		}
		preflightOwner.singletons = append(preflightOwner.singletons, singleton)
		if cause = singleton.AddFact([]ast.ASTHandle{constant}); cause != nil {
			return cause
		}
		constantRelations[constant] = singleton
		return nil
	}
	for _, entry := range compiled {
		for _, column := range entry.headColumns {
			if err = makeConstant(column.constant); err != nil {
				return err
			}
		}
		for _, seed := range entry.seeds {
			if err = makeConstant(seed.constant); err != nil {
				return err
			}
		}
	}
	fullRelations := make(map[ast.ASTHandle]*ExternalRelation)
	for _, entry := range compiled {
		seeds := append(append([]externalRuleColumnSeed(nil), entry.seeds...), entry.headSeeds...)
		for _, seed := range seeds {
			if seed.sort.IsNull() || fullRelations[seed.sort] != nil {
				continue
			}
			full, cause := plugin.NewTotal([]ast.ASTHandle{seed.sort})
			if cause != nil {
				return cause
			}
			fullRelations[seed.sort] = full
			preflightOwner.totals = append(preflightOwner.totals, full)
		}
	}
	var nullaryTotal *ExternalRelation
	needsNullaryTotal := false
	for _, entry := range compiled {
		hasFullSeed := len(entry.seeds) != 0 && !entry.seeds[0].sort.IsNull()
		if len(entry.tails) == 0 && !hasFullSeed && (len(entry.interpreted) != 0 || len(entry.negative) != 0) {
			needsNullaryTotal = true
			break
		}
	}
	if needsNullaryTotal {
		nullaryTotal, err = plugin.NewTotal(nil)
		if err != nil {
			return err
		}
		preflightOwner.totals = append(preflightOwner.totals, nullaryTotal)
	}
	checkpoint := func() error {
		canceled, cause := plugin.ast.Canceled()
		if cause != nil {
			return cause
		}
		if canceled {
			return fmt.Errorf("muz/rel: %s", plugin.ast.ResourceLimit().CancelMessage())
		}
		return nil
	}
	if err = checkpoint(); err != nil {
		return err
	}
	for _, entry := range compiled {
		if !entry.ground {
			continue
		}
		target, cause := plugin.PredicateRelation(entry.head)
		if cause != nil {
			return cause
		}
		if cause = target.AddFact(entry.values); cause != nil {
			return cause
		}
	}
	executionOwner := &externalRuleExecutionOwner{runtime: plugin.ast.NumeralService().RuntimeOwner()}
	defer func() { resultErr = finishExternalRuleExecution(executionOwner, resultErr) }()
	closeOwned := func(relation *ExternalRelation) error { return executionOwner.closeOne(relation) }
	executeEntry := func(entry externalFactRule, overrideIndex int, override, delta *ExternalRelation, widen bool) error {
		var value *ExternalRelation
		var cause error
		seedStart := 0
		headSeedStart := 0
		if len(entry.tails) == 0 {
			if len(entry.seeds) != 0 && !entry.seeds[0].sort.IsNull() {
				value, cause = fullRelations[entry.seeds[0].sort].Clone()
				seedStart = 1
			} else if len(entry.headSeeds) != 0 {
				value, cause = fullRelations[entry.headSeeds[0].sort].Clone()
				headSeedStart = 1
			} else {
				value, cause = nullaryTotal.Clone()
			}
		} else {
			source := override
			var sourceErr error
			if overrideIndex != 0 {
				source, sourceErr = plugin.PredicateRelation(entry.tails[0])
			}
			if sourceErr != nil {
				return sourceErr
			}
			value, cause = prepareExternalRuleTail(source, entry.filters[0])
		}
		value = executionOwner.adopt(value)
		if cause != nil {
			return cause
		}
		for tailIndex := 1; tailIndex < len(entry.tails); tailIndex++ {
			second := override
			var secondErr error
			if overrideIndex != tailIndex {
				second, secondErr = plugin.PredicateRelation(entry.tails[tailIndex])
			}
			if secondErr != nil {
				return errors.Join(secondErr, closeOwned(value))
			}
			secondValue, secondErr := prepareExternalRuleTail(second, entry.filters[tailIndex])
			secondValue = executionOwner.adopt(secondValue)
			if secondErr != nil {
				return errors.Join(secondErr, closeOwned(value))
			}
			joined, joinErr := JoinExternalRelations(value, secondValue, entry.joinLeft[tailIndex-1], entry.joinRight[tailIndex-1])
			joined = executionOwner.adopt(joined)
			joinErr = errors.Join(joinErr, closeOwned(value), closeOwned(secondValue))
			if joinErr != nil {
				return joinErr
			}
			value = joined
		}
		for _, seed := range entry.seeds[seedStart:] {
			second := fullRelations[seed.sort]
			if second == nil {
				second = constantRelations[seed.constant]
			}
			joined, joinErr := JoinExternalRelations(value, second, nil, nil)
			joined = executionOwner.adopt(joined)
			joinErr = errors.Join(joinErr, closeOwned(value))
			if joinErr != nil {
				return joinErr
			}
			value = joined
		}
		for _, negative := range entry.negative {
			relation, negativeErr := plugin.PredicateRelation(negative.predicate)
			if negativeErr == nil {
				negativeErr = value.FilterNegation(relation, negative.targetColumns, negative.negativeColumns)
			}
			if negativeErr != nil {
				return errors.Join(negativeErr, closeOwned(value))
			}
		}
		if len(entry.interpreted) != 0 {
			if cause = filterExternalRuleInterpreted(value, entry.interpreted, entry.columns); cause != nil {
				return errors.Join(cause, closeOwned(value))
			}
		}
		for _, seed := range entry.headSeeds[headSeedStart:] {
			joined, joinErr := JoinExternalRelations(value, fullRelations[seed.sort], nil, nil)
			joined = executionOwner.adopt(joined)
			joinErr = errors.Join(joinErr, closeOwned(value))
			if joinErr != nil {
				return joinErr
			}
			value = joined
		}
		assembled, assembleErr := assembleExternalRuleHead(value, entry.headColumns, constantRelations)
		assembled = executionOwner.adopt(assembled)
		assembleErr = errors.Join(assembleErr, closeOwned(value))
		if assembleErr != nil {
			return assembleErr
		}
		value = assembled
		target, cause := plugin.PredicateRelation(entry.head)
		if cause == nil {
			cause = target.Union(value, delta, widen)
		}
		cause = errors.Join(cause, closeOwned(value))
		return cause
	}
	closeDeltas := func(deltas map[ast.ASTHandle]*ExternalRelation, heads []ast.ASTHandle) error {
		var cause error
		for index := len(heads) - 1; index >= 0; index-- {
			if delta := deltas[heads[index]]; delta != nil {
				cause = errors.Join(cause, closeOwned(delta))
			}
		}
		return cause
	}
	newDeltas := func(heads []ast.ASTHandle, clone bool) (map[ast.ASTHandle]*ExternalRelation, error) {
		deltas := make(map[ast.ASTHandle]*ExternalRelation, len(heads))
		for _, head := range heads {
			target, cause := plugin.PredicateRelation(head)
			var delta *ExternalRelation
			if cause == nil && clone {
				delta, cause = target.Clone()
			} else if cause == nil {
				var sorts []ast.ASTHandle
				sorts, cause = target.Sorts()
				if cause == nil {
					delta, cause = plugin.NewEmpty(sorts)
				}
			}
			delta = executionOwner.adopt(delta)
			if cause != nil {
				return nil, errors.Join(cause, closeDeltas(deltas, heads))
			}
			deltas[head] = delta
		}
		return deltas, nil
	}
	for _, component := range components {
		if err = checkpoint(); err != nil {
			return err
		}
		if !component.recursive {
			for _, scheduled := range component.rules {
				entry := compiled[scheduled.index]
				if !entry.ground {
					if err = executeEntry(entry, -1, nil, nil, false); err != nil {
						return err
					}
				}
			}
			continue
		}
		for _, scheduled := range component.rules {
			entry := compiled[scheduled.index]
			if !entry.ground && len(scheduled.internalTails) == 0 {
				if err = executeEntry(entry, -1, nil, nil, false); err != nil {
					return err
				}
			}
		}
		current, cause := newDeltas(component.heads, true)
		if cause != nil {
			return cause
		}
		for {
			if err = checkpoint(); err != nil {
				return errors.Join(err, closeDeltas(current, component.heads))
			}
			active := false
			for _, head := range component.heads {
				empty, emptyErr := current[head].Empty()
				if emptyErr != nil {
					return errors.Join(emptyErr, closeDeltas(current, component.heads))
				}
				active = active || !empty
			}
			if !active {
				if err = closeDeltas(current, component.heads); err != nil {
					return err
				}
				break
			}
			next, nextErr := newDeltas(component.heads, false)
			if nextErr != nil {
				return errors.Join(nextErr, closeDeltas(current, component.heads))
			}
			for _, scheduled := range component.rules {
				entry := compiled[scheduled.index]
				if entry.ground {
					continue
				}
				if options.AllOrNothingDeltas {
					if err = executeEntry(entry, -1, nil, next[entry.head], options.CompileWithWidening); err != nil {
						return errors.Join(err, closeDeltas(next, component.heads), closeDeltas(current, component.heads))
					}
					continue
				}
				for _, tailIndex := range scheduled.internalTails {
					delta := current[entry.tails[tailIndex]]
					if err = executeEntry(entry, tailIndex, delta, next[entry.head], options.CompileWithWidening); err != nil {
						return errors.Join(err, closeDeltas(next, component.heads), closeDeltas(current, component.heads))
					}
				}
			}
			if err = closeDeltas(current, component.heads); err != nil {
				return errors.Join(err, closeDeltas(next, component.heads))
			}
			current = next
		}
	}
	return nil
}
