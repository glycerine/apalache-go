package api

// Source: generated src/api/api_commands.cpp, basic AST reference, sort and
// Boolean construction commands.

// RegisterZ3GeneratedASTCommands installs the implemented basic AST handlers.
func RegisterZ3GeneratedASTCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{9, "Z3_inc_ref", replayIncAST}, {10, "Z3_dec_ref", replayDecAST},
		{32, "Z3_mk_bool_sort", replayBoolSort}, {33, "Z3_mk_int_sort", replayIntSort}, {34, "Z3_mk_real_sort", replayRealSort}, {35, "Z3_mk_bv_sort", replayBVSort},
		{31, "Z3_mk_uninterpreted_sort", replayUninterpretedSort}, {49, "Z3_mk_func_decl", replayFunction}, {50, "Z3_mk_app", replayApplication},
		{36, "Z3_mk_finite_domain_sort", replayFiniteDomainSort}, {37, "Z3_mk_array_sort", replayArraySort}, {38, "Z3_mk_array_sort_n", replayArraySortN},
		{39, "Z3_mk_tuple_sort", replayTupleSort}, {40, "Z3_mk_enumeration_sort", replayEnumerationSort}, {41, "Z3_mk_list_sort", replayListSort},
		{51, "Z3_mk_const", replayConstant}, {52, "Z3_mk_fresh_func_decl", replayFreshFunction}, {53, "Z3_mk_fresh_const", replayFreshConstant},
		{54, "Z3_mk_true", replayTrue}, {55, "Z3_mk_false", replayFalse}, {56, "Z3_mk_eq", replayEqual}, {58, "Z3_mk_not", replayNot},
		{57, "Z3_mk_distinct", replayDistinct}, {59, "Z3_mk_ite", replayIte}, {60, "Z3_mk_iff", replayIff},
		{61, "Z3_mk_implies", replayImplies}, {62, "Z3_mk_xor", replayXor}, {63, "Z3_mk_and", replayAnd}, {64, "Z3_mk_or", replayOr},
		{65, "Z3_mk_add", replayAdd}, {66, "Z3_mk_mul", replayMul}, {67, "Z3_mk_sub", replaySub}, {68, "Z3_mk_unary_minus", replayNeg},
		{69, "Z3_mk_div", replayDiv}, {70, "Z3_mk_mod", replayMod}, {71, "Z3_mk_rem", replayRem}, {72, "Z3_mk_power", replayPower},
		{73, "Z3_mk_lt", replayLT}, {74, "Z3_mk_le", replayLE}, {75, "Z3_mk_gt", replayGT}, {76, "Z3_mk_ge", replayGE},
		{77, "Z3_mk_int2real", replayIntToReal}, {78, "Z3_mk_real2int", replayRealToInt}, {79, "Z3_mk_is_int", replayIsInt},
		{148, "Z3_mk_numeral", replayNumeral}, {149, "Z3_mk_real", replayReal}, {150, "Z3_mk_int", replayIntNumeral},
		{128, "Z3_mk_select", replaySelect}, {129, "Z3_mk_select_n", replaySelectN}, {130, "Z3_mk_store", replayStore},
		{131, "Z3_mk_store_n", replayStoreN}, {132, "Z3_mk_const_array", replayConstArray}, {133, "Z3_mk_map", replayArrayMap}, {134, "Z3_mk_array_default", replayArrayDefault}, {135, "Z3_mk_as_array", replayAsArray},
		{151, "Z3_mk_unsigned_int", replayUIntNumeral}, {152, "Z3_mk_int64", replayInt64Numeral}, {153, "Z3_mk_unsigned_int64", replayUInt64Numeral}, {154, "Z3_mk_bv_numeral", replayBitVectorBoolNumeral},
		{563, "Z3_mk_interpolant", replayMkInterpolant}, {564, "Z3_mk_interpolation_context", replayMkInterpolationContext},
		{565, "Z3_get_interpolant", replayGetInterpolant},
		{566, "Z3_compute_interpolant", replayComputeInterpolant},
		{567, "Z3_interpolation_profile", replayInterpolationProfile},
		{568, "Z3_read_interpolation_problem", replayReadInterpolationProblem},
		{569, "Z3_check_interpolant", replayCheckInterpolant}, {570, "Z3_write_interpolation_problem", replayWriteInterpolationProblem},
		{334, "Z3_benchmark_to_smtlib_string", replayBenchmarkString}, {335, "Z3_parse_smtlib2_string", replayParseSMT2String}, {336, "Z3_parse_smtlib2_file", replayParseSMT2File},
		{337, "Z3_eval_smtlib2_string", replayEvaluateSMT2}, {338, "Z3_get_parser_error", replayGetParserError},
		{339, "Z3_get_error_code", replayGetErrorCode}, {340, "Z3_set_error", replaySetErrorCode}, {341, "Z3_get_error_msg", replayErrorMessage},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	if err := registerZ3GeneratedBitVectorCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedQuantifierCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedQuantifierInspection(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedSimplificationCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedSubstitutionCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedInspectionCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedSequenceCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedPseudoBooleanCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedDatatypeInspection(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedNumeralInspection(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedPrintCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedGoalCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedVectorCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedTacticCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedMapCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedAlgebraicCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedFixedpointCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedFloatingPointCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedQECommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedSetCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedRCFCommands(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedDatatypeConstructorCommands(r); err != nil {
		return err
	}
	return registerZ3GeneratedOptimizeCommands(r)
}
func replayIncAST(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	entry, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if entry.references == ^uint32(0) {
		return ErrZ3ReplayState
	}
	if err = owner.results.manager.IncRef(entry.handle); err != nil {
		return err
	}
	entry.references++
	return nil
}
func replayDecAST(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	entry, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if entry.references == 0 {
		return ErrZ3ReplayReference
	}
	consumed, err := owner.results.manager.DecRefConsumed(entry.handle)
	if consumed {
		entry.references--
	}
	if err != nil {
		return err
	}
	if !consumed {
		return ErrZ3ReplayState
	}
	return nil
}
func replayBoolSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.BoolSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayIntSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.IntegerSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayRealSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.RealSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBVSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	bits, err := r.Uint(1)
	if err != nil {
		return err
	}
	h, err := owner.context.BitVectorSort(bits)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBool(r *Z3Replayer, value bool) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.Bool(value)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayTrue(r *Z3Replayer) error  { return replayBool(r, true) }
func replayFalse(r *Z3Replayer) error { return replayBool(r, false) }
func replayNot(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	arg, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	h, err := owner.context.Not(arg.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayEqual(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	left, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	right, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.Equal(left.handle, right.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
