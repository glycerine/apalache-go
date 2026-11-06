package fp

// Source: dl_compiler.cpp compile_preds_init/compile_dependent_rules and
// dl_instruction.cpp instr_union/while_loop. This widening consumer admits
// recursive strata whose rules have up to two positive tails.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type boundIntervalWideningRule struct {
	index int
	head  ast.ASTHandle
	tails []ast.ASTHandle
}

type boundIntervalWideningStratum struct {
	heads []boundIntervalWideningHead
}

type boundIntervalWideningHead struct {
	head      ast.ASTHandle
	initial   []int
	recursive []boundIntervalWideningVariant
	all       []boundIntervalWideningVariant
}

type boundIntervalWideningVariant struct {
	rule      int
	tail      int // -1 selects full relations for all-or-nothing compilation
	predicate ast.ASTHandle
}

func (a *boundIntervalDatalogActivation) wideningRule(index int, rule *base.Rule) (_ boundIntervalWideningRule, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	plan := boundIntervalWideningRule{index: index}
	var err error
	plan.head, err = rule.HeadDeclaration()
	if err != nil || a.entries[plan.head] == nil {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	positive, err := rule.PositiveTailSize()
	if err != nil || positive > 2 {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	total, err := rule.TailSize()
	if err != nil || positive == 0 && total != 0 {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	for i := 0; i < positive; i++ {
		tail, err := rule.TailDeclaration(i)
		if err != nil || a.entries[tail] == nil {
			return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		plan.tails = append(plan.tails, tail)
	}
	return plan, nil
}

func (a *boundIntervalDatalogActivation) wideningPlan() ([]boundIntervalWideningStratum, error) {
	count, err := a.rules.Len()
	if err != nil {
		return nil, err
	}
	byHead := make(map[ast.ASTHandle][]boundIntervalWideningRule)
	for i := 0; i < count; i++ {
		if err = a.checkpoint(); err != nil {
			return nil, err
		}
		rule, err := a.rules.Rule(i)
		if err != nil {
			return nil, err
		}
		plan, err := a.wideningRule(i, rule)
		if err != nil {
			return nil, err
		}
		byHead[plan.head] = append(byHead[plan.head], plan)
	}
	// Source saturation closes the program before reading its dependency-first
	// strata. The activation borrows this same engine-owned rule set.
	finalized, err := a.rules.IsFinalized()
	if err != nil {
		return nil, err
	}
	if !finalized {
		valid, err := a.rules.Finalize()
		if err != nil || !valid {
			return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
	}
	strata, err := a.rules.Strata()
	if err != nil {
		return nil, err
	}
	result := make([]boundIntervalWideningStratum, 0, len(strata))
	for _, predicates := range strata {
		if len(predicates) == 0 {
			continue
		}
		inside := make(map[ast.ASTHandle]bool, len(predicates))
		dependencies := make(map[ast.ASTHandle][]ast.ASTHandle, len(predicates))
		for _, head := range predicates {
			inside[head] = true
			dependencies[head], err = a.rules.Dependencies(head)
			if err != nil {
				return nil, err
			}
		}
		ordered, err := rel.SourceRecursivePredicateOrder(predicates, dependencies)
		if err != nil {
			return nil, err
		}
		plan := boundIntervalWideningStratum{}
		for _, head := range ordered {
			headPlan := boundIntervalWideningHead{head: head}
			for _, rule := range byHead[head] {
				headPlan.all = append(headPlan.all, boundIntervalWideningVariant{rule: rule.index, tail: -1})
				if len(rule.tails) == 0 {
					continue // Ground rules have already initialized their targets.
				}
				recursive := false
				// Each SCC-tail occurrence selects that predicate's old input
				// register while its other operands read live full relations.
				for index, tail := range rule.tails {
					if inside[tail] {
						recursive = true
						headPlan.recursive = append(headPlan.recursive, boundIntervalWideningVariant{rule: rule.index, tail: index, predicate: tail})
					}
				}
				if !recursive {
					headPlan.initial = append(headPlan.initial, rule.index)
				}
			}
			plan.heads = append(plan.heads, headPlan)
		}
		result = append(result, plan)
	}
	return result, nil
}

// All-or-nothing compilation runs every head rule in each loop, including
// ground rules. Construct the ground relation independently before widening
// it into the target, just as the compiler assembles a fresh head register.
func (a *boundIntervalDatalogActivation) evaluateWideningRule(index int, head ast.ASTHandle, inputs boundIntervalRuleInputs, products map[ast.ASTHandle]*rel.IntervalBoundProduct, delta *rel.IntervalBoundProduct) error {
	rule, err := a.rules.Rule(index)
	if err != nil {
		return err
	}
	tails, err := rule.TailSize()
	if err != nil {
		return errors.Join(err, rule.Close())
	}
	if tails != 0 {
		_, err = a.evaluatePositiveRuleInto(rule, inputs, products, delta, true)
		return err
	}
	entry := a.entries[head]
	source, selected, err := rel.NewSelectedIntervalBoundProduct(a.manager, entry.sorts, entry.kinds, true)
	if err != nil || !selected {
		return errors.Join(err, ErrBoundIntervalDatalogFragment, rule.Close())
	}
	if err = a.applyFactRule(rule, map[ast.ASTHandle]*rel.IntervalBoundProduct{head: source}); err != nil {
		return err
	}
	_, err = products[head].Union(source, delta, true)
	return err
}

func (a *boundIntervalDatalogActivation) saturateWithWidening(products map[ast.ASTHandle]*rel.IntervalBoundProduct, allOrNothing bool) error {
	plan, err := a.wideningPlan()
	if err != nil {
		return err
	}
	for _, stratum := range plan {
		current := make(map[ast.ASTHandle]*rel.IntervalBoundProduct, len(stratum.heads))
		recursive := false
		for _, head := range stratum.heads {
			recursive = recursive || len(head.recursive) != 0
		}
		for _, head := range stratum.heads {
			for _, index := range head.initial {
				if err = a.checkpoint(); err != nil {
					return err
				}
				rule, err := a.rules.Rule(index)
				if err != nil {
					return err
				}
				if _, err = a.evaluatePositiveRule(rule, products); err != nil {
					return err
				}
			}
			if recursive {
				current[head.head], err = products[head.head].Clone()
				if err != nil {
					return err
				}
			}
		}
		if !recursive {
			continue
		}
		for {
			active := false
			for _, head := range stratum.heads {
				active = active || !current[head.head].Empty()
			}
			if !active {
				break
			}
			next := make(map[ast.ASTHandle]*rel.IntervalBoundProduct, len(stratum.heads))
			for _, head := range stratum.heads {
				loop := head.recursive
				if allOrNothing {
					loop = head.all
				}
				for _, variant := range loop {
					if err = a.checkpoint(); err != nil {
						return err
					}
					inputs := boundIntervalRuleInputs{products: products}
					if variant.tail >= 0 {
						delta := current[variant.predicate]
						// An absent delta is an empty source register, not the
						// selector's nil override meaning a full relation.
						if delta.Empty() {
							continue
						}
						inputs.deltaTail, inputs.delta = variant.tail, delta
					}
					if next[head.head] == nil {
						entry := a.entries[head.head]
						var selected bool
						next[head.head], selected, err = rel.NewSelectedIntervalBoundProduct(a.manager, entry.sorts, entry.kinds, true)
						if err != nil || !selected {
							return errors.Join(err, ErrBoundIntervalDatalogFragment)
						}
					}
					if err = a.evaluateWideningRule(variant.rule, head.head, inputs, products, next[head.head]); err != nil {
						return err
					}
					// instr_union clears an empty delta after EACH variant.
					// Hidden component changes must not combine across runs.
					if next[head.head].Empty() {
						delete(next, head.head)
					}
				}
			}
			// All SCC predicates use global deltas in the source. Move
			// every output together, only after the complete iteration.
			current = next
		}
	}
	return nil
}
