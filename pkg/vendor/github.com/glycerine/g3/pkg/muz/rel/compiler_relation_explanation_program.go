package rel

// Source: dl_compiler.cpp compile_preds_init/compile_loop, dispatched through
// the whole-relation product/sieve operators installed by mk_explanations.

import (
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// CompileRelationExplanationRulesWithGuard compiles positive and stratified
// finite-negative explanation rules with whole-relation proof histories.
// Recursive loop controls use product emptiness: proof-only delta growth does
// not extend a loop whose finite tuple delta is empty. Both blocks transfer
// to the caller, and unsupported inputs publish neither block.
func CompileRelationExplanationRulesWithGuard(relations *RelationManager, set *base.RuleSet, guard *rewriter.TheoryRewriter) (*InstructionBlock, *InstructionBlock, error) {
	return compileExplanationRulesWithGuard(relations, set, guard, true)
}
