package tactic

// Source: src/sat/tactic/atom2bool_var.h:25-40;
// src/sat/tactic/atom2bool_var.cpp:25-42.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
)

// Atom2BoolVar is the concrete SAT atom mapping used by goal2sat, sat2goal and
// the incremental SAT owner. It owns the inherited expr2var mapping and borrows
// its AST manager. The mapping owns one AST reference per distinct atom.
//
// Do not copy this value. Close releases the inherited mapping in source
// destructor order.
type Atom2BoolVar struct {
	self    *Atom2BoolVar
	manager *ast.ASTManager
	mapping *ast.Expr2Var
	closed  bool
}

func NewAtom2BoolVar(m *ast.ASTManager) (*Atom2BoolVar, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	mapping, err := ast.NewExpr2Var(m)
	if err != nil {
		return nil, err
	}
	a := &Atom2BoolVar{manager: m, mapping: mapping}
	a.self = a
	return a, nil
}

func (a *Atom2BoolVar) open() error {
	if a == nil || a.self != a || a.closed || a.mapping == nil {
		return ast.ErrASTHandle
	}
	owner, err := a.mapping.Manager()
	if err != nil {
		return err
	}
	if owner != a.manager {
		return ast.ErrASTHandle
	}
	return nil
}

func (a *Atom2BoolVar) Manager() (*ast.ASTManager, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	return a.manager, nil
}

// Insert preserves atom2bool_var::insert's concrete Boolean-variable type.
func (a *Atom2BoolVar) Insert(n ast.ASTHandle, v sat.BoolVar) error {
	if err := a.open(); err != nil {
		return err
	}
	return a.mapping.Insert(n, uint32(v))
}

// ToBoolVar returns null_bool_var when n is absent.
func (a *Atom2BoolVar) ToBoolVar(n ast.ASTHandle) (sat.BoolVar, error) {
	if err := a.open(); err != nil {
		return sat.NullBoolVar, err
	}
	v, err := a.mapping.ToVar(n)
	if err != nil || v == ^uint32(0) {
		return sat.NullBoolVar, err
	}
	return sat.BoolVar(v), nil
}

// ToVar preserves the inherited expr2var observer for incremental SAT
// consumers that still use the source UINT_MAX sentinel.
func (a *Atom2BoolVar) ToVar(n ast.ASTHandle) (uint32, error) {
	if err := a.open(); err != nil {
		return ^uint32(0), err
	}
	return a.mapping.ToVar(n)
}

func (a *Atom2BoolVar) IsVar(n ast.ASTHandle) (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	return a.mapping.IsVar(n)
}

// InterpretedAtoms is the inherited cumulative interpreted_vars flag. Pop does
// not clear it; Reset does, exactly as expr2var.
func (a *Atom2BoolVar) InterpretedAtoms() (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	return a.mapping.InterpretedVars()
}

func (a *Atom2BoolVar) Entries() ([]ast.ExprVarBinding, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	return a.mapping.Entries()
}

func (a *Atom2BoolVar) Push() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.mapping.Push()
}

func (a *Atom2BoolVar) Pop(scopes uint32) error {
	if err := a.open(); err != nil {
		return err
	}
	return a.mapping.Pop(scopes)
}

func (a *Atom2BoolVar) ResetRecent() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.mapping.ResetRecent()
}

func (a *Atom2BoolVar) Reset() error {
	if err := a.open(); err != nil {
		return err
	}
	return a.mapping.Reset()
}

// mkInverse implements atom2bool_var::mk_inv. Each populated expression slot
// owns one reference through the concrete expr_ref_vector counterpart.
func (a *Atom2BoolVar) mkInverse(out *satGoalLiteralExpressions) error {
	if err := a.open(); err != nil {
		return err
	}
	entries, err := a.mapping.Entries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		literal := sat.MakeLiteral(sat.BoolVar(entry.Variable), false)
		need := literal.Negated().Index() + 1
		if need > uint32(len(out.values)) {
			if err = out.resize(a.manager, need); err != nil {
				return err
			}
		}
		if err = out.set(a.manager, literal.Index(), entry.Expr); err != nil {
			return err
		}
		negated, err := a.manager.MkNot(entry.Expr)
		if err != nil {
			return err
		}
		if err = out.set(a.manager, literal.Negated().Index(), negated); err != nil {
			return err
		}
	}
	return nil
}

func (a *Atom2BoolVar) Close() error {
	if a == nil || a.self != a {
		return ast.ErrASTHandle
	}
	if a.closed {
		return nil
	}
	if a.mapping != nil {
		if err := a.mapping.Close(); err != nil {
			return err
		}
		a.mapping = nil
	}
	a.manager = nil
	a.closed = true
	return nil
}
