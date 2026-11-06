package sls

// Source: src/tactic/sls/sls_evaluator.h bitwise and comparison cases.
// Christoph Winter, 2012-02-29.

import "github.com/glycerine/g3/pkg/util/numeral"

type bvBitwiseKind uint8

const (
	bvAnd bvBitwiseKind = iota
	bvOr
	bvXor
	bvNand
	bvNor
)

type bvComparisonKind uint8

const (
	bvLess bvComparisonKind = iota
	bvLessEqual
	bvGreater
	bvGreaterEqual
)

func (e *bvEvaluatorSlice) bitwise(args []*numeral.Int, width uint32, kind bvBitwiseKind, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if len(args) < 2 || args[0] == nil || result == nil {
		return errEvaluatorArity
	}
	if (kind == bvNand || kind == bvNor) && width == 0 {
		return errEvaluatorWidth
	}
	if err = e.manager.Copy(result, args[0]); err != nil {
		return err
	}
	var temporary numeral.Int
	if kind == bvNand || kind == bvNor {
		defer sourceDeleteIntegers(e.manager, &err, &temporary)
	}
	for _, arg := range args[1:] {
		if arg == nil {
			return errEvaluatorArity
		}
		switch kind {
		case bvAnd:
			err = e.manager.BitwiseAnd(result, arg, result)
		case bvOr:
			err = e.manager.BitwiseOr(result, arg, result)
		case bvXor:
			err = e.manager.BitwiseXor(result, arg, result)
		case bvNand:
			if err = e.manager.BitwiseAnd(result, arg, &temporary); err == nil {
				err = e.manager.BitwiseNot(width, &temporary, result)
			}
		case bvNor:
			if err = e.manager.BitwiseOr(result, arg, &temporary); err == nil {
				err = e.manager.BitwiseNot(width, &temporary, result)
			}
		default:
			return errEvaluatorArity
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *bvEvaluatorSlice) bitwiseNot(child *numeral.Int, width uint32, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if child == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	return e.manager.BitwiseNot(width, child, result)
}

func (e *bvEvaluatorSlice) comparisonResult(compare int, kind bvComparisonKind, result *numeral.Int) error {
	if err := e.manager.Reset(result); err != nil {
		return err
	}
	trueResult := compare < 0 && kind == bvLess || compare <= 0 && kind == bvLessEqual ||
		compare > 0 && kind == bvGreater || compare >= 0 && kind == bvGreaterEqual
	if trueResult {
		return e.manager.SetInt32(result, 1)
	}
	return nil
}

func (e *bvEvaluatorSlice) unsignedCompare(left, right *numeral.Int, kind bvComparisonKind, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if left == nil || right == nil || result == nil || kind > bvGreaterEqual {
		return errEvaluatorArity
	}
	compare, err := e.manager.Compare(left, right)
	if err != nil {
		return err
	}
	return e.comparisonResult(compare, kind, result)
}

func (e *bvEvaluatorSlice) signedCompare(left, right *numeral.Int, width uint32, kind bvComparisonKind, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if left == nil || right == nil || result == nil || kind > bvGreaterEqual {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	var x, y numeral.Int
	defer sourceDeleteIntegers(e.manager, &err, &x, &y)
	if _, _, err = e.signedOperands(left, right, width, &x, &y); err != nil {
		return err
	}
	compare, err := e.manager.Compare(&x, &y)
	if err != nil {
		return err
	}
	err = e.comparisonResult(compare, kind, result)
	return err
}

// identity covers OP_BIT2BOOL and OP_SIGN_EXT, which both copy the child's
// already-normalized bit-vector value without numerical transformation.
func (e *bvEvaluatorSlice) identity(child *numeral.Int, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if child == nil || result == nil {
		return errEvaluatorArity
	}
	return e.manager.Copy(result, child)
}
