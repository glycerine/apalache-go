package fp

// Source: src/muz/rel/rel_context.cpp rel_context::query(unsigned,...).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// RelationsAnswer returns the conjunction of relation formulas produced by the
// latest Datalog relation-array query. The caller owns the returned reference.
func (e *PropositionalEngine) RelationsAnswer() (answer ast.ASTRef, resultErr error) {
	if err := e.requireResultSnapshot(EngineDatalog); err != nil {
		return answer, err
	}
	if !e.queryRelationsSet {
		return answer, ErrPropositionalEngine
	}
	if e.externalIntervalDatalog != nil {
		return answer, ErrExternalIntervalDatalogFragment
	}
	if e.lastResult == lbool.False {
		h, err := e.manager.MkFalse()
		if err != nil {
			return answer, err
		}
		return e.manager.NewRef(h)
	}
	if e.lastResult != lbool.True && !(e.lastResult == lbool.Undef && e.karrDatalog != nil && e.karrDatalog.results != nil) {
		return answer, ErrPropositionalEngine
	}
	refs := make([]ast.ASTRef, 0, len(e.queryRelations))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && answer != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&answer))
			answer = ast.ASTRef{}
		}
	}()
	values := make([]ast.ASTHandle, 0, len(e.queryRelations))
	for _, decl := range e.queryRelations {
		ref, err := e.Answer(decl)
		if err != nil {
			return answer, err
		}
		refs = append(refs, ref)
		h, err := e.manager.RefGet(ref)
		if err != nil {
			return answer, err
		}
		values = append(values, h)
	}
	answer, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return answer, err
	}
	h, err := e.manager.MkAndArray(values)
	if !h.IsNull() {
		err = errors.Join(err, e.manager.SetRef(answer, h))
	}
	return answer, err
}
