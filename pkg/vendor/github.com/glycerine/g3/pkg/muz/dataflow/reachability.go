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
	facts      map[ast.ASTHandle]bool
	known      map[ast.ASTHandle]bool
	knownOrder []ast.ASTHandle
}

func (r *Reachability) markKnown(declaration ast.ASTHandle) {
	if r.known[declaration] {
		return
	}
	r.known[declaration] = true
	r.knownOrder = append(r.knownOrder, declaration)
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

func propagate(queue *[]ast.ASTHandle, declaration ast.ASTHandle) {
	*queue = append(*queue, declaration)
}

// BottomUp computes predicate reachability from rules with no positive tails.
// Negative and interpreted tails do not prevent source reachability_info from
// seeding a head.
func BottomUp(rules *base.RuleSet) (*Reachability, error) {
	snapshot, _, byTail, err := snapshot(rules)
	if err != nil {
		return nil, err
	}
	result := &Reachability{facts: make(map[ast.ASTHandle]bool), known: make(map[ast.ASTHandle]bool)}
	var current []ast.ASTHandle
	for _, rule := range snapshot {
		if rule.positive == 0 {
			result.markKnown(rule.head)
			if !result.facts[rule.head] {
				result.facts[rule.head] = true
				propagate(&current, rule.head)
			}
		}
	}
	for len(current) != 0 {
		var next []ast.ASTHandle
		for _, declaration := range current {
			for _, index := range byTail[declaration] {
				rule := snapshot[index]
				// Source step_bottom_up inserts the head's default fact before
				// propagate_up, even when another positive tail is unreachable.
				result.markKnown(rule.head)
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
					propagate(&next, rule.head)
				}
			}
		}
		current = next
	}
	return result, nil
}

// TopDown computes the cone reachable from output predicates through every
// uninterpreted tail, including negative tails as in the source domain.
// Finalized nested dependencies also participate so quantified preprocessing
// cannot hide a live predicate inside an interpreted tail.
func TopDown(rules *base.RuleSet) (*Reachability, error) {
	snapshot, byHead, _, err := snapshot(rules)
	if err != nil {
		return nil, err
	}
	outputs, err := rules.OutputPredicates()
	if err != nil {
		return nil, err
	}
	result := &Reachability{facts: make(map[ast.ASTHandle]bool), known: make(map[ast.ASTHandle]bool)}
	var current []ast.ASTHandle
	for _, declaration := range outputs {
		if len(byHead[declaration]) != 0 {
			result.markKnown(declaration)
			result.facts[declaration] = true
			propagate(&current, declaration)
		}
	}
	for len(current) != 0 {
		var next []ast.ASTHandle
		for _, declaration := range current {
			for _, index := range byHead[declaration] {
				for _, tail := range snapshot[index].tails {
					result.markKnown(tail)
					if !result.facts[tail] {
						result.facts[tail] = true
						propagate(&next, tail)
					}
				}
			}
			// Keep top-level source visitation order, then include finalized
			// dependencies nested under interpreted/quantified expressions.
			dependencies, err := rules.Dependencies(declaration)
			if err != nil {
				return nil, err
			}
			for _, dependency := range dependencies {
				result.markKnown(dependency)
				if !result.facts[dependency] {
					result.facts[dependency] = true
					propagate(&next, dependency)
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

// KnownDeclarations returns the declarations for which the source dataflow
// fact database constructed an entry. A missing declaration still denotes the
// null fact, but is distinct when a consumer enumerates the database.
func (r *Reachability) KnownDeclarations() []ast.ASTHandle {
	if r == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.knownOrder...)
}
