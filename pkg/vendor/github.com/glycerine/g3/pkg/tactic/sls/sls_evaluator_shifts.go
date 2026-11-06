package sls

// Source: src/tactic/sls/sls_evaluator.h OP_BASHR, OP_BLSHR, and OP_BSHL.
// Christoph Winter, 2012-02-29.

import "github.com/glycerine/g3/pkg/util/numeral"

func (e *bvEvaluatorSlice) arithmeticShiftRight(value, amount *numeral.Int, width uint32, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if value == nil || amount == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	if err = e.manager.Copy(result, value); err != nil {
		return err
	}
	signPower, err := e.powers.borrow(width - 1)
	if err != nil {
		return err
	}
	var first, shift, temporary numeral.Int
	defer sourceDeleteIntegers(e.manager, &err, &first, &shift, &temporary)
	if err = e.manager.BitwiseAnd(result, signPower, &first); err != nil {
		return err
	}
	if err = e.manager.Copy(&shift, amount); err != nil {
		return err
	}
	two := numeral.NewInt(2)
	for {
		zero, zeroErr := e.manager.IsZero(&shift)
		if zeroErr != nil {
			return zeroErr
		}
		if zero {
			break
		}
		if err = e.manager.MachineDiv(result, &two, &temporary); err != nil {
			return err
		}
		if err = e.manager.Add(&temporary, &first, result); err != nil {
			return err
		}
		if err = e.manager.Dec(&shift); err != nil {
			return err
		}
	}
	return nil
}

func (e *bvEvaluatorSlice) logicalShiftRight(value, amount *numeral.Int, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if value == nil || amount == nil || result == nil {
		return errEvaluatorArity
	}
	if err = e.manager.Copy(result, value); err != nil {
		return err
	}
	var shift numeral.Int
	defer sourceDeleteIntegers(e.manager, &err, &shift)
	if err = e.manager.Copy(&shift, amount); err != nil {
		return err
	}
	two := numeral.NewInt(2)
	for {
		zero, zeroErr := e.manager.IsZero(&shift)
		if zeroErr != nil {
			return zeroErr
		}
		if zero {
			break
		}
		if err = e.manager.MachineDiv(result, &two, result); err != nil {
			return err
		}
		if err = e.manager.Dec(&shift); err != nil {
			return err
		}
	}
	return nil
}

func (e *bvEvaluatorSlice) shiftLeft(value, amount *numeral.Int, width uint32, result *numeral.Int) (err error) {
	if err = e.arithmeticOwner(); err != nil {
		return err
	}
	if value == nil || amount == nil || result == nil {
		return errEvaluatorArity
	}
	if width == 0 {
		return errEvaluatorWidth
	}
	if err = e.manager.Copy(result, value); err != nil {
		return err
	}
	var shift numeral.Int
	defer sourceDeleteIntegers(e.manager, &err, &shift)
	if err = e.manager.Copy(&shift, amount); err != nil {
		return err
	}
	two := numeral.NewInt(2)
	for {
		zero, zeroErr := e.manager.IsZero(&shift)
		if zeroErr != nil {
			return zeroErr
		}
		if zero {
			break
		}
		if err = e.manager.Mul(result, &two, result); err != nil {
			return err
		}
		if err = e.manager.Dec(&shift); err != nil {
			return err
		}
	}
	power, err := e.powers.borrow(width)
	if err != nil {
		return err
	}
	err = e.manager.Rem(result, power, result)
	return err
}
