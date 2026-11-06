package duality

// Source: duality_rpfp.cpp:992-1000,1193-1195,1211-1218,1276-1282.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// CachedGraphAssertions connects graph formula ownership to a borrowed
// dedicated solver/cache scope. Cached scope operations keep solver guards
// permanent; per-edge solver switching remains a separate operation.
type CachedGraphAssertions struct {
	active *CachedAssumptions
	graph  *RPFP
	scope  *EdgeSolverScope
}

func NewCachedGraphAssertions(graph *RPFP, scope *EdgeSolverScope) (*CachedGraphAssertions, error) {
	if err := graph.open(); err != nil {
		return nil, err
	}
	if scope == nil || scope.Solver == nil || scope.Assumptions == nil {
		return nil, ErrWrapperOwner
	}
	if graph.context != scope.Assumptions.context {
		return nil, ErrContextMismatch
	}
	if graph.solver == nil {
		if err := graph.BindSolver(scope.Solver); err != nil {
			return nil, err
		}
	} else if graph.solver != scope.Solver {
		return nil, ErrWrapperOwner
	}
	active, err := NewCachedAssumptions(scope)
	if err != nil {
		return nil, err
	}
	return &CachedGraphAssertions{graph: graph, scope: scope, active: active}, nil
}
func (c *CachedGraphAssertions) AssertEdgeCache(edge *Edge, literals *[]*Expr, withChildren bool) (result error) {
	if c == nil || literals == nil || edge == nil || edge.Owner != c.graph || edge.closed {
		return ErrWrapperOwner
	}
	full, err := edge.F.IsFull()
	if err != nil {
		return err
	}
	if full && (!withChildren || len(edge.Children) == 0) {
		return nil
	}
	formula, err := c.graph.GetEdgeFormula(edge, 0, withChildren, false)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	added, err := c.scope.Assumptions.GetAssumptionLits(formula, nil)
	if err != nil {
		return err
	}
	*literals = append(*literals, added...)
	if withChildren {
		for _, child := range edge.Children {
			if err = c.ConstrainParentCache(edge, child, literals); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *CachedGraphAssertions) ConstrainParentCache(parent *Edge, child *Node, literals *[]*Expr) (result error) {
	if c == nil {
		return ErrWrapperOwner
	}
	formula, err := c.graph.GetAnnotation(child)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	return c.ConstrainEdgeLocalizedCache(parent, formula, literals)
}
func (c *CachedGraphAssertions) ConstrainEdgeLocalizedCache(edge *Edge, term *Expr, literals *[]*Expr) (result error) {
	if c == nil || literals == nil || edge == nil || edge.Owner != c.graph || edge.closed {
		return ErrWrapperOwner
	}
	if err := term.open(); err != nil {
		return err
	}
	if term.value.object.context != c.graph.context {
		return ErrContextMismatch
	}
	added, err := c.scope.Assumptions.GetAssumptionLits(term, nil)
	if err != nil {
		return err
	}
	var closeErr error
	defer func() {
		if added != nil {
			for _, literal := range added {
				closeErr = errors.Join(closeErr, literal.Close())
			}
		}
		result = errors.Join(result, closeErr)
	}()
	m := c.graph.context.manager
	owned, err := m.CopyRef(term.value.ref)
	if err != nil {
		return err
	}
	recorded, err := m.CopyRef(owned)
	if err != nil {
		return errors.Join(err, m.DelRef(&owned))
	}
	edge.constraints = append(edge.constraints, owned)
	last := &c.graph.scopes[len(c.graph.scopes)-1]
	last.constraints = append(last.constraints, edgeConstraint{edge, recorded})
	*literals = append(*literals, added...)
	added = nil
	return nil
}

// AssertNodeCache intentionally does not append to the caller's vector: the
// source takes that vector by value. Generated guards remain cached, while
// the local literal copies are destroyed at return.
func (c *CachedGraphAssertions) AssertNodeCache(node *Node, literals []*Expr) (result error) {
	if c == nil || node == nil || node.Owner != c.graph || node.closed {
		return ErrWrapperOwner
	}
	if node.dual != (ast.ASTRef{}) {
		return nil
	}
	formula, err := c.graph.GetUpperBound(node)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	added, err := c.scope.Assumptions.GetAssumptionLits(formula, nil)
	if err != nil {
		return err
	}
	defer func() {
		for _, literal := range added {
			result = errors.Join(result, literal.Close())
		}
	}()
	node.dual, err = c.graph.context.manager.CopyRef(formula.value.ref)
	if err != nil {
		return err
	}
	last := &c.graph.scopes[len(c.graph.scopes)-1]
	last.nodes = append(last.nodes, node)
	return nil
}
