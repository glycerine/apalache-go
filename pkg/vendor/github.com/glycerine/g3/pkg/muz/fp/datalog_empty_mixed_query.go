package fp

// Source: src/muz/rel/rel_context.cpp rel_context::query. An empty program
// obtains each requested relation independently, so a heterogeneous set of
// finite and infinite outputs in a rule-free context does not require a
// whole-program relation plugin merely to decide whether any output is nonempty.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (e *PropositionalEngine) queryEmptyMixedDatalog(declarations []ast.ASTHandle) (lbool.LBool, bool, error) {
	if e.udocDatalog != nil || e.externalIntervalDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil || e.boundIntervalDatalog != nil {
		return lbool.Undef, false, nil
	}
	count, err := e.set.Len()
	if err != nil || count != 0 {
		return lbool.Undef, false, err
	}
	hasFinite, hasInfinite, nonempty := false, false, false
	for _, declaration := range declarations {
		state := e.predicates[declaration]
		if state == nil || state.external != nil {
			return lbool.Undef, false, nil
		}
		if state.table == nil {
			hasInfinite = true
			continue
		}
		size, err := state.table.Len()
		if err != nil {
			return lbool.Undef, false, err
		}
		hasFinite = true
		nonempty = nonempty || size != 0
	}
	if len(declarations) != 0 && (!hasFinite || !hasInfinite) {
		return lbool.Undef, false, nil
	}
	if err = e.relations.ResetSaturatedMarks(); err != nil {
		return lbool.Undef, true, err
	}
	if err = e.resetNegationDependentTables(); err != nil {
		return lbool.Undef, true, err
	}
	canceled, err := e.manager.Canceled()
	if err != nil {
		return lbool.Undef, true, err
	}
	if canceled {
		return lbool.Undef, true, z3exception.NewDefaultException(e.manager.ResourceLimit().CancelMessage())
	}
	e.compiled = false
	e.datalogStats.Reset()
	if len(declarations) == 0 {
		return lbool.True, true, nil
	}
	return lbool.FromBool(nonempty), true, nil
}
