package rel

// Source: dl_compiler.cpp concrete instruction dispatch through the product
// and sieve relation plugins registered by translate_rel_level_relation.

import (
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// CompileRelationExplanationRuleWithGuard lowers one transformed rule with
// typed whole-relation explanation registers. Its result preserves e_union
// aggregation across all finite tuples belonging to the same relation.
func CompileRelationExplanationRuleWithGuard(relations *RelationManager, rule *base.Rule, guard *rewriter.TheoryRewriter) (*InstructionBlock, error) {
	registers := explanationRuleRegisters{inputTail: -1, outputDelta: VoidRegister, relationLevel: true}
	return compileExplanationRuleWithRegisters(relations, rule, guard, &registers)
}
