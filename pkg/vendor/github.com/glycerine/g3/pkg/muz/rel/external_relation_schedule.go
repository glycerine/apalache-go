package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// ExternalRuleExecutionOptions are the source compiler switches that affect
// recursive relation callbacks. AllOrNothingDeltas is exposed so callers never
// silently receive the default semi-naive callback schedule.
type ExternalRuleExecutionOptions struct {
	CompileWithWidening bool
	AllOrNothingDeltas  bool
}

type externalScheduledRule struct {
	index         int
	internalTails []int
}

type externalRuleComponent struct {
	stratum   uint32
	recursive bool
	heads     []ast.ASTHandle
	rules     []externalScheduledRule
}

func planExternalRuleComponents(set *base.RuleSet, compiled []externalFactRule) ([]externalRuleComponent, error) {
	finalized, err := set.IsFinalized()
	if err != nil {
		return nil, err
	}
	if !finalized {
		valid, cause := set.Finalize()
		if cause != nil {
			return nil, cause
		}
		if !valid {
			return nil, ErrCompilerUnsupported
		}
	}
	maximum := uint32(0)
	used := make(map[uint32]bool)
	for index := range compiled {
		compiled[index].stratum, err = set.PredicateStratum(compiled[index].head)
		if err != nil {
			return nil, err
		}
		if compiled[index].stratum > maximum {
			maximum = compiled[index].stratum
		}
		used[compiled[index].stratum] = true
	}
	if len(compiled) == 0 {
		return nil, nil
	}
	components := make([]externalRuleComponent, 0, len(used))
	componentIndex := make(map[uint32]int, len(used))
	for stratum := uint32(0); stratum <= maximum; stratum++ {
		if used[stratum] {
			componentIndex[stratum] = len(components)
			components = append(components, externalRuleComponent{stratum: stratum})
		}
	}
	headSeen := make([]map[ast.ASTHandle]bool, len(components))
	for index := range compiled {
		entry := &compiled[index]
		componentAt := componentIndex[entry.stratum]
		component := &components[componentAt]
		if headSeen[componentAt] == nil {
			headSeen[componentAt] = make(map[ast.ASTHandle]bool)
		}
		if !headSeen[componentAt][entry.head] {
			headSeen[componentAt][entry.head] = true
			component.heads = append(component.heads, entry.head)
		}
		scheduled := externalScheduledRule{index: index}
		for tailIndex, tail := range entry.tails {
			tailStratum, cause := set.PredicateStratum(tail)
			if cause != nil {
				return nil, cause
			}
			if tailStratum == entry.stratum {
				component.recursive = true
				scheduled.internalTails = append(scheduled.internalTails, tailIndex)
			}
		}
		component.rules = append(component.rules, scheduled)
	}
	return components, nil
}
