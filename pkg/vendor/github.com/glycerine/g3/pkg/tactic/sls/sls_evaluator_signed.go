package sls

// Source: src/tactic/sls/sls_evaluator.h signed division, remainder, and
// modulo cases. Christoph Winter, 2012-02-29.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func sourceDeleteIntegers(manager *numeral.IntegerManager, err *error, values ...*numeral.Int) {
	for _, value := range values {
		if cleanup := manager.Del(value); cleanup != nil {
			*err = errors.Join(*err, cleanup)
		}
	}
}

func (e *bvEvaluatorSlice) signedOperands(left, right *numeral.Int, width uint32, x, y *numeral.Int) (power, half *numeral.Int, err error) {
	if err = e.manager.Copy(x, left); err != nil {
		return nil, nil, err
	}
	if err = e.manager.Copy(y, right); err != nil {
		return nil, nil, err
	}
	if power, err = e.powers.borrow(width); err != nil {
		return nil, nil, err
	}
	if half, err = e.powers.borrow(width - 1); err != nil {
		return nil, nil, err
	}
	ge, err := e.manager.GreaterEqual(x, half)
	if err != nil {
		return nil, nil, err
	}
	if ge {
		if err = e.manager.Sub(x, power, x); err != nil {
			return nil, nil, err
		}
	}
	ge, err = e.manager.GreaterEqual(y, half)
	if err != nil {
		return nil, nil, err
	}
	if ge {
		err = e.manager.Sub(y, power, y)
	}
	return power, half, err
}

func (e *bvEvaluatorSlice) signedDiv(left, right *numeral.Int, width uint32, result *numeral.Int) (err error) {
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
	defer sourceDeleteIntegers(e.manager, &err, &x, &y)
	power, _, err := e.signedOperands(left, right, width, &x, &y)
	if err != nil {
		return err
	}
	zero, err := e.manager.IsZero(&y)
	if err != nil {
		return err
	}
	if zero {
		negative, signErr := e.manager.IsNeg(&x)
		if signErr != nil {
			return signErr
		}
		if negative {
			err = e.manager.SetInt32(result, 1)
		} else {
			// The source performs a second cache lookup on this branch.
			all, borrowErr := e.powers.borrow(width)
			if borrowErr != nil {
				return borrowErr
			}
			if err = e.manager.Copy(result, all); err == nil {
				err = e.manager.Dec(result)
			}
		}
	} else {
		err = e.manager.MachineDiv(&x, &y, result)
	}
	if err != nil {
		return err
	}
	negative, err := e.manager.IsNeg(result)
	if err != nil {
		return err
	}
	if negative {
		err = e.manager.Add(result, power, result)
	}
	return err
}

func (e *bvEvaluatorSlice) signedRem(left, right *numeral.Int, width uint32, result *numeral.Int) (err error) {
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
	defer sourceDeleteIntegers(e.manager, &err, &x, &y)
	power, _, err := e.signedOperands(left, right, width, &x, &y)
	if err != nil {
		return err
	}
	zero, err := e.manager.IsZero(&y)
	if err != nil {
		return err
	}
	if zero {
		err = e.manager.Copy(result, &x)
	} else {
		err = e.manager.Rem(&x, &y, result)
	}
	if err != nil {
		return err
	}
	negative, err := e.manager.IsNeg(result)
	if err != nil {
		return err
	}
	if negative {
		err = e.manager.Add(result, power, result)
	}
	return err
}

func (e *bvEvaluatorSlice) signedMod(left, right *numeral.Int, width uint32, result *numeral.Int) (err error) {
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
	defer sourceDeleteIntegers(e.manager, &err, &x, &y)
	power, _, err := e.signedOperands(left, right, width, &x, &y)
	if err != nil {
		return err
	}
	zero, err := e.manager.IsZero(&y)
	if err != nil {
		return err
	}
	if zero {
		err = e.manager.Copy(result, &x)
	} else {
		negativeX, signErr := e.manager.IsNeg(&x)
		if signErr != nil {
			return signErr
		}
		negativeY, signErr := e.manager.IsNeg(&y)
		if signErr != nil {
			return signErr
		}
		err = func() (innerErr error) {
			var absX, absY numeral.Int
			defer sourceDeleteIntegers(e.manager, &innerErr, &absX, &absY)
			if innerErr = e.manager.Copy(&absX, &x); innerErr != nil {
				return innerErr
			}
			if innerErr = e.manager.Copy(&absY, &y); innerErr != nil {
				return innerErr
			}
			if negativeX {
				if innerErr = e.manager.Neg(&absX); innerErr != nil {
					return innerErr
				}
			}
			if negativeY {
				if innerErr = e.manager.Neg(&absY); innerErr != nil {
					return innerErr
				}
			}
			if innerErr = e.manager.Mod(&absX, &absY, result); innerErr != nil {
				return innerErr
			}
			resultZero, zeroErr := e.manager.IsZero(result)
			if zeroErr != nil {
				return zeroErr
			}
			switch {
			case resultZero || !negativeX && !negativeY:
			case negativeX && !negativeY:
				if innerErr = e.manager.Neg(result); innerErr == nil {
					innerErr = e.manager.Add(result, &y, result)
				}
			case !negativeX && negativeY:
				innerErr = e.manager.Add(result, &y, result)
			default:
				innerErr = e.manager.Neg(result)
			}
			return innerErr
		}()
	}
	if err != nil {
		return err
	}
	negative, err := e.manager.IsNeg(result)
	if err != nil {
		return err
	}
	if negative {
		err = e.manager.Add(result, power, result)
	}
	return err
}
