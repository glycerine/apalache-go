package api

// Source: generated api_commands.cpp, tuple and datatype inspection.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedDatatypeInspection(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{212, "Z3_get_tuple_sort_mk_decl"}, {213, "Z3_get_tuple_sort_num_fields"}, {214, "Z3_get_tuple_sort_field_decl"},
		{215, "Z3_get_datatype_sort_num_constructors"}, {216, "Z3_get_datatype_sort_constructor"},
		{217, "Z3_get_datatype_sort_recognizer"}, {218, "Z3_get_datatype_sort_constructor_accessor"}, {219, "Z3_datatype_update_field"},
		{220, "Z3_get_relation_arity"}, {221, "Z3_get_relation_column"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayDatatypeInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayDatatypeInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	var index, field uint32
	if id == 214 || id == 216 || id == 217 || id == 218 || id == 221 {
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
	}
	if id == 218 {
		field, err = r.Uint(3)
		if err != nil {
			return err
		}
	}
	var result ast.ASTHandle
	switch id {
	case 212:
		result, err = owner.context.TupleConstructor(sort.handle)
	case 213:
		_, err = owner.context.TupleFieldCount(sort.handle)
	case 214:
		result, err = owner.context.TupleField(sort.handle, index)
	case 215:
		_, err = owner.context.DatatypeConstructorCount(sort.handle)
	case 216:
		result, err = owner.context.DatatypeConstructor(sort.handle, index)
	case 217:
		result, err = owner.context.DatatypeRecognizer(sort.handle, index)
	case 218:
		result, err = owner.context.DatatypeAccessor(sort.handle, index, field)
	case 219:
		var value, replacement *replayASTOwner
		value, err = r.astObject(owner, 2)
		if err != nil {
			return err
		}
		replacement, err = r.astObject(owner, 3)
		if err != nil {
			return err
		}
		result, err = owner.context.DatatypeUpdateField(sort.handle, value.handle, replacement.handle)
	case 220:
		_, err = owner.context.RelationArity(sort.handle)
	case 221:
		result, err = owner.context.RelationColumn(sort.handle, index)
	}
	if err != nil {
		return err
	}
	if id == 213 || id == 215 || id == 220 {
		return nil
	}
	if id == 219 {
		return r.storeAST(owner, result)
	}
	return r.storeBorrowedAST(owner, result)
}
