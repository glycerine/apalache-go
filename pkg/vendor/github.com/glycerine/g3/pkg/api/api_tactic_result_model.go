package api

// Source: src/api/api_tactic.cpp:516-531.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// ConvertModel copies source and applies the result's retained model converter
// for subgoal index. The returned API model owns the converted model.
func (r *APITacticResult) ConvertModel(index uint32, source *APIModel) (result *APIModel, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if err := source.open(); err != nil {
		return nil, err
	}
	if source.owner != r.owner {
		return nil, ErrSMTContextOwner
	}
	// The source tests strictly greater here and never indexes the goal
	// array. Equality is defined when no converter needs a subgoal index.
	if uint64(index) > uint64(len(r.goals)) {
		return nil, ast.ErrASTDomain
	}
	derived := source.core.DerivedModel()
	if derived == nil {
		return nil, ast.ErrASTDomain
	}
	copied, err := derived.Copy()
	if err != nil {
		if copied != nil {
			err = errors.Join(err, copied.Close())
		}
		return nil, err
	}
	var ref tactic.ModelRef
	if err = ref.Set(copied.Core()); err != nil {
		return nil, errors.Join(err, copied.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, ref.Close()) }()
	if r.modelConverter != nil {
		if err = r.modelConverter.ApplyModel(&ref, index); err != nil {
			return nil, err
		}
	}
	core, err := ref.Get()
	if err != nil || core == nil {
		return nil, err
	}
	return newAPIModel(r.owner, core, false)
}
