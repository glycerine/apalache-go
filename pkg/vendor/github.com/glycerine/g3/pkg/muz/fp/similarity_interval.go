package fp

// Source: rel_context transformation registration and similarity merge_class
// arithmetic fact dispatch through the selected interval_relation plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (a *intervalDatalogActivation) queryWithSimilarity(declarations []ast.ASTHandle, widen bool, threshold uint32) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	if err := a.discardEvaluator(); err != nil {
		return lbool.Undef, err
	}
	program, err := rel.NewIntervalSimilarityProgram(a.manager, a.rules, threshold, declarations)
	if err != nil {
		return lbool.Undef, err
	}
	// The activation owns the program before any evaluator construction can
	// fail; cleanup retry always visits the evaluator before its borrowed rules.
	a.similarity = program
	rules, err := program.Rules()
	if err != nil {
		return lbool.Undef, errors.Join(err, a.discardEvaluator())
	}
	predicates := append([]ast.ASTHandle(nil), a.predicates...)
	seeds := make(map[ast.ASTHandle]*rel.IntervalRelation, len(a.store.entries))
	for declaration, entry := range a.store.entries {
		seeds[declaration] = entry.relation
	}
	for _, auxiliary := range program.AuxiliaryPredicates() {
		seed, found := program.AuxiliaryRelation(auxiliary)
		if !found {
			return lbool.Undef, errors.Join(ErrIntervalDatalogFragment, a.discardEvaluator())
		}
		predicates = append(predicates, auxiliary)
		seeds[auxiliary] = seed
	}
	evaluator, err := NewIntervalDatalogEvaluator(a.manager, rules, predicates, seeds)
	if err != nil {
		return lbool.Undef, errors.Join(err, a.discardEvaluator())
	}
	a.evaluator = evaluator
	if err = evaluator.SaturatePositive(widen); err != nil {
		return lbool.Undef, err
	}
	return evaluator.Query(declarations)
}
