package api

import "github.com/glycerine/g3/pkg/ast"

// Source: generated api_commands.cpp, AST print mode and formatting.

func registerZ3GeneratedPrintCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{328, "Z3_set_ast_print_mode"}, {329, "Z3_ast_to_string"}, {330, "Z3_pattern_to_string"},
		{331, "Z3_sort_to_string"}, {332, "Z3_func_decl_to_string"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayPrintCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayPrintCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 328 {
		mode, e := r.Uint(1)
		if e != nil {
			return e
		}
		owner.printMode = mode
		return nil
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if owner.printMode > uint32(APIASTPrintSMT2) {
		return ast.ErrASTDomain
	}
	if id == 330 {
		_, err = owner.context.PatternString(value.handle)
		return err
	}
	_, err = owner.context.ASTString(value.handle, APIASTPrintMode(owner.printMode))
	return err
}
