package sls

// Source: src/tactic/sls/sls_evaluator.h:25-116, constructor/destructor and
// the complete retained basic-family Boolean operator cases.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrEvaluatorUnsupported = errors.New("sls: evaluator operation is not ported")

// Evaluator owns its three source numeral constants and borrows the AST
// manager, tracker, integer manager, and power cache.
type Evaluator struct {
	self           *Evaluator
	manager        *ast.ASTManager
	tracker        *Tracker
	numbers        *numeral.IntegerManager
	powers         *powers
	zero, one, two numeral.Int
	closing        int
	closed         bool
}

func NewEvaluator(manager *ast.ASTManager, tracker *Tracker, numbers *numeral.IntegerManager, powers *powers) (*Evaluator, error) {
	if manager == nil || tracker == nil || tracker.manager != manager || tracker.open() != nil || numbers == nil || numbers.Closed() || tracker.numbers != numbers || powers == nil || powers.closed || powers.manager != numbers || tracker.powers != powers {
		return nil, ErrTracker
	}
	e := &Evaluator{manager: manager, tracker: tracker, numbers: numbers, powers: powers, zero: numeral.NewInt(0), one: numeral.NewInt(1), two: numeral.NewInt(2)}
	e.self = e
	return e, nil
}

func (e *Evaluator) open() error {
	if e == nil || e.self != e || e.closed || e.manager == nil || e.tracker == nil || e.tracker.open() != nil || e.numbers == nil || e.numbers.Closed() || e.powers == nil || e.powers.closed {
		return ErrTracker
	}
	return nil
}

// Evaluate computes the source basic-family and bit-vector app values from the
// current tracker assignments.
func (e *Evaluator) Evaluate(node ast.ASTHandle, result *numeral.Int) error {
	if err := e.open(); err != nil {
		return err
	}
	if result == nil {
		return ErrTracker
	}
	kind, err := e.manager.GetKind(node)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	n, err := e.manager.GetNumArgs(node)
	if err != nil {
		return err
	}
	if n == 0 {
		value, x := e.tracker.GetValue(node)
		if x != nil {
			return x
		}
		return e.numbers.Copy(result, value)
	}
	if err = e.numbers.Copy(result, &e.zero); err != nil {
		return err
	}
	family, err := e.manager.GetFamilyID(node)
	if err != nil {
		return err
	}
	op, err := e.manager.GetDeclKind(node)
	if err != nil {
		return err
	}
	argValue := func(index uint32) (*numeral.Int, error) {
		h, x := e.manager.GetArg(node, index)
		if x != nil {
			return nil, x
		}
		return e.tracker.GetValue(h)
	}
	if family != e.manager.BasicFamilyID() {
		bvFamily, x := e.manager.BitVectorUtilityFamilyID()
		if x != nil {
			return x
		}
		if family == bvFamily {
			return e.evaluateBV(node, n, op, argValue, result)
		}
		return ErrEvaluatorUnsupported
	}
	switch op {
	case ast.ASTOpAnd:
		if err = e.numbers.Copy(result, &e.one); err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			value, x := argValue(i)
			if x != nil {
				return x
			}
			different, x := e.numbers.NotEqual(value, result)
			if x != nil {
				return x
			}
			if different {
				return e.numbers.Copy(result, &e.zero)
			}
		}
		return nil
	case ast.ASTOpOr:
		for i := uint32(0); i < n; i++ {
			value, x := argValue(i)
			if x != nil {
				return x
			}
			different, x := e.numbers.NotEqual(value, result)
			if x != nil {
				return x
			}
			if different {
				return e.numbers.Copy(result, &e.one)
			}
		}
		return nil
	case ast.ASTOpNot:
		if n != 1 {
			return ast.ErrASTDomain
		}
		value, x := argValue(0)
		if x != nil {
			return x
		}
		zero, x := e.numbers.IsZero(value)
		if x != nil {
			return x
		}
		if zero {
			return e.numbers.Copy(result, &e.one)
		}
		return e.numbers.Copy(result, &e.zero)
	case ast.ASTOpEq:
		if n < 2 {
			return ast.ErrASTDomain
		}
		if err = e.numbers.Copy(result, &e.one); err != nil {
			return err
		}
		first, x := argValue(0)
		if x != nil {
			return x
		}
		for i := uint32(1); i < n; i++ {
			value, y := argValue(i)
			if y != nil {
				return y
			}
			different, y := e.numbers.NotEqual(value, first)
			if y != nil {
				return y
			}
			if different {
				return e.numbers.Copy(result, &e.zero)
			}
		}
		return nil
	case ast.ASTOpDistinct:
		if err = e.numbers.Copy(result, &e.one); err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			left, x := argValue(i)
			if x != nil {
				return x
			}
			for j := i + 1; j < n; j++ {
				right, y := argValue(j)
				if y != nil {
					return y
				}
				equal, y := e.numbers.Equal(left, right)
				if y != nil {
					return y
				}
				if equal {
					return e.numbers.Copy(result, &e.zero)
				}
			}
		}
		return nil
	case ast.ASTOpIte:
		if n != 3 {
			return ast.ErrASTDomain
		}
		condition, x := argValue(0)
		if x != nil {
			return x
		}
		one, x := e.numbers.IsOne(condition)
		if x != nil {
			return x
		}
		branch := uint32(2)
		if one {
			branch = 1
		}
		value, x := argValue(branch)
		if x != nil {
			return x
		}
		return e.numbers.Copy(result, value)
	default:
		return ErrEvaluatorUnsupported
	}
}

// Close is retryable across each source numeral destructor.
func (e *Evaluator) Close() error {
	if e == nil || e.self != e {
		return ErrTracker
	}
	if e.closed {
		return nil
	}
	if e.numbers == nil || e.numbers.Closed() {
		return ErrTracker
	}
	values := []*numeral.Int{&e.zero, &e.one, &e.two}
	for e.closing < len(values) {
		if err := e.numbers.Del(values[e.closing]); err != nil {
			return err
		}
		e.closing++
	}
	e.closed = true
	return nil
}
