package rel

// Source: dl_compiler.cpp order_rules/compile_strats, specialized to positive
// rules over the registered finite/explanation product, including source
// semi-naive delta/fixpoint lowering for recursive dependency components.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// CompileExplanationRulesWithGuard compiles a positive transformed explanation
// rule set, including its finite output bridges, into dependency-ordered
// execution and termination blocks. Both blocks transfer to
// the caller. Unsupported rules publish neither block.
func CompileExplanationRulesWithGuard(relations *RelationManager, set *base.RuleSet, guard *rewriter.TheoryRewriter) (*InstructionBlock, *InstructionBlock, error) {
	return compileExplanationRulesWithGuard(relations, set, guard, false)
}

func compileExplanationRulesWithGuard(relations *RelationManager, set *base.RuleSet, guard *rewriter.TheoryRewriter, relationLevel bool) (execution, termination *InstructionBlock, resultErr error) {
	if err := relations.open(); err != nil {
		return nil, nil, err
	}
	if set == nil {
		return nil, nil, ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return nil, nil, err
	}
	var rules []*base.Rule
	var plans []explanationRuleLayout
	defer func() {
		for _, rule := range rules {
			resultErr = errors.Join(resultErr, rule.Close())
		}
		if resultErr != nil {
			if execution != nil {
				resultErr = errors.Join(resultErr, execution.Close())
			}
			if termination != nil {
				resultErr = errors.Join(resultErr, termination.Close())
			}
			execution, termination = nil, nil
		}
	}()
	heads := make([]ast.ASTHandle, 0)
	byHead := make(map[ast.ASTHandle][]int)
	dependencies := make(map[ast.ASTHandle][]ast.ASTHandle)
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return nil, nil, err
		}
		rules = append(rules, rule)
		plan, err := planExplanationRule(relations, rule)
		if err != nil {
			return nil, nil, fmt.Errorf("compile explanation rule %d: %w", index, err)
		}
		plans = append(plans, plan)
		if len(plan.conditions) != 0 && guard == nil {
			return nil, nil, ErrCompilerUnsupported
		}
		if _, found := byHead[plan.head]; !found {
			heads = append(heads, plan.head)
		}
		byHead[plan.head] = append(byHead[plan.head], index)
		if !plan.tail.IsNull() {
			dependencies[plan.head] = append(dependencies[plan.head], plan.tail)
		}
		for _, join := range plan.joins {
			dependencies[plan.head] = append(dependencies[plan.head], join.predicate)
		}
		for _, negative := range plan.negatives {
			dependencies[plan.head] = append(dependencies[plan.head], negative.predicate)
		}
	}
	if err := ValidateExplanationNegativeRelations(set); err != nil {
		return nil, nil, err
	}
	finalized, err := set.IsFinalized()
	if err != nil {
		return nil, nil, err
	}
	if !finalized {
		valid, err := set.Finalize()
		if err != nil {
			return nil, nil, err
		}
		if !valid {
			return nil, nil, ErrCompilerUnsupported
		}
	}
	groups := make(map[uint32][]ast.ASTHandle)
	var maximum uint32
	for _, head := range heads {
		stratum, err := set.PredicateStratum(head)
		if err != nil {
			return nil, nil, err
		}
		groups[stratum] = append(groups[stratum], head)
		if stratum > maximum {
			maximum = stratum
		}
	}
	execution, termination = NewInstructionBlock(), NewInstructionBlock()
	registers := explanationRuleRegisters{inputTail: -1, outputDelta: VoidRegister, relationLevel: relationLevel}
	appendRuntime := func(destination *InstructionBlock, instruction Instruction) error {
		if relationLevel {
			switch instruction.(type) {
			case *whileInstruction, *saturationInstruction:
			default:
				converted, err := relationExplanationInstructionFor(instruction)
				if err != nil {
					return errors.Join(err, instruction.Close())
				}
				instruction = converted
			}
		}
		if err := destination.Append(instruction); err != nil {
			return errors.Join(err, instruction.Close())
		}
		return nil
	}
	appendRule := func(destination *InstructionBlock, index int) error {
		block, err := compileExplanationRuleWithRegisters(relations, rules[index], guard, &registers)
		if err != nil {
			return err
		}
		destination.instructions = append(destination.instructions, block.instructions...)
		block.instructions = nil
		return block.Close()
	}
	for stratum := uint32(0); stratum <= maximum; stratum++ {
		group := groups[stratum]
		if len(group) == 0 {
			continue
		}
		// Source compile_strats skips completed dependency components when
		// compiling again after an interrupted saturation round.
		allSaturated := true
		for _, head := range group {
			saturated, err := relations.IsSaturated(head)
			if err != nil {
				return execution, termination, err
			}
			allSaturated = allSaturated && saturated
		}
		if allSaturated {
			continue
		}
		inside := make(map[ast.ASTHandle]bool, len(group))
		keys := make([]finitePredicateOrderKey, 0, len(group))
		keyHeads := make(map[finitePredicateOrderKey]ast.ASTHandle, len(group))
		keyDependencies := make(map[finitePredicateOrderKey][]finitePredicateOrderKey)
		recursive := len(group) > 1
		for _, head := range group {
			inside[head] = true
			key := finitePredicateKey(head)
			keys = append(keys, key)
			keyHeads[key] = head
			seenDependencies := make(map[finitePredicateOrderKey]bool)
			for _, tail := range dependencies[head] {
				tailKey := finitePredicateKey(tail)
				if !seenDependencies[tailKey] {
					keyDependencies[key] = append(keyDependencies[key], tailKey)
					seenDependencies[tailKey] = true
				}
				recursive = recursive || tail == head
			}
		}
		if !recursive {
			for _, index := range byHead[group[0]] {
				if err := appendRule(execution, index); err != nil {
					return execution, termination, err
				}
			}
		} else {
			ordered, _ := sourceDependencyOrder(keys, keyDependencies)
			if len(ordered) != len(group) {
				return execution, termination, ErrCompilerUnsupported
			}
			group = group[:0]
			for _, key := range ordered {
				group = append(group, keyHeads[key])
			}
			oldDeltas, newDeltas := make(map[ast.ASTHandle]uint32), make(map[ast.ASTHandle]uint32)
			for _, head := range group {
				oldDeltas[head] = registers.next
				registers.next++
				newDeltas[head] = registers.next
				registers.next++
			}
			// compile_preds_init runs outside-only seed rules, then clones
			// every full head, including facts present before compilation.
			for _, head := range group {
				for _, index := range byHead[head] {
					plan := plans[index]
					dependsInside := inside[plan.tail]
					for _, join := range plan.joins {
						dependsInside = dependsInside || inside[join.predicate]
					}
					if !dependsInside {
						if err := appendRule(execution, index); err != nil {
							return execution, termination, err
						}
					}
				}
				load, err := NewLoadInstruction(relations, head, oldDeltas[head])
				if err != nil {
					return execution, termination, err
				}
				if err := appendRuntime(execution, load); err != nil {
					return execution, termination, err
				}
			}
			body := NewInstructionBlock()
			// Transfer the body immediately so all later failures have one
			// owning cleanup path through the enclosing execution block.
			controls := make([]uint32, 0, len(group))
			for _, head := range group {
				controls = append(controls, oldDeltas[head])
			}
			loop := NewWhileLoopInstruction(controls, body)
			if err := appendRuntime(execution, loop); err != nil {
				return execution, termination, err
			}
			for _, head := range group {
				if err := appendRuntime(body, NewDeallocateInstruction(newDeltas[head])); err != nil {
					return execution, termination, err
				}
				for _, index := range byHead[head] {
					plan := plans[index]
					tails := []ast.ASTHandle{plan.tail}
					for _, join := range plan.joins {
						tails = append(tails, join.predicate)
					}
					for position, tail := range tails {
						if !inside[tail] {
							continue
						}
						registers.inputTail, registers.inputDelta = position, oldDeltas[tail]
						registers.outputDelta = newDeltas[head]
						if err := appendRule(body, index); err != nil {
							return execution, termination, err
						}
					}
				}
			}
			registers.inputTail, registers.outputDelta = -1, VoidRegister
			for _, head := range group {
				if err := appendRuntime(body, NewMoveInstruction(newDeltas[head], oldDeltas[head])); err != nil {
					return execution, termination, err
				}
			}
		}
		for _, head := range group {
			mark, err := NewMarkSaturatedInstruction(relations, head)
			if err != nil {
				return execution, termination, err
			}
			// Per-rule STORE instructions already publish completed results.
			// Mark this component only after every rule/recursive loop finishes.
			if err := appendRuntime(execution, mark); err != nil {
				return execution, termination, err
			}
		}
	}
	return execution, termination, nil
}
