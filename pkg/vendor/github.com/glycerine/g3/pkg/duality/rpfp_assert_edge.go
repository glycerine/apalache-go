package duality

// Source: duality_rpfp.cpp:917-975.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *RPFP) GetEdgeFormula(edge *Edge, persist int, withChildren, underapprox bool) (_ *Expr, result error) {
	if err := g.solverReady(); err != nil {
		return nil, err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return nil, ErrWrapperOwner
	}
	if edge.dual != (ast.ASTRef{}) {
		h, err := g.context.manager.RefGet(edge.dual)
		if err != nil {
			return nil, err
		}
		return NewExpr(g.context, h)
	}
	reducedTimer, err := startProfile(g.context, "ReducedDualEdge")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, reducedTimer.Close()) }()
	formula, err := g.ReducedDualEdge(edge)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	if err = reducedTimer.Close(); err != nil {
		return nil, err
	}
	childrenTimer, err := startProfile(g.context, "getting children")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, childrenTimer.Close()) }()
	if underapprox {
		var pins []*Expr
		defer func() {
			for _, v := range pins {
				result = errors.Join(result, v.Close())
			}
		}()
		args := make([]ast.ASTHandle, 0, len(edge.Children))
		for _, child := range edge.Children {
			flag, err := g.UnderapproxFlag(child)
			if err != nil {
				return nil, err
			}
			pins = append(pins, flag)
			negative, err := flag.Not()
			if err != nil {
				return nil, err
			}
			pins = append(pins, negative)
			under, err := g.GetUnderapprox(child)
			if err != nil {
				return nil, err
			}
			pins = append(pins, under)
			guarded, err := negative.Or(under)
			if err != nil {
				return nil, err
			}
			pins = append(pins, guarded)
			h, err := guarded.Raw()
			if err != nil {
				return nil, err
			}
			args = append(args, h)
		}
		h, err := g.context.manager.MkAndArgs(args)
		if err != nil {
			return nil, err
		}
		conjunction, err := NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
		pins = append(pins, conjunction)
		combined, err := formula.And(conjunction)
		if err != nil {
			return nil, err
		}
		pins = append(pins, combined)
		edge.dual, err = g.context.manager.CopyRef(combined.value.ref)
		if err != nil {
			return nil, err
		}
	} else {
		edge.dual, err = g.context.manager.CopyRef(formula.value.ref)
		if err != nil {
			return nil, err
		}
	}
	if err = childrenTimer.Close(); err != nil {
		return nil, err
	}
	persistTimer, err := startProfile(g.context, "Persisting")
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, persistTimer.Close()) }()
	if persist < 0 {
		persist = 0
	}
	if persist < len(g.scopes) {
		idx := len(g.scopes) - 1 - persist
		g.scopes[idx].edges = append(g.scopes[idx].edges, edge)
	}
	if err = persistTimer.Close(); err != nil {
		return nil, err
	}
	h, err := g.context.manager.RefGet(edge.dual)
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, h)
}
func (g *RPFP) AssertEdge(edge *Edge, persist int, withChildren, underapprox bool) (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	full, err := edge.F.IsFull()
	if err != nil {
		return err
	}
	if full && (!withChildren || len(edge.Children) == 0) {
		return nil
	}
	formula, err := g.GetEdgeFormula(edge, persist, withChildren, underapprox)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	h, err := formula.Raw()
	if err != nil {
		return err
	}
	assertTimer, err := startProfile(g.context, "solver add")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, assertTimer.Close()) }()
	if err = g.solver.Assert(h); err != nil {
		return err
	}
	if err = assertTimer.Close(); err != nil {
		return err
	}
	if withChildren {
		for _, child := range edge.Children {
			if err = g.ConstrainParent(edge, child); err != nil {
				return err
			}
		}
	}
	return nil
}
