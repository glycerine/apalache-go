package duality

// Source: duality_rpfp.cpp:1343-1372,1418-1420.
import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (g *RPFP) GetConstraintTree(root, skip *Node) (*TermTree, error) {
	tree, err := g.ToTermTree(root, skip)
	if err != nil {
		return nil, err
	}
	wrapped, err := g.AddUpperBound(root, tree)
	if err != nil {
		return nil, errors.Join(err, tree.Close())
	}
	return wrapped, nil
}

// Solve interpolates the currently asserted graph and updates its node
// annotations on UNSAT. SAT leaves the acquired counterexample model owned
// by the graph. The source persist argument affects only disabled code in
// DecodeTree and is therefore unnecessary here.
func (g *RPFP) Solve(root *Node, manager *interp.Manager, config *interp.RunConfig) (status lbool.LBool, result error) {
	return g.solveWithTheory(root, manager, nil, config)
}

// solveWithTheory supplies the background axioms recorded by the owning
// Duality engine to proof interpolation. Direct graph callers retain the
// source-compatible empty-background Solve surface above.
func (g *RPFP) solveWithTheory(root *Node, manager *interp.Manager, theory []*Expr, config *interp.RunConfig) (status lbool.LBool, result error) {
	if err := g.open(); err != nil {
		return lbool.Undef, err
	}
	timer, err := startProfile(g.context, "Solve")
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	tree, err := g.GetConstraintTree(root, nil)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, tree.Close()) }()
	if err = g.ClearProofCore(); err != nil {
		return lbool.Undef, err
	}
	interpolationTimer, err := startProfile(g.context, "interpolate_tree")
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, interpolationTimer.Close()) }()
	status, learned, err := g.InterpolateTree(manager, tree, theory, true, config)
	err = errors.Join(err, interpolationTimer.Close())
	if learned != nil {
		defer func() { result = errors.Join(result, learned.Close()) }()
	}
	if err != nil {
		return status, err
	}
	if status == lbool.False {
		if learned == nil || len(learned.children) != 1 {
			return status, ErrWrapperOwner
		}
		result = g.DecodeTree(root, learned.children[0])
	}
	return
}
