package api

// Source: generated api_commands.cpp and src/api/api_array.cpp.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedSetCommands(r *Z3Replayer) error {
	names := []string{"Z3_mk_set_sort", "Z3_mk_empty_set", "Z3_mk_full_set", "Z3_mk_set_add", "Z3_mk_set_del", "Z3_mk_set_union", "Z3_mk_set_intersect", "Z3_mk_set_difference", "Z3_mk_set_complement", "Z3_mk_set_member", "Z3_mk_set_subset", "Z3_mk_array_ext"}
	for i, name := range names {
		id := uint32(136 + i)
		if err := r.RegisterCommand(id, name, func(r *Z3Replayer) error { return replaySetCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replaySetCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	c := owner.context
	var result ast.ASTHandle
	if id == 141 || id == 142 {
		sets, e := replayASTArray(r, owner, 1, 2)
		if e != nil {
			return e
		}
		if id == 141 {
			result, err = c.SetUnion(sets...)
		} else {
			result, err = c.SetIntersection(sets...)
		}
	} else {
		first, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		switch id {
		case 136:
			result, err = c.SetSort(first.handle)
		case 137:
			result, err = c.EmptySet(first.handle)
		case 138:
			result, err = c.FullSet(first.handle)
		case 144:
			result, err = c.SetComplement(first.handle)
		default:
			second, e := r.astObject(owner, 2)
			if e != nil {
				return e
			}
			switch id {
			case 139:
				result, err = c.SetAdd(first.handle, second.handle)
			case 140:
				result, err = c.SetDelete(first.handle, second.handle)
			case 143:
				result, err = c.SetDifference(first.handle, second.handle)
			case 145:
				result, err = c.SetMember(first.handle, second.handle)
			case 146:
				result, err = c.SetSubset(first.handle, second.handle)
			case 147:
				result, err = c.ArrayExt(first.handle, second.handle)
			}
		}
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, result)
}
