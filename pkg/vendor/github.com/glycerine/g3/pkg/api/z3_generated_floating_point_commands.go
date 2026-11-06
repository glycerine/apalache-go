package api

// Source: generated api_commands.cpp and src/api/api_fpa.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func registerZ3GeneratedFloatingPointCommands(r *Z3Replayer) error {
	names := []string{
		"rounding_mode_sort", "round_nearest_ties_to_even", "rne", "round_nearest_ties_to_away", "rna",
		"round_toward_positive", "rtp", "round_toward_negative", "rtn", "round_toward_zero", "rtz",
		"sort", "sort_half", "sort_16", "sort_single", "sort_32", "sort_double", "sort_64", "sort_quadruple", "sort_128",
		"nan", "inf", "zero", "fp", "numeral_float", "numeral_double", "numeral_int", "numeral_int_uint", "numeral_int64_uint64",
	}
	for i, name := range names {
		id := uint32(571 + i)
		if err := r.RegisterCommand(id, "Z3_mk_fpa_"+name, func(r *Z3Replayer) error { return replayFloatingPointConstructor(r, id) }); err != nil {
			return err
		}
	}
	ops := []string{"abs", "neg", "add", "sub", "mul", "div", "fma", "sqrt", "rem", "round_to_integral", "min", "max", "leq", "lt", "geq", "gt", "eq", "is_normal", "is_subnormal", "is_zero", "is_infinite", "is_nan", "is_negative", "is_positive"}
	for i, name := range ops {
		id := uint32(600 + i)
		if err := r.RegisterCommand(id, "Z3_mk_fpa_"+name, func(r *Z3Replayer) error { return replayFloatingPointOperation(r, id) }); err != nil {
			return err
		}
	}
	conversions := []string{"to_fp_bv", "to_fp_float", "to_fp_real", "to_fp_signed", "to_fp_unsigned", "to_ubv", "to_sbv", "to_real"}
	for i, name := range conversions {
		id := uint32(624 + i)
		if err := r.RegisterCommand(id, "Z3_mk_fpa_"+name, func(r *Z3Replayer) error { return replayFloatingPointConversion(r, id) }); err != nil {
			return err
		}
	}
	if err := r.RegisterCommand(649, "Z3_mk_fpa_to_ieee_bv", func(r *Z3Replayer) error { return replayFloatingPointConversion(r, 649) }); err != nil {
		return err
	}
	if err := r.RegisterCommand(650, "Z3_mk_fpa_to_fp_int_real", replayFloatingPointFromExponentSignificand); err != nil {
		return err
	}
	inspections := []string{"get_ebits", "get_sbits", "is_numeral_nan", "is_numeral_inf", "is_numeral_zero", "is_numeral_normal", "is_numeral_subnormal", "is_numeral_positive", "is_numeral_negative", "get_numeral_sign_bv", "get_numeral_significand_bv", "get_numeral_sign", "get_numeral_significand_string", "get_numeral_significand_uint64", "get_numeral_exponent_string", "get_numeral_exponent_int64", "get_numeral_exponent_bv"}
	for i, name := range inspections {
		id := uint32(632 + i)
		if err := r.RegisterCommand(id, "Z3_fpa_"+name, func(r *Z3Replayer) error { return replayFloatingPointInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayFloatingPointFromExponentSignificand(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 4)
	for i := range args {
		value, e := r.astObject(owner, uint32(i+1))
		if e != nil {
			return e
		}
		args[i] = value.handle
	}
	value, err := owner.context.FloatingPointFromExponentSignificand(args[0], args[1], args[2], args[3])
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}

func replayFloatingPointInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	c := owner.context
	switch id {
	case 632:
		_, err = c.FloatingPointExponentBits(value.handle)
	case 633:
		_, err = c.FloatingPointSignificandBits(value.handle)
	case 634, 635, 636, 637, 638, 639, 640:
		kinds := [...]ast.FPAClassificationKind{ast.FPAIsNaN, ast.FPAIsInfinite, ast.FPAIsZero, ast.FPAIsNormal, ast.FPAIsSubnormal, ast.FPAIsPositive, ast.FPAIsNegative}
		_, err = c.FloatingPointNumeralClass(value.handle, kinds[id-634])
	case 641, 642, 648:
		var result ast.ASTHandle
		if id == 641 {
			result, err = c.FloatingPointNumeralSignBit(value.handle)
		} else if id == 642 {
			result, err = c.FloatingPointNumeralSignificandBitVector(value.handle)
		} else {
			biased, e := r.Bool(2)
			if e != nil {
				return e
			}
			result, err = c.FloatingPointNumeralExponentBitVector(value.handle, biased)
		}
		if err != nil {
			return err
		}
		return r.storeAST(owner, result)
	case 643:
		if _, err = r.Int64(2); err != nil {
			return err
		}
		var sign int
		sign, err = c.FloatingPointNumeralSign(value.handle)
		if err == nil {
			err = r.SetInt64(2, int64(sign))
		}
	case 644:
		_, err = c.FloatingPointNumeralSignificandString(value.handle)
	case 645:
		if _, err = r.Uint64(2); err != nil {
			return err
		}
		var n uint64
		n, err = c.FloatingPointNumeralSignificandUint64(value.handle)
		if err == nil {
			err = r.SetUint64(2, n)
		}
	case 646:
		var biased bool
		biased, err = r.Bool(2)
		if err == nil {
			_, err = c.FloatingPointNumeralExponentString(value.handle, biased)
		}
	case 647:
		if _, err = r.Int64(2); err != nil {
			return err
		}
		biased, e := r.Bool(3)
		if e != nil {
			return e
		}
		var n int64
		n, err = c.FloatingPointNumeralExponentInt64(value.handle, biased)
		if err == nil {
			err = r.SetInt64(2, n)
		}
	}
	return err
}

func replayFloatingPointConversion(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	m := owner.results.manager
	family, err := m.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	var args []ast.ASTHandle
	count := uint32(2)
	if id == 624 || id == 631 || id == 649 {
		count = 1
	}
	for i := uint32(1); i <= count; i++ {
		v, e := r.astObject(owner, i)
		if e != nil {
			return e
		}
		args = append(args, v.handle)
	}
	var params []ast.ASTParameter
	defer func() {
		for i := range params {
			err = errors.Join(err, m.DelParameter(&params[i]))
		}
	}()
	addParam := func(n uint32) error {
		p, e := m.NewParameterUint(n)
		if e == nil {
			params = append(params, p)
		}
		return e
	}
	kind := ast.FPAOpToFP
	if id >= 624 && id <= 628 {
		sort, e := r.astObject(owner, count+1)
		if e != nil {
			return e
		}
		ebits, sbits, ok, e := m.FloatingPointSortShape(sort.handle)
		if e != nil {
			return e
		}
		if !ok {
			return ast.ErrASTDomain
		}
		sourceSort, e := m.GetSort(args[count-1])
		if e != nil {
			return e
		}
		sourceKind, e := owner.context.SortKind(sourceSort)
		if e != nil {
			return e
		}
		want := APISortBitVector
		if id == 625 {
			want = APISortFloatingPoint
		}
		if id == 626 {
			want = APISortReal
		}
		if sourceKind != want {
			return ast.ErrASTDomain
		}
		if e = addParam(ebits); e != nil {
			return e
		}
		if e = addParam(sbits); e != nil {
			return e
		}
		if id == 628 {
			kind = ast.FPAOpToFPUnsigned
		}
	} else if id == 629 || id == 630 {
		width, e := r.Uint(3)
		if e != nil {
			return e
		}
		if e = addParam(width); e != nil {
			return e
		}
		if id == 629 {
			kind = ast.FPAOpToUBV
		} else {
			kind = ast.FPAOpToSBV
		}
	} else if id == 631 {
		kind = ast.FPAOpToReal
	} else {
		kind = ast.FPAOpToIEEEBV
	}
	value, err := m.MkFamilyApp(family, kind, params, args)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}

func replayFloatingPointOperation(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	type operation struct {
		kind  int32
		arity uint32
	}
	ops := [...]operation{
		{ast.FPAOpAbs, 1}, {ast.FPAOpNeg, 1}, {ast.FPAOpAdd, 3}, {ast.FPAOpSub, 3},
		{ast.FPAOpMul, 3}, {ast.FPAOpDiv, 3}, {ast.FPAOpFMA, 4}, {ast.FPAOpSqrt, 2},
		{ast.FPAOpRem, 2}, {ast.FPAOpRoundToIntegral, 2}, {ast.FPAOpMin, 2}, {ast.FPAOpMax, 2},
		{ast.FPAOpLe, 2}, {ast.FPAOpLt, 2}, {ast.FPAOpGe, 2}, {ast.FPAOpGt, 2}, {ast.FPAOpEq, 2},
		{int32(ast.FPAIsNormal), 1}, {int32(ast.FPAIsSubnormal), 1}, {int32(ast.FPAIsZero), 1},
		{int32(ast.FPAIsInfinite), 1}, {int32(ast.FPAIsNaN), 1}, {int32(ast.FPAIsNegative), 1}, {int32(ast.FPAIsPositive), 1},
	}
	op := ops[id-600]
	args := make([]ast.ASTHandle, op.arity)
	for i := range args {
		value, e := r.astObject(owner, uint32(i+1))
		if e != nil {
			return e
		}
		args[i] = value.handle
	}
	manager := owner.results.manager
	family, err := manager.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	value, err := manager.MkFamilyApp(family, op.kind, nil, args)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}

func replayFloatingPointConstructor(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	m := owner.results.manager
	family, err := m.RegisterFloatingPointPlugin()
	if err != nil {
		return err
	}
	var value ast.ASTHandle
	switch {
	case id == 571:
		value, err = m.RoundingModeSort()
	case id >= 572 && id <= 581:
		modes := []int32{ast.FPAOpRMNearestEven, ast.FPAOpRMNearestAway, ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardNegative, ast.FPAOpRMTowardZero}
		value, err = m.MkFamilyApp(family, modes[(id-572)/2], nil, nil)
	case id >= 582 && id <= 590:
		var ebits, sbits uint32
		if id == 582 {
			ebits, err = r.Uint(1)
			if err != nil {
				return err
			}
			sbits, err = r.Uint(2)
			if err != nil {
				return err
			}
		} else {
			shapes := [][2]uint32{{5, 11}, {8, 24}, {11, 53}, {15, 113}}
			shape := shapes[(id-583)/2]
			ebits, sbits = shape[0], shape[1]
		}
		value, err = m.FloatingPointSort(ebits, sbits)
	case id >= 591 && id <= 593:
		sort, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if id == 591 {
			value, err = m.MkFPANaNOfSort(sort.handle)
		} else {
			negative, e := r.Bool(2)
			if e != nil {
				return e
			}
			if id == 592 {
				value, err = m.MkFPAInfinityOfSort(sort.handle, negative)
			} else {
				value, err = m.MkFPAZeroOfSort(sort.handle, negative)
			}
		}
	case id == 594:
		args := make([]ast.ASTHandle, 3)
		for i := range args {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			args[i] = v.handle
		}
		value, err = m.MkFamilyApp(family, ast.FPAOpFP, nil, args)
	case id >= 595 && id <= 597:
		sort, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		switch id {
		case 595:
			v, e := r.Float(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralFloat32(v, sort.handle)
		case 596:
			v, e := r.Double(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralFloat64(v, sort.handle)
		case 597:
			v, e := r.Int(1)
			if e != nil {
				return e
			}
			value, err = m.MkFPANumeralInt32(v, sort.handle)
		}
	case id == 598 || id == 599:
		sign, e := r.Bool(1)
		if e != nil {
			return e
		}
		var exponent int64
		var significand uint64
		if id == 598 {
			v, e := r.Int(2)
			if e != nil {
				return e
			}
			exponent = int64(v)
			n, e := r.Uint(3)
			if e != nil {
				return e
			}
			significand = uint64(n)
		} else {
			exponent, e = r.Int64(2)
			if e != nil {
				return e
			}
			significand, e = r.Uint64(3)
			if e != nil {
				return e
			}
		}
		sort, e := r.astObject(owner, 4)
		if e != nil {
			return e
		}
		value, err = m.MkFPANumeralComponents(sign, exponent, significand, sort.handle)
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
