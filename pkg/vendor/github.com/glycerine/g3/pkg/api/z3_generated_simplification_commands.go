package api

// Source: generated api_commands.cpp, Z3_simplify and parameter descriptions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func registerZ3GeneratedSimplificationCommands(r *Z3Replayer) error {
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{287, "Z3_simplify", func(r *Z3Replayer) error { return replaySimplify(r, false) }},
		{288, "Z3_simplify_ex", func(r *Z3Replayer) error { return replaySimplify(r, true) }},
		{289, "Z3_simplify_get_help", replaySimplifyHelp},
		{290, "Z3_simplify_get_param_descrs", replaySimplifyDescriptions},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replaySimplify(r *Z3Replayer, configured bool) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	expression, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	var result ast.ASTRef
	if configured {
		parameters, e := replayParametersAt(r, owner, 2)
		if e != nil {
			return e
		}
		result, err = owner.context.SimplifyWithParameters(expression.handle, parameters.value)
	} else {
		result, err = owner.context.Simplify(expression.handle)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owner.results.manager.DelRef(&result)) }()
	value, err := owner.results.manager.RefGet(result)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
func replaySimplifyHelp(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	_, err = owner.context.SimplifyHelp()
	return err
}
func replaySimplifyDescriptions(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := owner.context.SimplifyParameterDescriptions()
	if err != nil {
		return err
	}
	return storeReplayDescriptions(r, owner, value)
}
