package smtcore

// Source: src/cmd_context/context_params.cpp:174-178.

import uparams "github.com/glycerine/g3/pkg/util/params"

// CollectSolverParamDescriptions adds the three context-level solver options
// appended to every concrete solver's own catalog by the native API.
func CollectSolverParamDescriptions(descriptions *uparams.ParamDescrs) error {
	if descriptions == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	entries := []struct {
		name, description, defaultValue string
	}{
		{"proof", "proof generation, it must be enabled when the Z3 context is created", "false"},
		{"model", "model generation for solvers, this parameter can be overwritten when creating a solver", "true"},
		{"unsat_core", "unsat-core generation for solvers, this parameter can be overwritten when creating a solver, not every solver in Z3 supports unsat core generation", "false"},
	}
	for i := range entries {
		entry := &entries[i]
		if err := descriptions.InsertName(entry.name, uparams.ParamBool, &entry.description, &entry.defaultValue, nil); err != nil {
			return err
		}
	}
	return nil
}
