package api

// Source: generated api_commands.cpp, algebraic arithmetic and comparisons.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func registerZ3GeneratedAlgebraicCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{468, "Z3_algebraic_is_value"}, {469, "Z3_algebraic_is_pos"}, {470, "Z3_algebraic_is_neg"}, {471, "Z3_algebraic_is_zero"},
		{472, "Z3_algebraic_sign"}, {473, "Z3_algebraic_add"}, {474, "Z3_algebraic_sub"}, {475, "Z3_algebraic_mul"},
		{476, "Z3_algebraic_div"}, {477, "Z3_algebraic_root"}, {478, "Z3_algebraic_power"}, {479, "Z3_algebraic_lt"},
		{480, "Z3_algebraic_gt"}, {481, "Z3_algebraic_le"}, {482, "Z3_algebraic_ge"}, {483, "Z3_algebraic_eq"}, {484, "Z3_algebraic_neq"},
		{485, "Z3_algebraic_roots"}, {486, "Z3_algebraic_eval"}, {487, "Z3_polynomial_subresultants"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayAlgebraicCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayAlgebraicCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	first, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if id >= 485 {
		return replayAlgebraicPolynomial(r, owner, first, id)
	}
	var second ast.ASTHandle
	if id >= 473 && id <= 476 || id >= 479 && id <= 484 {
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		second = value.handle
	}
	var result ast.ASTHandle
	c := owner.context
	switch id {
	case 468:
		_, err = c.IsAlgebraicValue(first.handle)
	case 469:
		_, err = c.AlgebraicIsPositive(first.handle)
	case 470:
		_, err = c.AlgebraicIsNegative(first.handle)
	case 471:
		_, err = c.AlgebraicIsZero(first.handle)
	case 472:
		_, err = c.AlgebraicSign(first.handle)
	case 473:
		result, err = c.AlgebraicAdd(first.handle, second)
	case 474:
		result, err = c.AlgebraicSub(first.handle, second)
	case 475:
		result, err = c.AlgebraicMul(first.handle, second)
	case 476:
		result, err = c.AlgebraicDiv(first.handle, second)
	case 477, 478:
		n, e := r.Uint(2)
		if e != nil {
			return e
		}
		if id == 477 {
			result, err = c.AlgebraicRoot(first.handle, n)
		} else {
			result, err = c.AlgebraicPower(first.handle, n)
		}
	case 479:
		_, err = c.AlgebraicLT(first.handle, second)
	case 480:
		_, err = c.AlgebraicGT(first.handle, second)
	case 481:
		_, err = c.AlgebraicLE(first.handle, second)
	case 482:
		_, err = c.AlgebraicGE(first.handle, second)
	case 483:
		_, err = c.AlgebraicEqual(first.handle, second)
	case 484:
		_, err = c.AlgebraicNotEqual(first.handle, second)
	}
	if err != nil {
		return err
	}
	if id >= 473 && id <= 478 {
		return r.storeAST(owner, result)
	}
	return nil
}

func replayAlgebraicPolynomial(r *Z3Replayer, owner *replayContextOwner, first *replayASTOwner, id uint32) (err error) {
	if id == 485 || id == 486 {
		assignments, e := replayASTArray(r, owner, 2, 3)
		if e != nil {
			return e
		}
		if id == 486 {
			_, err = owner.context.AlgebraicEval(first.handle, assignments)
			return err
		}
		roots, e := owner.context.AlgebraicRoots(first.handle, assignments)
		if e != nil {
			return e
		}
		vector, e := owner.context.NewASTVectorValues(roots...)
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, vector)
	}
	second, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	variable, err := r.astObject(owner, 3)
	if err != nil {
		return err
	}
	result, err := owner.context.PolynomialSubresultants(first.handle, second.handle, variable.handle)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, result.Close()) }()
	values, err := result.Values()
	if err != nil {
		return err
	}
	vector, err := owner.context.NewASTVectorValues(values...)
	if err != nil {
		return err
	}
	return replayStoreVector(r, owner, vector)
}
