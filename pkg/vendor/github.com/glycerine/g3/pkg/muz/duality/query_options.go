package duality

import (
	"errors"

	graph "github.com/glycerine/g3/pkg/duality"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// QueryOptions are the solver options set by dl_interface::query. Recursion
// bounds belong to prepared clauses, rather than the disabled source override.
type QueryOptions struct {
	FullExpand, NoConj, FeasibleEdges, UseUnderapprox                bool
	StratifiedInlining, BatchExpand, EnableRestarts, Report, Profile bool
	ConjectureFile                                                   string
}

func DefaultQueryOptions() QueryOptions { return QueryOptions{FeasibleEdges: true} }

// ReadQueryOptions follows fixedpoint_params local-over-module precedence.
// Defaults come from fixedpoint_params.pyg, including use_underapprox=false,
// which differs from the standalone Duality constructor default.
func ReadQueryOptions(context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef) (out QueryOptions, result error) {
	if context == nil || globals == nil {
		return out, graph.ErrWrapperOwner
	}
	global, err := globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return out, err
	}
	defer func() { result = errors.Join(result, context.Del(&global)) }()
	out = DefaultQueryOptions()
	for _, setting := range []struct {
		name   string
		target *bool
	}{
		{"duality.full_expand", &out.FullExpand}, {"duality.no_conj", &out.NoConj},
		{"duality.feasible_edges", &out.FeasibleEdges}, {"duality.use_underapprox", &out.UseUnderapprox},
		{"duality.stratified_inlining", &out.StratifiedInlining}, {"duality.batch_expand", &out.BatchExpand},
		{"duality.enable_restarts", &out.EnableRestarts}, {"duality.profile", &out.Profile},
	} {
		if local == nil {
			*setting.target, err = context.GetBoolName(&global, &setting.name, *setting.target)
		} else {
			*setting.target, err = context.GetBoolFallbackName(local, &setting.name, &global, *setting.target)
		}
		if err != nil {
			return out, err
		}
	}
	name := "duality.conjecture_file"
	if local == nil {
		out.ConjectureFile, _, err = context.GetStringName(&global, &name, nil)
	} else {
		out.ConjectureFile, _, err = context.GetStringFallbackName(local, &name, &global, nil)
	}
	return out, err
}
func (o QueryOptions) apply(engine *graph.Engine) {
	for _, setting := range []struct {
		name  string
		value bool
	}{
		{"full_expand", o.FullExpand}, {"no_conj", o.NoConj}, {"feasible_edges", o.FeasibleEdges},
		{"use_underapprox", o.UseUnderapprox}, {"stratified_inlining", o.StratifiedInlining},
		{"batch_expand", o.BatchExpand}, {"enable_restarts", o.EnableRestarts}, {"report", o.Report},
	} {
		value := "0"
		if setting.value {
			value = "1"
		}
		engine.SetOption(setting.name, value)
	}
	// The source setter stores this option while returning false.
	engine.SetOption("conjecture_file", o.ConjectureFile)
}
func (e *QueryEngine) SetOptions(options QueryOptions) error {
	if err := e.open(); err != nil {
		return err
	}
	e.options = options
	return nil
}
