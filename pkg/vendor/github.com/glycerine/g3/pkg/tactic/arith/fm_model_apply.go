package arith

// Source: src/tactic/arith/fm_tactic.cpp:180-244.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"math/big"
)

func (c *FMModelConverter) ApplyModel(out *tactic.ModelRef, _ uint32) (err error) {
	if e := c.open(); e != nil {
		return e
	}
	md, e := out.Get()
	if e != nil {
		return e
	}
	if md == nil || md.Manager() != c.manager {
		return ast.ErrASTHandle
	}
	if e = c.traceModel(md, 184, true); e != nil {
		return e
	}
	ev, e := model.NewModelEvaluator(md, c.services)
	if e != nil {
		if ev != nil {
			e = errors.Join(e, ev.Close())
		}
		return e
	}
	defer func() { err = errors.Join(err, ev.Close()) }()
	if e = ev.SetModelCompletion(true); e != nil {
		return e
	}
	ref, e := c.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&ref)) }()
	for i := len(c.variables) - 1; i >= 0; i-- {
		if e = c.traceVariable(c.variables[i], ast.ASTHandle{}); e != nil {
			return e
		}
		var lower, upper *big.Rat
		for _, clause := range c.clauses[i] {
			if !c.manager.ResourceLimit().Inc() {
				return tactic.NewTacticException(c.manager.ResourceLimit().CancelMessage())
			}
			kind, value, e := c.processBound(c.variables[i], clause, ev, ref)
			if e != nil {
				return e
			}
			if e = c.traceBound(kind, value, clause); e != nil {
				return e
			}
			switch kind {
			case fmLowerBound:
				if lower == nil || value.Cmp(lower) > 0 {
					lower = value
				}
			case fmUpperBound:
				if upper == nil || value.Cmp(upper) < 0 {
					upper = value
				}
			}
		}
		sort, e := c.manager.GetRange(c.variables[i])
		if e != nil {
			return e
		}
		kind, e := c.manager.GetDeclKind(sort)
		if e != nil {
			return e
		}
		var witness *big.Rat
		if kind == 1 {
			if lower != nil {
				witness = lower
			} else if upper != nil {
				witness = upper
			} else {
				witness = new(big.Rat)
			}
		} else {
			switch {
			case lower != nil && upper != nil:
				witness = new(big.Rat).Quo(new(big.Rat).Add(lower, upper), big.NewRat(2, 1))
			case lower != nil:
				witness = new(big.Rat).Add(lower, big.NewRat(1, 1))
			case upper != nil:
				witness = new(big.Rat).Sub(upper, big.NewRat(1, 1))
			default:
				witness = new(big.Rat)
			}
		}
		value, e := c.manager.ArithmeticNumeralRat(witness, kind == 1)
		if e != nil {
			return e
		}
		if e = c.traceVariable(c.variables[i], value); e != nil {
			return e
		}
		if e = md.RegisterConstant(c.variables[i], value); e != nil {
			return e
		}
	}
	return c.traceModel(md, 243, false)
}
