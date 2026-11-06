package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// Source: src/tactic/arith/fm_tactic.cpp:1445-1516.
func (e *FMEliminator) discard(cs []*FMConstraint) error {
	for _, c := range cs {
		if err := e.Store.Delete(c); err != nil {
			return err
		}
	}
	return nil
}

func (e *FMEliminator) Try(x uint32) (bool, error) {
	if e.Inconsistent || e.Variables.IsForbidden(x) {
		return false, nil
	}
	e.Variables.Cleanup(x)
	lowers := e.Variables.Index.Lower[x]
	uppers := e.Variables.Index.Upper[x]
	if len(lowers) == 0 || len(uppers) == 0 {
		if err := e.markDead(x); err != nil {
			return false, err
		}
		return true, e.Store.diagnostics.eliminationEnd(0, 0, true, false)
	}
	if !e.Config.permits(len(lowers), len(uppers)) || !e.Variables.CanEliminate(x) {
		return false, nil
	}
	limit := uint32(len(lowers)) + uint32(len(uppers)) + e.Config.Extra
	if e.Counter != nil {
		*e.Counter += uint32(len(lowers)) * uint32(len(uppers))
	}
	if err := e.Store.diagnostics.elimination(x, lowers, uppers); err != nil {
		return false, err
	}
	created := make([]*FMConstraint, 0)
	for _, lower := range lowers {
		for _, upper := range uppers {
			if e.Inconsistent || uint32(len(created)) > limit {
				err := e.Store.diagnostics.eliminationEnd(0, uint32(len(created)), false, true)
				return false, errors.Join(err, e.discard(created))
			}
			if e.Counter != nil {
				*e.Counter += uint32(len(lower.Variables) + len(upper.Variables) + len(lower.Literals) + len(upper.Literals))
			}
			resolution, err := FMResolve(e.Store, e.Variables, lower, upper, x)
			if err != nil {
				_ = e.discard(created)
				return false, err
			}
			if resolution.Inconsistent {
				if err := e.setInconsistent(resolution.Dependency); err != nil {
					_ = e.discard(created)
					return false, err
				}
				continue
			}
			if resolution.Constraint != nil {
				created = append(created, resolution.Constraint)
			}
		}
	}
	if err := e.markDead(x); err != nil {
		return false, errors.Join(err, e.discard(created))
	}
	if e.Counter != nil {
		*e.Counter += uint32(len(created))
	}
	for _, c := range created {
		if err := e.Variables.Index.Backward(c); err != nil {
			return false, err
		}
		switch e.Variables.Register(c) {
		case FMRegistrationFalse:
			e.Inconsistent = true
			if err := e.Store.Delete(c); err != nil {
				return false, err
			}
		case FMRegistrationDeferred:
			if err := e.Store.diagnostics.deferred(c); err != nil {
				return false, err
			}
			if e.Output != nil {
				formula, err := e.Expressions.ConstraintExpression(c)
				if err != nil {
					return false, err
				}
				if err = e.Output.AssertWithProof(formula, ast.ASTHandle{}, c.Dependency); err != nil {
					return false, err
				}
				if err = e.Store.Delete(c); err != nil {
					return false, err
				}
			}
		}
	}
	return true, e.Store.diagnostics.eliminationEnd(uint32(len(lowers))+uint32(len(uppers)), uint32(len(created)), false, false)
}
