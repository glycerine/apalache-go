package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp join_planner global pair queue,
// registration, stratum accounting, and binary rule application.

import (
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type simpleJoinRuleState struct {
	rule     *base.Rule
	head     ast.ASTHandle
	prefix   usymbol.Symbol
	name     usymbol.Symbol
	tails    []ast.ASTHandle
	negated  []bool
	positive int
	modified bool
}

type simpleJoinGroup struct {
	key           normalizedPairKey
	cost          float32
	consumers     uint32
	stratified    bool
	outputs       map[uint32]struct{}
	registrations uint32
	parentRule    *base.Rule
	parentPrefix  usymbol.Symbol
}

func (p *SimpleJoinsPlugin) statePairOutside(state *simpleJoinRuleState, left, right int) []ast.ASTHandle {
	outside := make([]ast.ASTHandle, 0, len(state.tails)-1)
	outside = append(outside, state.head)
	for index, tail := range state.tails {
		if index != left && index != right {
			outside = append(outside, tail)
		}
	}
	return outside
}

// activeJoinGroups rebuilds the live pair graph after each source-style batch
// application. Cost and stratification entries persist while a normalized
// pair remains active, matching pair_info's first-cost and sticky-stratum
// accounting; entries disappear when their final edge disappears.
func (p *SimpleJoinsPlugin) activeJoinGroups(states []*simpleJoinRuleState, strata *base.RuleSet, costs map[normalizedPairKey]float32, stratified map[normalizedPairKey]bool, outputSets map[normalizedPairKey]map[uint32]struct{}, registrationHistory map[normalizedPairKey][]*simpleJoinRuleState, traceCounts map[normalizedPairKey]map[*simpleJoinRuleState]int) ([]*simpleJoinGroup, error) {
	groups := make(map[normalizedPairKey]*simpleJoinGroup)
	order := make([]normalizedPairKey, 0)
	active := make(map[normalizedPairKey]bool)
	traceSeen := make(map[normalizedPairKey]map[*simpleJoinRuleState]int)
	initialRegistrations := make(map[normalizedPairKey][]*simpleJoinRuleState)
	for _, state := range states {
		for left := 0; left+1 < state.positive; left++ {
			for right := left + 1; right < state.positive; right++ {
				pair, err := p.normalizePair(state.tails[left], state.tails[right])
				if err != nil {
					return nil, err
				}
				initialRegistrations[pair.key] = append(initialRegistrations[pair.key], state)
			}
		}
	}
	for key, registrations := range initialRegistrations {
		if registrationHistory[key] == nil {
			registrationHistory[key] = append([]*simpleJoinRuleState(nil), registrations...)
		}
	}
	for _, state := range states {
		for left := 0; left+1 < state.positive; left++ {
			for right := left + 1; right < state.positive; right++ {
				pair, err := p.normalizePair(state.tails[left], state.tails[right])
				if err != nil {
					return nil, err
				}
				outside := p.statePairOutside(state, left, right)
				cost, err := p.estimatedPairCost(state.tails[:state.positive], outside, left, right)
				if err != nil {
					return nil, err
				}
				group := groups[pair.key]
				if group == nil {
					group = &simpleJoinGroup{key: pair.key, outputs: make(map[uint32]struct{})}
					groups[pair.key] = group
					order = append(order, pair.key)
					if _, found := costs[pair.key]; !found {
						costs[pair.key] = cost
						stratified[pair.key] = true
						_ = p.tracePairCost(state.tails[left], state.tails[right], cost)
					}
					group.cost = costs[pair.key]
					group.stratified = stratified[pair.key]
				}
				active[pair.key] = true
				group.registrations++
				group.parentRule = state.rule
				group.parentPrefix = state.prefix
				if state.positive > 2 {
					group.consumers++
				}
				live := make(map[uint32]ast.ASTHandle)
				for _, expression := range outside {
					if err = p.variables(expression, live); err != nil {
						return nil, err
					}
				}
				for variable, canonical := range pair.actualToCanonical {
					if _, needed := live[variable]; needed {
						group.outputs[canonical] = struct{}{}
					}
				}
				leftDecl, err := p.manager.GetDecl(state.tails[left])
				if err != nil {
					return nil, err
				}
				rightDecl, err := p.manager.GetDecl(state.tails[right])
				if err != nil {
					return nil, err
				}
				headDecl, err := p.manager.GetDecl(state.head)
				if err != nil {
					return nil, err
				}
				leftStratum, leftErr := strata.PredicateStratum(leftDecl)
				rightStratum, rightErr := strata.PredicateStratum(rightDecl)
				headStratum, headErr := strata.PredicateStratum(headDecl)
				if leftErr != nil || rightErr != nil || headErr != nil {
					group.stratified = false
					stratified[pair.key] = false
				} else {
					sourceStratum := leftStratum
					if rightStratum > sourceStratum {
						sourceStratum = rightStratum
					}
					if headStratum <= sourceStratum {
						group.stratified = false
						stratified[pair.key] = false
					}
				}
				if traceSeen[pair.key] == nil {
					traceSeen[pair.key] = make(map[*simpleJoinRuleState]int)
				}
				traceSeen[pair.key][state]++
				if traceCounts[pair.key] == nil {
					traceCounts[pair.key] = make(map[*simpleJoinRuleState]int)
				}
				if traceSeen[pair.key][state] > traceCounts[pair.key][state] {
					_ = p.tracePairRegistration(state, state.tails[left], state.tails[right], outside, group.cost, group.consumers, group.stratified)
					traceCounts[pair.key][state]++
				}
			}
		}
	}
	for key := range costs {
		if !active[key] {
			delete(costs, key)
			delete(stratified, key)
			delete(outputSets, key)
			delete(registrationHistory, key)
			delete(traceCounts, key)
		}
	}
	result := make([]*simpleJoinGroup, 0, len(order))
	for _, key := range order {
		group := groups[key]
		if previous := outputSets[key]; previous != nil {
			for output := range previous {
				group.outputs[output] = struct{}{}
			}
		}
		if outputSets[key] == nil {
			outputSets[key] = make(map[uint32]struct{})
		}
		for output := range group.outputs {
			outputSets[key][output] = struct{}{}
		}
		group.cost = costs[key]
		group.stratified = stratified[key]
		group.registrations = uint32(len(registrationHistory[key]))
		if len(registrationHistory[key]) != 0 {
			last := registrationHistory[key][len(registrationHistory[key])-1]
			group.parentRule = last.rule
			group.parentPrefix = last.prefix
		}
		if group.consumers != 0 {
			result = append(result, group)
		}
	}
	return result, nil
}

func chooseSimpleJoinGroup(groups []*simpleJoinGroup) *simpleJoinGroup {
	var best *simpleJoinGroup
	var bestCost float32
	for _, group := range groups {
		cost := group.cost / float32(group.consumers)
		if group.stratified {
			if cost > 0 {
				cost /= 16
			} else {
				cost *= 16
			}
		}
		if best == nil || cost < bestCost {
			best, bestCost = group, cost
		}
	}
	return best
}

func groupOutputs(group *simpleJoinGroup, history map[normalizedPairKey]map[uint32]struct{}) map[normalizedPairKey][]uint32 {
	outputs := make(map[normalizedPairKey][]uint32, 1)
	set := history[group.key]
	values := make([]uint32, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	outputs[group.key] = values
	return outputs
}

func (p *SimpleJoinsPlugin) applySimpleJoinGroup(group *simpleJoinGroup, outputs map[normalizedPairKey][]uint32, shared map[sharedJoinKey]sharedJoinDefinition, introduced *[]*base.Rule, registrationHistory map[normalizedPairKey][]*simpleJoinRuleState, traceCounts map[normalizedPairKey]map[*simpleJoinRuleState]int) error {
	prefix := group.parentPrefix
	if group.registrations > 1 {
		name, err := p.manager.Symbols.String(prefix)
		if err != nil {
			return err
		}
		prefix, err = p.manager.Symbols.Intern(fmt.Sprintf("%s_and_%d", name, group.registrations-1))
		if err != nil {
			return err
		}
	}
	ordered := make([]*simpleJoinRuleState, 0)
	seen := make(map[*simpleJoinRuleState]bool)
	for _, state := range registrationHistory[group.key] {
		if !seen[state] {
			seen[state] = true
			ordered = append(ordered, state)
		}
	}
	for _, state := range ordered {
		before, err := p.statePairRegistrationCounts(state)
		if err != nil {
			return err
		}
		for {
			joined := false
			for left := 0; left+1 < state.positive && !joined; left++ {
				for right := left + 1; right < state.positive; right++ {
					pair, err := p.normalizePair(state.tails[left], state.tails[right])
					if err != nil {
						return err
					}
					if pair.key != group.key {
						continue
					}
					outside := p.statePairOutside(state, left, right)
					auxiliary, definition, err := p.auxiliary(pair, outside, prefix, group.parentRule, shared, outputs)
					if err != nil {
						return err
					}
					if definition != nil {
						*introduced = append(*introduced, definition)
					}
					state.tails[left] = auxiliary
					state.negated[left] = false
					lastPositive := state.positive - 1
					if right != lastPositive {
						state.tails[right] = state.tails[lastPositive]
						state.negated[right] = state.negated[lastPositive]
					}
					copy(state.tails[lastPositive:], state.tails[lastPositive+1:])
					copy(state.negated[lastPositive:], state.negated[lastPositive+1:])
					state.tails = state.tails[:len(state.tails)-1]
					state.negated = state.negated[:len(state.negated)-1]
					state.positive--
					state.modified = true
					joined = true
					break
				}
			}
			if !joined {
				break
			}
		}
		after, err := p.statePairRegistrationCounts(state)
		if err != nil {
			return err
		}
		if err = updateSimpleJoinRegistrationHistory(registrationHistory, traceCounts, state, before, after); err != nil {
			return err
		}
	}
	return nil
}

func updateSimpleJoinRegistrationHistory(history map[normalizedPairKey][]*simpleJoinRuleState, traceCounts map[normalizedPairKey]map[*simpleJoinRuleState]int, state *simpleJoinRuleState, before, after map[normalizedPairKey]int) error {
	for key, count := range before {
		removed := count - after[key]
		for range removed {
			registrations := history[key]
			removedRegistration := false
			for index, registered := range registrations {
				if registered == state {
					history[key] = append(registrations[:index], registrations[index+1:]...)
					removedRegistration = true
					break
				}
			}
			if !removedRegistration {
				return fmt.Errorf("muz/transforms: registered simple-join pair edge is missing")
			}
			if traceCounts[key][state] > 0 {
				traceCounts[key][state]--
			}
		}
	}
	for key, count := range after {
		for range count - before[key] {
			history[key] = append(history[key], state)
		}
	}
	return nil
}

func (p *SimpleJoinsPlugin) statePairRegistrationCounts(state *simpleJoinRuleState) (map[normalizedPairKey]int, error) {
	counts := make(map[normalizedPairKey]int)
	for left := 0; left+1 < state.positive; left++ {
		for right := left + 1; right < state.positive; right++ {
			pair, err := p.normalizePair(state.tails[left], state.tails[right])
			if err != nil {
				return nil, err
			}
			counts[pair.key]++
		}
	}
	return counts, nil
}

func newSimpleJoinState(p *SimpleJoinsPlugin, rule *base.Rule) (*simpleJoinRuleState, error) {
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	declaration, err := p.manager.GetDecl(head)
	if err != nil {
		return nil, err
	}
	prefix, err := p.manager.GetASTName(declaration)
	if err != nil {
		return nil, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	tailCount, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	state := &simpleJoinRuleState{rule: rule, head: head, prefix: prefix, name: name, positive: positive, tails: make([]ast.ASTHandle, tailCount), negated: make([]bool, tailCount)}
	for i := range state.tails {
		state.tails[i], state.negated[i], err = rule.Tail(i)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}
