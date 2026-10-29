package dataflow

// Source: src/muz/dataflow/dataflow.h and reachability.h. This file ports the
// concrete reachability instantiation used by the cone-of-influence passes;
// it does not reproduce the C++ generic data-flow template.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

var ErrReachabilityRules = errors.New("muz/dataflow: rules must be finalized")

type reachRule struct {
	head     ast.ASTHandle
	tails    []ast.ASTHandle
	positive int
}

// Reachability is the completed Boolean abstract-domain result. Missing
// predicate declarations denote the source null fact and are unreachable.
type Reachability struct {
	facts map[ast.ASTHandle]bool
}

func snapshot(rules *base.RuleSet) ([]reachRule, map[ast.ASTHandle][]int, map[ast.ASTHandle][]int, error) {
	if rules == nil {
		return nil, nil, nil, ErrReachabilityRules
	}
	finalized, err := rules.IsFinalized()
	if err != nil || !finalized {
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, nil, ErrReachabilityRules
	}
	count, err := rules.Len()
	if err != nil {
		return nil, nil, nil, err
	}
	result := make([]reachRule, 0, count)
	byHead := make(map[ast.ASTHandle][]int)
	byTail := make(map[ast.ASTHandle][]int)
	for i := 0; i < count; i++ {
		rule, e := rules.Rule(i)
		if e != nil {
			return nil, nil, nil, e
		}
		entry, e := snapshotRule(rule)
		if cleanup := rule.Close(); cleanup != nil {
			e = errors.Join(e, cleanup)
		}
		if e != nil {
			return nil, nil, nil, e
		}
		index := len(result)
		result = append(result, entry)
		byHead[entry.head] = append(byHead[entry.head], index)
		for _, tail := range entry.tails {
			byTail[tail] = append(byTail[tail], index)
		}
	}
	return result, byHead, byTail, nil
}

func snapshotRule(rule *base.Rule) (reachRule, error) {
	head, err := rule.HeadDeclaration()
	if err != nil {
		return reachRule{}, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return reachRule{}, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return reachRule{}, err
	}
	entry := reachRule{head: head, positive: positive, tails: make([]ast.ASTHandle, uninterpreted)}
	for i := 0; i < uninterpreted; i++ {
		entry.tails[i], err = rule.TailDeclaration(i)
		if err != nil {
			return reachRule{}, err
		}
	}
	return entry, nil
}

func propagate(queue map[ast.ASTHandle]bool, declaration ast.ASTHandle) {
	queue[declaration] = true
}

// BottomUp computes predicate reachability from rules with no positive tails.
// Negative and interpreted tails do not prevent source reachability_info from
// seeding a head.
func BottomUp(rules *base.RuleSet) (*Reachability, error) {
	snapshot, _, byTail, err := snapshot(rules)
	if err != nil {
		return nil, err
	}
	result := &Reachability{facts: make(map[ast.ASTHandle]bool)}
	current := make(map[ast.ASTHandle]bool)
	for _, rule := range snapshot {
		if rule.positive == 0 && !result.facts[rule.head] {
			result.facts[rule.head] = true
			propagate(current, rule.head)
		}
	}
	for len(current) != 0 {
		next := make(map[ast.ASTHandle]bool)
		for declaration := range current {
			for _, index := range byTail[declaration] {
				rule := snapshot[index]
				if result.facts[rule.head] {
					continue
				}
				reachable := true
				for i := 0; i < rule.positive; i++ {
					if !result.facts[rule.tails[i]] {
						reachable = false
						break
					}
				}
				if reachable {
					result.facts[rule.head] = true
					propagate(next, rule.head)
				}
			}
		}
		current = next
	}
	return result, nil
}

// TopDown computes the cone reachable from output predicates through every
// uninterpreted tail, including negative tails as in the source domain.
func TopDown(rules *base.RuleSet) (*Reachability, error) {
	snapshot, byHead, _, err := snapshot(rules)
	if err != nil {
		return nil, err
	}
	outputs, err := rules.OutputPredicates()
	if err != nil {
		return nil, err
	}
	result := &Reachability{facts: make(map[ast.ASTHandle]bool)}
	current := make(map[ast.ASTHandle]bool)
	for _, declaration := range outputs {
		if len(byHead[declaration]) != 0 {
			result.facts[declaration] = true
			propagate(current, declaration)
		}
	}
	for len(current) != 0 {
		next := make(map[ast.ASTHandle]bool)
		for declaration := range current {
			for _, index := range byHead[declaration] {
				for _, tail := range snapshot[index].tails {
					if !result.facts[tail] {
						result.facts[tail] = true
						propagate(next, tail)
					}
				}
			}
		}
		current = next
	}
	return result, nil
}

func (r *Reachability) IsReachable(declaration ast.ASTHandle) bool {
	return r != nil && r.facts[declaration]
}
