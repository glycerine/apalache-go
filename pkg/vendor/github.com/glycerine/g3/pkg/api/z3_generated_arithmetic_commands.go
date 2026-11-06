package api

// Source: generated api_commands.cpp, arithmetic and numeral commands.

import "github.com/glycerine/g3/pkg/ast"

func replayNumeral(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	text, err := r.String(1)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.Numeral(text, sort.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayReal(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	a, err := r.Int(1)
	if err != nil {
		return err
	}
	b, err := r.Int(2)
	if err != nil {
		return err
	}
	h, err := owner.context.Real(int64(a), int64(b))
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayNumber(r *Z3Replayer, signed, narrow bool) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	sort, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	if signed {
		v, e := r.Int64(1)
		if e != nil {
			return e
		}
		if narrow {
			v = int64(int32(v))
		}
		h, err = owner.context.IntNumeral(v, sort.handle)
	} else {
		v, e := r.Uint64(1)
		if e != nil {
			return e
		}
		if narrow {
			v = uint64(uint32(v))
		}
		h, err = owner.context.UintNumeral(v, sort.handle)
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayIntNumeral(r *Z3Replayer) error    { return replayNumber(r, true, true) }
func replayUIntNumeral(r *Z3Replayer) error   { return replayNumber(r, false, true) }
func replayInt64Numeral(r *Z3Replayer) error  { return replayNumber(r, true, false) }
func replayUInt64Numeral(r *Z3Replayer) error { return replayNumber(r, false, false) }

func replayArithmeticArray(r *Z3Replayer, op uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	args, err := replayASTArray(r, owner, 1, 2)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	switch op {
	case 0:
		h, err = owner.context.ArithmeticAdd(args...)
	case 1:
		h, err = owner.context.ArithmeticMul(args...)
	case 2:
		h, err = owner.context.ArithmeticSub(args...)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayAdd(r *Z3Replayer) error { return replayArithmeticArray(r, 0) }
func replayMul(r *Z3Replayer) error { return replayArithmeticArray(r, 1) }
func replaySub(r *Z3Replayer) error { return replayArithmeticArray(r, 2) }
func replayArithmeticUnary(r *Z3Replayer, op uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	a, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	switch op {
	case 0:
		h, err = owner.context.ArithmeticNeg(a.handle)
	case 1:
		h, err = owner.context.IntegerToReal(a.handle)
	case 2:
		h, err = owner.context.RealToInteger(a.handle)
	case 3:
		h, err = owner.context.IsIntegerValue(a.handle)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayNeg(r *Z3Replayer) error       { return replayArithmeticUnary(r, 0) }
func replayIntToReal(r *Z3Replayer) error { return replayArithmeticUnary(r, 1) }
func replayRealToInt(r *Z3Replayer) error { return replayArithmeticUnary(r, 2) }
func replayIsInt(r *Z3Replayer) error     { return replayArithmeticUnary(r, 3) }
func replayArithmeticBinary(r *Z3Replayer, op uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	a, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	b, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	var h ast.ASTHandle
	switch op {
	case 0:
		h, err = owner.context.ArithmeticDiv(a.handle, b.handle)
	case 1:
		h, err = owner.context.ArithmeticMod(a.handle, b.handle)
	case 2:
		h, err = owner.context.ArithmeticRem(a.handle, b.handle)
	case 3:
		h, err = owner.context.ArithmeticPower(a.handle, b.handle)
	case 4:
		h, err = owner.context.ArithmeticLT(a.handle, b.handle)
	case 5:
		h, err = owner.context.ArithmeticLE(a.handle, b.handle)
	case 6:
		h, err = owner.context.ArithmeticGT(a.handle, b.handle)
	case 7:
		h, err = owner.context.ArithmeticGE(a.handle, b.handle)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayDiv(r *Z3Replayer) error   { return replayArithmeticBinary(r, 0) }
func replayMod(r *Z3Replayer) error   { return replayArithmeticBinary(r, 1) }
func replayRem(r *Z3Replayer) error   { return replayArithmeticBinary(r, 2) }
func replayPower(r *Z3Replayer) error { return replayArithmeticBinary(r, 3) }
func replayLT(r *Z3Replayer) error    { return replayArithmeticBinary(r, 4) }
func replayLE(r *Z3Replayer) error    { return replayArithmeticBinary(r, 5) }
func replayGT(r *Z3Replayer) error    { return replayArithmeticBinary(r, 6) }
func replayGE(r *Z3Replayer) error    { return replayArithmeticBinary(r, 7) }
