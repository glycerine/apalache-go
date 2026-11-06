package sls

// Source: src/tactic/sls/sls_evaluator.h OP_CONCAT and OP_EXTRACT cases.
// Christoph Winter, 2012-02-29.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	errEvaluatorOwner = errors.New("sls: invalid evaluator owner")
	errEvaluatorArity = errors.New("sls: invalid evaluator arity")
	errEvaluatorWidth = errors.New("sls: invalid bit-vector width")
)

// bvEvaluatorSlice is the concrete numerical part of the retained evaluator's
// concat/extract cases.  The later AST dispatcher supplies tracker values and
// declaration widths; this owner does not invent an AST or engine facade.
type bvEvaluatorSlice struct {
	manager *numeral.IntegerManager
	powers  *powers
}

func newBVEvaluatorSlice(manager *numeral.IntegerManager, powers *powers) (*bvEvaluatorSlice, error) {
	if manager == nil || manager.Closed() || powers == nil || powers.closed || powers.manager != manager {
		return nil, errEvaluatorOwner
	}
	return &bvEvaluatorSlice{manager: manager, powers: powers}, nil
}

// concat preserves the source left-to-right accumulation and lazy power-cache
// lookup order. widths[i] is the actual BV sort width of args[i].
func (e *bvEvaluatorSlice) concat(args []*numeral.Int, widths []uint32, result *numeral.Int) error {
	if e == nil || e.manager == nil || e.manager.Closed() || e.powers == nil || e.powers.closed {
		return errEvaluatorOwner
	}
	if len(args) < 2 || len(widths) != len(args) || result == nil {
		return errEvaluatorArity
	}
	if err := e.manager.Reset(result); err != nil {
		return err
	}
	for i, arg := range args {
		if arg == nil || widths[i] == 0 {
			return errEvaluatorWidth
		}
		if i != 0 {
			power, err := e.powers.borrow(widths[i])
			if err != nil {
				return err
			}
			if err = e.manager.Mul(result, power, result); err != nil {
				return err
			}
		}
		if err := e.manager.Add(result, arg, result); err != nil {
			return err
		}
	}
	return nil
}

// extract computes [high:low] exactly as the source: first remainder modulo
// 2^(high+1), then machine division by 2^low.
func (e *bvEvaluatorSlice) extract(child *numeral.Int, high, low uint32, result *numeral.Int) error {
	if e == nil || e.manager == nil || e.manager.Closed() || e.powers == nil || e.powers.closed {
		return errEvaluatorOwner
	}
	if child == nil || result == nil {
		return errEvaluatorArity
	}
	if high < low || high == math.MaxUint32 {
		return errEvaluatorWidth
	}
	power, err := e.powers.borrow(high + 1)
	if err != nil {
		return err
	}
	if err = e.manager.Rem(child, power, result); err != nil {
		return err
	}
	return e.manager.MachineDiv2kInPlace(result, low)
}
