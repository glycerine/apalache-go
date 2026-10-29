package tactic

// Concrete ownership operations for goal's native slices.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func goalIncExpr(m *ast.ASTManager, h ast.ASTHandle) error {
	// Source expression-reference arrays can contain empty proof entries.
	// Goal formula operations validate nonnull expressions before storage.
	if h.IsNull() {
		return m.IncRef(h)
	}
	kind, e := m.GetKind(h)
	if e != nil {
		return e
	}
	if kind > ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	return m.IncRef(h)
}

func goalPushExpr(m *ast.ASTManager, values *[]ast.ASTHandle, h ast.ASTHandle) error {
	if e := goalIncExpr(m, h); e != nil {
		return e
	}
	*values = append(*values, h)
	return nil
}

func goalPushDependency(m *ast.ASTManager, values *[]ast.ASTDependency, d ast.ASTDependency) error {
	if e := m.DependencyInc(d); e != nil {
		return e
	}
	*values = append(*values, d)
	return nil
}

func goalSetExpr(m *ast.ASTManager, values []ast.ASTHandle, i uint32, h ast.ASTHandle) error {
	if uint64(i) >= uint64(len(values)) {
		return arena.ErrBounds
	}
	if e := goalIncExpr(m, h); e != nil {
		return e
	}
	consumed, e := m.DecRefConsumed(values[i])
	if !consumed {
		return errors.Join(e, m.DecRef(h))
	}
	values[i] = h
	return e
}

func goalSetDependency(m *ast.ASTManager, values []ast.ASTDependency, i uint32, d ast.ASTDependency) error {
	if uint64(i) >= uint64(len(values)) {
		return arena.ErrBounds
	}
	if e := m.DependencyInc(d); e != nil {
		return e
	}
	consumed, e := m.DependencyDecConsumed(values[i])
	if !consumed {
		return errors.Join(e, m.DependencyDec(d))
	}
	values[i] = d
	return e
}

// Keep only unconsumed ownership if a guard refuses cleanup. A destructor
// error after decrement must never leave a stale reference for the next close.
func goalDropExprs(m *ast.ASTManager, values *[]ast.ASTHandle) error {
	var first error
	remaining := 0
	for _, h := range *values {
		consumed, e := m.DecRefConsumed(h)
		if first == nil {
			first = e
		}
		if !consumed {
			(*values)[remaining] = h
			remaining++
		}
	}
	clear((*values)[remaining:])
	*values = (*values)[:remaining]
	if remaining == 0 {
		*values = nil
	}
	return first
}

func goalDropDependencies(m *ast.ASTManager, values *[]ast.ASTDependency) error {
	var first error
	remaining := 0
	for _, d := range *values {
		consumed, e := m.DependencyDecConsumed(d)
		if first == nil {
			first = e
		}
		if !consumed {
			(*values)[remaining] = d
			remaining++
		}
	}
	clear((*values)[remaining:])
	*values = (*values)[:remaining]
	if remaining == 0 {
		*values = nil
	}
	return first
}

func goalPopExpr(m *ast.ASTManager, values *[]ast.ASTHandle) error {
	last := len(*values) - 1
	if last < 0 {
		return arena.ErrBounds
	}
	consumed, e := m.DecRefConsumed((*values)[last])
	if consumed {
		(*values)[last] = ast.ASTHandle{}
		*values = (*values)[:last]
	}
	return e
}

func goalPopDependency(m *ast.ASTManager, values *[]ast.ASTDependency) error {
	last := len(*values) - 1
	if last < 0 {
		return arena.ErrBounds
	}
	consumed, e := m.DependencyDecConsumed((*values)[last])
	if consumed {
		(*values)[last] = ast.ASTDependency{}
		*values = (*values)[:last]
	}
	return e
}

func goalCopyExprs(m *ast.ASTManager, src []ast.ASTHandle, dst *[]ast.ASTHandle) error {
	var next []ast.ASTHandle
	for _, h := range src {
		if e := goalPushExpr(m, &next, h); e != nil {
			return errors.Join(e, goalDropExprs(m, &next))
		}
	}
	e := goalDropExprs(m, dst)
	if len(*dst) != 0 {
		return errors.Join(e, goalDropExprs(m, &next))
	}
	*dst = next
	return e
}

func goalCopyDependencies(m *ast.ASTManager, src []ast.ASTDependency, dst *[]ast.ASTDependency) error {
	var next []ast.ASTDependency
	for _, d := range src {
		if e := goalPushDependency(m, &next, d); e != nil {
			return errors.Join(e, goalDropDependencies(m, &next))
		}
	}
	e := goalDropDependencies(m, dst)
	if len(*dst) != 0 {
		return errors.Join(e, goalDropDependencies(m, &next))
	}
	*dst = next
	return e
}
