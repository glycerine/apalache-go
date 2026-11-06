package api

// Source: generated api_commands.cpp, term update and substitution commands.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func registerZ3GeneratedSubstitutionCommands(r *Z3Replayer) error {
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{291, "Z3_update_term", func(r *Z3Replayer) error { return replayTermRewrite(r, 291) }},
		{292, "Z3_substitute", func(r *Z3Replayer) error { return replayTermRewrite(r, 292) }},
		{293, "Z3_substitute_vars", func(r *Z3Replayer) error { return replayTermRewrite(r, 293) }},
		{294, "Z3_translate", replayTranslateAST},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}

func replayTermRewrite(r *Z3Replayer, command uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	term, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	args, err := replayASTArray(r, owner, 2, 3)
	if err != nil {
		return err
	}
	var result ast.ASTRef
	switch command {
	case 291:
		result, err = owner.context.UpdateTerm(term.handle, args...)
	case 292:
		var replacements []ast.ASTHandle
		replacements, err = replayASTArray(r, owner, 2, 4)
		if err != nil {
			return err
		}
		result, err = owner.context.Substitute(term.handle, args, replacements)
	case 293:
		result, err = owner.context.SubstituteVariables(term.handle, args)
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

func replayTranslateAST(r *Z3Replayer) (err error) {
	source, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	term, err := r.astObject(source, 1)
	if err != nil {
		return err
	}
	target, err := replayContext(r, 2)
	if err != nil {
		return err
	}
	result, err := source.context.TranslateAST(term.handle, target.context)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.results.manager.DelRef(&result)) }()
	value, err := target.results.manager.RefGet(result)
	if err != nil {
		return err
	}
	return r.storeAST(target, value)
}
