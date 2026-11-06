package fp

// Source: rel_context's priority-5000 similarity compressor before Karr rule
// execution; generated auxiliary facts use the Karr relation's source behavior.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (a *karrDatalogActivation) queryWithSimilarity(declarations []ast.ASTHandle, threshold uint32) (status lbool.LBool, resultErr error) {
	if a == nil || a.closed || threshold <= 1 {
		return lbool.Undef, ErrKarrDatalogFragment
	}
	a.results = nil
	program, err := rel.NewKarrSimilarityProgram(a.manager, a.rules, threshold, declarations)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, program.Close())
		if resultErr != nil {
			status = lbool.Undef
			a.results = nil
		}
	}()
	rules, err := program.Rules()
	if err != nil {
		return lbool.Undef, err
	}
	seeds := make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds)+len(program.AuxiliaryPredicates()))
	for declaration, seed := range a.seeds {
		seeds[declaration] = seed.Clone()
	}
	for _, auxiliary := range program.AuxiliaryPredicates() {
		seed, found := program.AuxiliaryRelation(auxiliary)
		if !found {
			return lbool.Undef, errors.Join(ErrKarrDatalogFragment, program.Close())
		}
		seeds[auxiliary] = seed.Clone()
	}
	working := &karrDatalogActivation{
		manager:        a.manager,
		rules:          rules,
		seeds:          seeds,
		formulaChecker: a.formulaChecker,
		formulaConfig:  a.formulaConfig,
	}
	status, resultErr = working.query(declarations)
	if resultErr != nil {
		return lbool.Undef, resultErr
	}
	// Publish only declarations owned by the original activation. Similarity
	// auxiliaries are query-local and their declarations retire with the program.
	results := make(map[ast.ASTHandle]*rel.KarrRelation, len(a.seeds))
	for declaration := range a.seeds {
		if result := working.results[declaration]; result != nil {
			results[declaration] = result
		}
	}
	for _, declaration := range declarations {
		if result := working.results[declaration]; result != nil {
			results[declaration] = result
		}
	}
	a.results = results
	return status, nil
}
