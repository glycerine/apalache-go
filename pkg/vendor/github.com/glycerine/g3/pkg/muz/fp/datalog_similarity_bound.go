package fp

// Source: rel_context::transform_rules and similarity merge_class/add_fact
// dispatch through the source bound_relation plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (r *boundDatalogRuntime) discardSimilarity() error {
	if r.similarityCleanup != nil {
		if err := r.similarityCleanup.Cleanup(); err != nil {
			return err
		}
		r.similarityCleanup = nil
	}
	if r.similarityRuntime != nil {
		if err := r.similarityRuntime.close(); err != nil {
			return err
		}
		r.similarityRuntime = nil
	}
	if r.similarity != nil {
		if err := r.similarity.Close(); err != nil {
			return err
		}
		r.similarity = nil
	}
	return nil
}

func (r *boundDatalogRuntime) queryWithSimilarity(declarations []ast.ASTHandle, threshold uint32) (lbool.LBool, error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	r.evaluated = nil
	if err := r.discardSimilarity(); err != nil {
		return lbool.Undef, err
	}
	program, err := rel.NewBoundSimilarityProgram(r.manager, r.rules, threshold, declarations)
	if err != nil {
		errors.As(err, &r.similarityCleanup)
		return lbool.Undef, errors.Join(err, r.discardSimilarity())
	}
	// Keep the transformed program reachable before construction of its
	// borrowing evaluator, and retire that evaluator first on every exit.
	r.similarity = program
	rules, err := program.Rules()
	if err != nil {
		return lbool.Undef, errors.Join(err, r.discardSimilarity())
	}
	predicates := append([]ast.ASTHandle(nil), r.order...)
	predicates = append(predicates, program.AuxiliaryPredicates()...)
	evaluator, err := newBoundDatalogRuntime(r.manager, rules, predicates)
	if err != nil {
		errors.As(err, &r.similarityCleanup)
		return lbool.Undef, errors.Join(err, r.discardSimilarity())
	}
	r.similarityRuntime = evaluator
	for _, declaration := range r.order {
		evaluator.entries[declaration].seed = r.entries[declaration].seed.Clone()
	}
	for _, auxiliary := range program.AuxiliaryPredicates() {
		seed, found := program.AuxiliaryRelation(auxiliary)
		if !found {
			return lbool.Undef, errors.Join(ErrBoundDatalogFragment, r.discardSimilarity())
		}
		evaluator.entries[auxiliary].seed = seed.Clone()
	}
	status, err := evaluator.query(declarations)
	if err != nil {
		return lbool.Undef, err
	}
	r.evaluated = make(map[ast.ASTHandle]*rel.BoundRelation, len(r.order))
	for _, declaration := range r.order {
		r.evaluated[declaration] = evaluator.evaluated[declaration]
	}
	return status, nil
}
