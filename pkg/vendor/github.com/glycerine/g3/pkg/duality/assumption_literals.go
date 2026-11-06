package duality

// Source: duality_rpfp.cpp:1166-1185.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type assumptionEntry struct {
	conjunct, literal ast.ASTRef
	guarded           bool
}

// AssumptionLiteralCache owns conjunct keys and fresh literals for one borrowed
// solver. Its guard assertions must stay in that solver for the cache lifetime.
type AssumptionLiteralCache struct {
	context *Context
	solver  solver.Solver
	entries map[ast.ASTHandle]*assumptionEntry
	closed  bool
}

func NewAssumptionLiteralCache(context *Context, target solver.Solver) (*AssumptionLiteralCache, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrWrapperOwner
	}
	m, err := target.Manager()
	if err != nil {
		return nil, err
	}
	if m != context.manager {
		return nil, ErrContextMismatch
	}
	return &AssumptionLiteralCache{context: context, solver: target, entries: make(map[ast.ASTHandle]*assumptionEntry)}, nil
}

// GetAssumptionLits returns owning expression copies in conjunct traversal
// order. Optional mappings borrow handles from the cache's owned entries.
func (c *AssumptionLiteralCache) GetAssumptionLits(formula *Expr, mappings map[ast.ASTHandle]ast.ASTHandle) (out []*Expr, result error) {
	if c == nil || c.closed {
		return nil, ErrWrapperOwner
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != c.context {
		return nil, ErrContextMismatch
	}
	var ownedOutput []*Expr
	conjuncts, err := CollectConjuncts(formula, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, conjunct := range conjuncts {
			result = errors.Join(result, conjunct.Close())
		}
		if result != nil {
			for _, literal := range ownedOutput {
				result = errors.Join(result, literal.Close())
			}
			out = nil
		}
	}()
	m := c.context.manager
	for _, conjunct := range conjuncts {
		h, err := conjunct.Raw()
		if err != nil {
			return nil, err
		}
		entry := c.entries[h]
		if entry == nil {
			owned, err := m.CopyRef(conjunct.value.ref)
			if err != nil {
				return nil, err
			}
			entry = &assumptionEntry{conjunct: owned}
			c.entries[h] = entry
		}
		if entry.literal == (ast.ASTRef{}) {
			sort, err := m.MkBoolSort()
			if err != nil {
				return nil, err
			}
			fresh, err := m.MkFreshConst("@alit", sort)
			if err != nil {
				return nil, err
			}
			entry.literal, err = m.NewRef(fresh)
			if err != nil {
				return nil, err
			}
		}
		literal, err := m.RefGet(entry.literal)
		if err != nil {
			return nil, err
		}
		if !entry.guarded {
			guard, err := m.MkImplies(literal, h)
			if err != nil {
				return nil, err
			}
			ref, err := m.NewRef(guard)
			if err != nil {
				return nil, err
			}
			assertErr := c.solver.Assert(guard)
			closeErr := m.DelRef(&ref)
			if assertErr == nil {
				entry.guarded = true
			}
			if err = errors.Join(assertErr, closeErr); err != nil {
				return nil, err
			}
		}
		if mappings != nil {
			mappings[literal] = h
		}
		copy, err := NewExpr(c.context, literal)
		if err != nil {
			return nil, err
		}
		ownedOutput = append(ownedOutput, copy)
	}
	return ownedOutput, nil
}
func (c *AssumptionLiteralCache) Close() error {
	if c == nil || c.closed {
		return nil
	}
	m := c.context.manager
	for key, entry := range c.entries {
		if entry.literal != (ast.ASTRef{}) {
			if err := m.DelRef(&entry.literal); err != nil {
				return err
			}
		}
		if entry.conjunct != (ast.ASTRef{}) {
			if err := m.DelRef(&entry.conjunct); err != nil {
				return err
			}
		}
		delete(c.entries, key)
	}
	c.closed = true
	return nil
}
