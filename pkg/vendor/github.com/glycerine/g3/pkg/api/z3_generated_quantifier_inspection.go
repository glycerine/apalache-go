package api

// Source: generated api_commands.cpp, pattern and quantifier inspection.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedQuantifierInspection(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{273, "Z3_pattern_to_ast"}, {274, "Z3_get_pattern_num_terms"}, {275, "Z3_get_pattern"},
		{276, "Z3_get_index_value"}, {277, "Z3_is_quantifier_forall"}, {278, "Z3_get_quantifier_weight"},
		{279, "Z3_get_quantifier_num_patterns"}, {280, "Z3_get_quantifier_pattern_ast"},
		{281, "Z3_get_quantifier_num_no_patterns"}, {282, "Z3_get_quantifier_no_pattern_ast"},
		{283, "Z3_get_quantifier_num_bound"}, {284, "Z3_get_quantifier_bound_name"},
		{285, "Z3_get_quantifier_bound_sort"}, {286, "Z3_get_quantifier_body"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayQuantifierInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayQuantifierInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	var index uint32
	if id == 275 || id == 280 || id == 282 || id == 284 || id == 285 {
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
	}
	var h ast.ASTHandle
	switch id {
	case 273:
		h = value.handle
	case 274:
		_, err = owner.context.PatternTermCount(value.handle)
	case 275:
		h, err = owner.context.PatternTerm(value.handle, index)
	case 276:
		_, err = owner.context.VariableIndex(value.handle)
	case 277:
		_, err = owner.context.IsForall(value.handle)
	case 278:
		_, err = owner.context.QuantifierWeight(value.handle)
	case 279:
		_, err = owner.context.QuantifierPatternCount(value.handle)
	case 280:
		h, err = owner.context.QuantifierPattern(value.handle, index)
	case 281:
		_, err = owner.context.QuantifierNoPatternCount(value.handle)
	case 282:
		h, err = owner.context.QuantifierNoPattern(value.handle, index)
	case 283:
		_, err = owner.context.QuantifierBoundCount(value.handle)
	case 284:
		_, err = owner.context.QuantifierBoundName(value.handle, index)
	case 285:
		h, err = owner.context.QuantifierBoundSort(value.handle, index)
	case 286:
		h, err = owner.context.QuantifierBody(value.handle)
	}
	if err != nil {
		return err
	}
	if id == 273 || id == 275 || id == 280 || id == 282 || id == 285 || id == 286 {
		return r.storeBorrowedAST(owner, h)
	}
	return nil
}
