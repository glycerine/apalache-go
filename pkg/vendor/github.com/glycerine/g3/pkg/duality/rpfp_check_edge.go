package duality

// Source: duality_solver.cpp:1657-1665 (CheckEdge).
import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CheckEdge checks a local rule with its child annotations. The cached edge
// formula persists across the temporary pop so repeated checks can reuse it.
func (g *RPFP) CheckEdge(edge *Edge) (status lbool.LBool, result error) {
	if err := g.solverReady(); err != nil {
		return lbool.Undef, err
	}
	if edge == nil || edge.Owner != g || edge.closed || edge.Parent == nil {
		return lbool.Undef, ErrWrapperOwner
	}
	if err := g.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, g.Pop(1)) }()
	if err := g.AssertNode(edge.Parent); err != nil {
		return lbool.Undef, err
	}
	if err := g.AssertEdge(edge, 1, true, false); err != nil {
		return lbool.Undef, err
	}
	return g.Check(edge.Parent, nil, nil)
}
