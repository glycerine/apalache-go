package sls

// Source: src/tactic/sls/sls_evaluator.h OP_BADD, OP_BSUB, OP_BMUL,
// OP_BNEG, OP_BUDIV/OP_BUDIV0/OP_BUDIV_I, and
// OP_BUREM/OP_BUREM0/OP_BUREM_I cases.
// Christoph Winter, 2012-02-29.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (e *bvEvaluatorSlice) arithmeticOwner() error {
	if e == nil || e.manager == nil || e.manager.Closed() || e.powers == nil || e.powers.closed {
		return errEvaluatorOwner
	}
	return nil
}

func (e *bvEvaluatorSlice) add(args []*numeral.Int, width uint32, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if len(args) < 2 || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	if err := e.manager.Reset(result); err != nil {
		return err
	}
	for _, arg := range args {
		if arg == nil {
			return errEvaluatorArity
		}
		if err := e.manager.Add(result, arg, result); err != nil {
			return err
		}
	}
	power, err := e.powers.borrow(width)
	if err != nil {
		return err
	}
	return e.manager.Rem(result, power, result)
}

func (e *bvEvaluatorSlice) sub(left, right *numeral.Int, width uint32, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if left == nil || right == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	power, err := e.powers.borrow(width)
	if err != nil {
		return err
	}
	var temporary numeral.Int
	defer func() {
		if cleanup := e.manager.Del(&temporary); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	if err = e.manager.Sub(left, right, &temporary); err != nil {
		return err
	}
	err = e.manager.Mod(&temporary, power, result)
	return err
}

func (e *bvEvaluatorSlice) mul(args []*numeral.Int, width uint32, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if len(args) < 2 || result == nil || args[0] == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	if err := e.manager.Copy(result, args[0]); err != nil {
		return err
	}
	for _, arg := range args[1:] {
		if arg == nil {
			return errEvaluatorArity
		}
		if err := e.manager.Mul(result, arg, result); err != nil {
			return err
		}
	}
	power, err := e.powers.borrow(width)
	if err != nil {
		return err
	}
	return e.manager.Rem(result, power, result)
}

func (e *bvEvaluatorSlice) neg(child *numeral.Int, width uint32, result *numeral.Int) error {
	if err := e.arithmeticOwner(); err != nil {
		return err
	}
	if child == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	zero, err := e.manager.IsZero(child)
	if err != nil {
		return err
	}
	if zero {
		return e.manager.Reset(result)
	}
	if err = e.manager.BitwiseNot(width, child, result); err != nil {
		return err
	}
	return e.manager.Inc(result)
}

func (e *bvEvaluatorSlice) unsignedDiv(left, right *numeral.Int, width uint32, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if left == nil || right == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	var x, y numeral.Int
	defer func() {
		// The source explicitly deletes x before y.
		if cleanup := e.manager.Del(&x); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
		if cleanup := e.manager.Del(&y); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	if err = e.manager.Copy(&x, left); err != nil {
		return err
	}
	if err = e.manager.Copy(&y, right); err != nil {
		return err
	}
	zero, err := e.manager.IsZero(&y)
	if err != nil {
		return err
	}
	if zero {
		power, borrowErr := e.powers.borrow(width)
		if borrowErr != nil {
			return borrowErr
		}
		if err = e.manager.Copy(result, power); err != nil {
			return err
		}
		return e.manager.Dec(result)
	}
	err = e.manager.MachineDiv(&x, &y, result)
	return err
}

func (e *bvEvaluatorSlice) unsignedRem(left, right *numeral.Int, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if left == nil || right == nil || result == nil {
		return errEvaluatorArity
	}
	var x, y numeral.Int
	defer func() {
		// The source explicitly deletes x before y.
		if cleanup := e.manager.Del(&x); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
		if cleanup := e.manager.Del(&y); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	if err = e.manager.Copy(&x, left); err != nil {
		return err
	}
	if err = e.manager.Copy(&y, right); err != nil {
		return err
	}
	zero, err := e.manager.IsZero(&y)
	if err != nil {
		return err
	}
	if zero {
		return e.manager.Copy(result, &x)
	}
	err = e.manager.Mod(&x, &y, result)
	return err
}
