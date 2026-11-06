package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:128-190; concrete MPFX instantiation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrFixedUnsupportedAtom = errors.New("unsupported atom")
var ErrFixedArithmeticLHS = errors.New("use simplify tactic with option :arith-lhs true")

// InternalizeInequality follows mk_ineq for the selected MPFX context.
// The context owns the returned inequality and its rounded scalar.
func (c *FixedExprConverter) InternalizeInequality(h ast.ASTHandle) (atom *subpaving.FixedInequality, err error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	neg := false
	for {
		yes, child, e := c.ast.IsNot(h)
		if e != nil {
			return nil, e
		}
		if !yes {
			break
		}
		neg = !neg
		h = child
	}
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp {
		return nil, ErrFixedUnsupportedAtom
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil {
		return nil, err
	}
	operator, err := c.ast.GetDeclKind(h)
	if err != nil {
		return nil, err
	}
	if family != c.family || (operator != 2 && operator != 3) {
		return nil, ErrFixedUnsupportedAtom
	}
	lower, open := operator == 3, false
	if neg {
		lower, open = !lower, true
	}
	rhs, err := c.ast.GetArg(h, 1)
	if err != nil {
		return nil, err
	}
	value, _, numeralRHS, err := c.ast.ArithmeticNumeralText(rhs)
	if err != nil {
		return nil, err
	}
	if !numeralRHS {
		return nil, ErrFixedArithmeticLHS
	}
	qm := c.qm
	k := numeral.NewRational()
	var n, d numeral.Int
	defer func() { err = errors.Join(err, qm.Z.Del(&d), qm.Z.Del(&n), qm.Del(&k)) }()
	if err = qm.SetString(&k, value); err != nil {
		return nil, err
	}
	lhs, err := c.ast.GetArg(h, 0)
	if err != nil {
		return nil, err
	}
	x, err := c.InternalizeTerm(lhs, &n, &d)
	if err != nil {
		return nil, err
	}
	if err = qm.MulInt(&d, &k, &k); err != nil {
		return nil, err
	}
	if err = qm.DivInt(&k, &n, &k); err != nil {
		return nil, err
	}
	sign, err := qm.Z.Sign(&n)
	if err != nil {
		return nil, err
	}
	if sign < 0 {
		lower = !lower
	}
	if err = emitGoalInequalityTrace(c.trace, qm, x, &k, lower, open); err != nil {
		return nil, err
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return nil, err
	}
	return shell.MkInequality(x, &k, lower, open)
}

// InternalizeClause uses a native typed slice for the source ref_buffer,
// preserving literal and clause order and releasing unowned atoms on unwind.
func (c *FixedExprConverter) InternalizeClause(h ast.ASTHandle) (err error) {
	if err := c.check(); err != nil {
		return err
	}
	args := []ast.ASTHandle{h}
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTApp {
		family, err := c.ast.GetFamilyID(h)
		if err != nil {
			return err
		}
		operator, err := c.ast.GetDeclKind(h)
		if err != nil {
			return err
		}
		if family == c.ast.BasicFamilyID() && operator == ast.ASTOpOr {
			count, err := c.ast.GetNumArgs(h)
			if err != nil {
				return err
			}
			args = make([]ast.ASTHandle, count)
			for i := range args {
				args[i], err = c.ast.GetArg(h, uint32(i))
				if err != nil {
					return err
				}
			}
		}
	}
	atoms := make([]*subpaving.FixedInequality, 0, len(args))
	defer func() {
		for _, atom := range atoms {
			if cleanup := c.engine.shell.DiscardUnreferencedInequality(atom); cleanup != nil {
				err = errors.Join(err, cleanup)
				break
			}
		}
	}()
	for _, literal := range args {
		atom, err := c.InternalizeInequality(literal)
		if err != nil {
			return err
		}
		atoms = append(atoms, atom)
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return err
	}
	_, err = shell.AddClause(atoms)
	return err
}

// InternalizeGoal processes formulas in source goal order. Search and tactic
// result ownership remain separate steps of the enclosing tactic.
func (c *FixedExprConverter) InternalizeGoal(g *goaltactic.Goal) error {
	if err := c.check(); err != nil {
		return err
	}
	if g == nil {
		return arena.ErrInvalidSlot
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		form, err := g.Form(i)
		if err != nil {
			return err
		}
		if err := c.InternalizeClause(form); err != nil {
			return err
		}
	}
	return nil
}

// SearchGoal follows the source's internalize-then-search order for MPFX.
func (c *FixedExprConverter) SearchGoal(g *goaltactic.Goal) error {
	if err := c.InternalizeGoal(g); err != nil {
		return err
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return err
	}
	return shell.Search()
}
