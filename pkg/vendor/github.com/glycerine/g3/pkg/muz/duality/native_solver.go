package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewNativeSolver creates the owning proof-producing backend used by the
// fixedpoint interface. The factory, context and parameter service are borrowed.
// Arithmetic equality normalization is enabled before assertions so the native
// certificates match the supported Craig interpolation rules.
// Source: duality_wrapper.cpp solver constructor; dl_interface::query proof scope.
func NewNativeSolver(context *graph.Context, params *uparams.ParamsContext, factory solver.Factory, mbqi bool) (out solver.Solver, result error) {
	if context == nil || params == nil || factory == nil {
		return nil, graph.ErrWrapperOwner
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return nil, err
	}
	attempt := &nativeSolverConstruction{params: params, settings: params.EmptyValue(), proof: scope}
	if numbers := m.NumeralService(); numbers != nil {
		attempt.runtime = numbers.RuntimeOwner()
	}
	defer func() { out, result = attempt.finish(result) }()
	p := &attempt.settings
	for _, setting := range []struct {
		name  string
		value bool
	}{
		{"proof", true}, {"model", true}, {"unsat_core", true}, {"mbqi", mbqi}, {"eq2ineq", true},
	} {
		if err := params.SetBoolName(p, &setting.name, setting.value); err != nil {
			return nil, err
		}
	}
	if mbqi {
		name := "array.extensional"
		if err := params.SetBoolName(p, &name, true); err != nil {
			return nil, err
		}
	}
	name := "mbqi.id"
	id := "itp"
	if err := params.SetStringName(p, &name, &id); err != nil {
		return nil, err
	}
	name = "mbqi.max_iterations"
	if err := params.SetUIntName(p, &name, 1); err != nil {
		return nil, err
	}
	attempt.native, err = factory.New(m, p, true, true, true, "")
	if err != nil {
		return nil, err
	}
	if err := attempt.native.UpdateParams(p); err != nil {
		return nil, err
	}
	return attempt.native, nil
}
