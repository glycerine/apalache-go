package api

// Source: generated api_commands.cpp, numeral inspection.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedNumeralInspection(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{257, "Z3_is_numeral_ast"}, {258, "Z3_is_algebraic_number"}, {261, "Z3_get_numeral_string"}, {262, "Z3_get_numeral_decimal_string"},
		{263, "Z3_get_numerator"}, {264, "Z3_get_denominator"}, {265, "Z3_get_numeral_small"},
		{266, "Z3_get_numeral_int"}, {267, "Z3_get_numeral_uint"}, {268, "Z3_get_numeral_uint64"},
		{269, "Z3_get_numeral_int64"}, {270, "Z3_get_numeral_rational_int64"},
		{271, "Z3_get_algebraic_number_lower"}, {272, "Z3_get_algebraic_number_upper"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayNumeralInspection(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayNumeralInspection(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	c := owner.context
	var result ast.ASTHandle
	switch id {
	case 257:
		_, err = c.IsNumeral(value.handle)
	case 258:
		_, err = c.IsIrrationalAlgebraicNumber(value.handle)
	case 261:
		_, err = c.NumeralString(value.handle)
	case 262:
		var precision uint32
		precision, err = r.Uint(2)
		if err == nil {
			_, err = c.NumeralDecimalString(value.handle, precision)
		}
	case 263:
		result, err = c.Numerator(value.handle)
	case 264:
		result, err = c.Denominator(value.handle)
	case 265, 270:
		if _, err = r.Int64(2); err != nil {
			return err
		}
		if _, err = r.Int64(3); err != nil {
			return err
		}
		var a, b int64
		var matched bool
		if id == 265 {
			a, b, matched, err = c.NumeralSmall(value.handle)
		} else {
			a, b, matched, err = c.NumeralRationalInt64(value.handle)
		}
		if err == nil && matched {
			if err = r.SetInt64(2, a); err == nil {
				err = r.SetInt64(3, b)
			}
		}
	case 266, 269:
		if _, err = r.Int64(2); err != nil {
			return err
		}
		var n int64
		var matched bool
		if id == 266 {
			var v int32
			v, matched, err = c.NumeralInt32(value.handle)
			n = int64(v)
		} else {
			n, matched, err = c.NumeralInt64(value.handle)
		}
		if err == nil && matched {
			err = r.SetInt64(2, n)
		}
	case 267, 268:
		if _, err = r.Uint64(2); err != nil {
			return err
		}
		var n uint64
		var matched bool
		if id == 267 {
			var v uint32
			v, matched, err = c.NumeralUint32(value.handle)
			n = uint64(v)
		} else {
			n, matched, err = c.NumeralUint64(value.handle)
		}
		if err == nil && matched {
			err = r.SetUint64(2, n)
		}
	case 271, 272:
		var precision uint32
		precision, err = r.Uint(2)
		if err != nil {
			return err
		}
		if id == 271 {
			result, err = c.AlgebraicNumberLower(value.handle, precision)
		} else {
			result, err = c.AlgebraicNumberUpper(value.handle, precision)
		}
	}
	if err != nil {
		return err
	}
	if id == 263 || id == 264 || id == 271 || id == 272 {
		return r.storeAST(owner, result)
	}
	return nil
}
