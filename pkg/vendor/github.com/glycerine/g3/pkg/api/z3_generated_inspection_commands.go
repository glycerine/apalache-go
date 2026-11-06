package api

// Source: generated api_commands.cpp, application and AST inspection.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedInspectionCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{203, "Z3_get_sort_name"}, {204, "Z3_get_sort_id"}, {205, "Z3_sort_to_ast"},
		{206, "Z3_is_eq_sort"}, {207, "Z3_get_sort_kind"}, {208, "Z3_get_bv_sort_size"},
		{209, "Z3_get_finite_domain_sort_size"}, {210, "Z3_get_array_sort_domain"}, {211, "Z3_get_array_sort_range"},
		{227, "Z3_func_decl_to_ast"}, {228, "Z3_is_eq_func_decl"}, {229, "Z3_get_func_decl_id"},
		{230, "Z3_get_decl_name"}, {231, "Z3_get_decl_kind"}, {232, "Z3_get_domain_size"},
		{233, "Z3_get_arity"}, {234, "Z3_get_domain"}, {235, "Z3_get_range"},
		{236, "Z3_get_decl_num_parameters"}, {237, "Z3_get_decl_parameter_kind"},
		{238, "Z3_get_decl_int_parameter"}, {239, "Z3_get_decl_double_parameter"},
		{240, "Z3_get_decl_symbol_parameter"}, {241, "Z3_get_decl_sort_parameter"},
		{242, "Z3_get_decl_ast_parameter"}, {243, "Z3_get_decl_func_decl_parameter"},
		{244, "Z3_get_decl_rational_parameter"},
		{245, "Z3_app_to_ast"}, {246, "Z3_get_app_decl"}, {247, "Z3_get_app_num_args"},
		{248, "Z3_get_app_arg"}, {249, "Z3_is_eq_ast"}, {250, "Z3_get_ast_id"}, {251, "Z3_get_ast_hash"},
		{252, "Z3_get_sort"}, {253, "Z3_is_well_sorted"}, {254, "Z3_get_bool_value"},
		{255, "Z3_get_ast_kind"}, {256, "Z3_is_app"}, {259, "Z3_to_app"}, {260, "Z3_to_func_decl"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayASTInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayASTInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	var result ast.ASTHandle
	switch id {
	case 203:
		_, err = owner.context.SortSymbol(value.handle)
	case 204:
		_, err = owner.context.SortID(value.handle)
	case 205:
		result, err = owner.context.SortToAST(value.handle)
	case 206:
		var other *replayASTOwner
		other, err = r.astObject(owner, 2)
		if err == nil {
			_, err = owner.context.SortEqual(value.handle, other.handle)
		}
	case 207:
		_, err = owner.context.SortKind(value.handle)
	case 208:
		_, err = owner.context.BitVectorSortSize(value.handle)
	case 209:
		if _, err = r.Uint64(2); err != nil {
			return err
		}
		var size uint64
		size, err = owner.context.FiniteDomainSortSize(value.handle)
		if err == nil {
			err = r.SetUint64(2, size)
		}
	case 210:
		result, err = owner.context.ArrayDomain(value.handle, 0)
	case 211:
		result, err = owner.context.ArrayRange(value.handle)
	case 227:
		result, err = owner.context.FunctionDeclarationToAST(value.handle)
	case 228:
		var other *replayASTOwner
		other, err = r.astObject(owner, 2)
		if err == nil {
			_, err = owner.context.FunctionEqual(value.handle, other.handle)
		}
	case 229:
		_, err = owner.context.FunctionDeclarationID(value.handle)
	case 230:
		_, err = owner.context.DeclarationSymbol(value.handle)
	case 231:
		_, err = owner.context.PublicDeclarationKind(value.handle)
	case 232:
		_, err = owner.context.DeclarationDomainSize(value.handle)
	case 233:
		_, err = owner.context.DeclarationArity(value.handle)
	case 234:
		var index uint32
		index, err = r.Uint(2)
		if err == nil {
			result, err = owner.context.DeclarationDomain(value.handle, index)
		}
	case 235:
		result, err = owner.context.DeclarationRange(value.handle)
	case 236:
		_, err = owner.context.DeclarationParameterCount(value.handle)
	case 237, 238, 239, 240, 241, 242, 243, 244:
		var index uint32
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
		switch id {
		case 237:
			_, err = owner.context.DeclarationParameterKind(value.handle, index)
		case 238:
			_, err = owner.context.DeclarationIntegerParameter(value.handle, index)
		case 239:
			_, err = owner.context.DeclarationDoubleParameter(value.handle, index)
		case 240:
			_, err = owner.context.DeclarationParameterSymbol(value.handle, index)
		case 241:
			result, err = owner.context.DeclarationSortParameter(value.handle, index)
		case 242:
			result, err = owner.context.DeclarationASTParameter(value.handle, index)
		case 243:
			result, err = owner.context.DeclarationFunctionParameter(value.handle, index)
		case 244:
			_, err = owner.context.DeclarationRationalParameter(value.handle, index)
		}
	case 245:
		result, err = owner.context.ApplicationToAST(value.handle)
	case 246:
		result, err = owner.context.ApplicationDeclaration(value.handle)
	case 247:
		_, err = owner.context.ApplicationArgumentCount(value.handle)
	case 248:
		var index uint32
		index, err = r.Uint(2)
		if err == nil {
			result, err = owner.context.ApplicationArgument(value.handle, index)
		}
	case 249:
		var other *replayASTOwner
		other, err = r.astObject(owner, 2)
		if err == nil {
			_, err = owner.context.ASTEqual(value.handle, other.handle)
		}
	case 250:
		_, err = owner.context.ASTID(value.handle)
	case 251:
		_, err = owner.context.ASTHash(value.handle)
	case 252:
		result, err = owner.context.SortOf(value.handle)
	case 253:
		_, err = owner.context.IsWellSorted(value.handle)
	case 254:
		_, err = owner.context.BooleanValue(value.handle)
	case 255:
		_, err = owner.context.APIKind(value.handle)
	case 256:
		_, err = owner.context.IsApplication(value.handle)
	case 259:
		result, err = owner.context.ToApplication(value.handle)
	case 260:
		result, err = owner.context.ToFunctionDeclaration(value.handle)
	}
	if err != nil {
		return err
	}
	switch id {
	case 205, 210, 211, 227, 234, 235, 241, 242, 243, 245, 246, 248, 252, 259, 260:
		return r.storeBorrowedAST(owner, result)
	}
	return nil
}
