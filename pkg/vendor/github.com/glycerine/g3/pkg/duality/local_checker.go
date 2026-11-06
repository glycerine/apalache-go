package duality

// Source: duality_solver.cpp:1657-1686; duality.h:1346-1360.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// LocalEdgeChecker owns the cloned RPFP, dedicated solvers and background
// axiom copies used by cached local inference. The source graph/factory are borrowed.
type LocalEdgeChecker struct {
	graph  *RPFP
	clones *RPFPCloneMap
	pool   *EdgeSolverPool
	axioms []*Expr
	closed bool
}

func NewLocalEdgeChecker(source *RPFP, factory solver.Factory, axioms []*Expr) (_ *LocalEdgeChecker, result error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	c := &LocalEdgeChecker{}
	defer func() {
		if result != nil {
			result = errors.Join(result, c.Close())
		}
	}()
	var err error
	c.graph, err = NewRPFPWithVocabulary(source.vocabulary)
	if err != nil {
		return nil, err
	}
	c.clones, err = NewRPFPCloneMap(c.graph)
	if err != nil {
		return nil, err
	}
	if err = c.clones.Clone(source); err != nil {
		return nil, err
	}
	c.pool, err = NewEdgeSolverPool(source.context, factory)
	if err != nil {
		return nil, err
	}
	for _, axiom := range axioms {
		if err = axiom.open(); err != nil {
			return nil, err
		}
		if axiom.value.object.context != source.context {
			return nil, ErrContextMismatch
		}
		copy, err := axiom.Clone()
		if err != nil {
			return nil, err
		}
		c.axioms = append(c.axioms, copy)
	}
	return c, nil
}
func (c *LocalEdgeChecker) CheckEdgeCaching(unwinding *Edge, bound *Transformer) (status lbool.LBool, result error) {
	if c == nil || c.closed || unwinding == nil || unwinding.closed || unwinding.Map == nil {
		return lbool.Undef, ErrWrapperOwner
	}
	if err := bound.open(); err != nil {
		return lbool.Undef, err
	}
	if bound.context != c.graph.context {
		return lbool.Undef, ErrContextMismatch
	}
	source := unwinding.Map
	edge, err := c.clones.GetEdgeClone(source)
	if err != nil {
		return lbool.Undef, err
	}
	if edge == nil {
		return lbool.Undef, ErrWrapperOwner
	}
	state, err := c.pool.SolverForEdge(source, true, true, c.axioms)
	if err != nil {
		return lbool.Undef, err
	}
	scope, err := state.Scope()
	if err != nil {
		return lbool.Undef, err
	}
	previous := c.graph.solver
	c.graph.solver = scope.Solver
	if c.graph.scopes == nil {
		c.graph.scopes = []graphScope{{}}
	}
	defer func() { c.graph.solver = previous; result = errors.Join(result, scope.Close()) }()
	copied, err := bound.Clone()
	if err != nil {
		return lbool.Undef, err
	}
	if err = edge.Parent.Bound.Close(); err != nil {
		return lbool.Undef, errors.Join(err, copied.Close())
	}
	edge.Parent.Bound = copied
	if len(unwinding.Children) != len(edge.Children) {
		return lbool.Undef, ast.ErrASTDomain
	}
	for n, child := range edge.Children {
		if err = replaceNodeAnnotation(child, unwinding.Children[n].Annotation); err != nil {
			return lbool.Undef, err
		}
	}
	cached, err := NewCachedGraphAssertions(c.graph, scope)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, cached.Close()) }()
	if err = cached.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, cached.Pop(1)) }()
	if err = cached.AssertNode(edge.Parent); err != nil {
		return lbool.Undef, err
	}
	if err = cached.AssertEdge(edge, 1, true, false); err != nil {
		return lbool.Undef, err
	}
	status, core, err := cached.Check(nil, false)
	for _, term := range core {
		err = errors.Join(err, term.Close())
	}
	return status, err
}
func (c *LocalEdgeChecker) Close() error {
	if c == nil || c.closed {
		return nil
	}
	if c.graph != nil {
		if err := c.graph.Close(); err != nil {
			return err
		}
		c.graph = nil
	}
	if c.pool != nil {
		if err := c.pool.Close(); err != nil {
			return err
		}
		c.pool = nil
	}
	for len(c.axioms) > 0 {
		n := len(c.axioms) - 1
		if err := c.axioms[n].Close(); err != nil {
			return err
		}
		c.axioms = c.axioms[:n]
	}
	c.closed = true
	return nil
}
