package duality

// Source: duality_wrapper.cpp:519-600 (interpolating_solver::interpolate_tree).
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// InterpolateTree uses the native proof interpolator. In incremental mode the
// tree constraints must already be asserted in the bound solver.
func (g *RPFP) InterpolateTree(manager *interp.Manager, tree *TermTree, theory []*Expr, incremental bool, config *interp.RunConfig) (status lbool.LBool, out *TermTree, result error) {
	if err := g.solverReady(); err != nil {
		return lbool.Undef, nil, err
	}
	if manager == nil {
		return lbool.Undef, nil, ErrWrapperOwner
	}
	m, err := manager.ASTManager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	if m != g.context.manager {
		return lbool.Undef, nil, ErrContextMismatch
	}
	partitions, parents, err := LinearizeAssumptions(tree)
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() {
		result = errors.Join(result, CloseLinearizedAssumptions(g.context, &partitions))
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	parts := make([][]ast.ASTHandle, len(partitions))
	for n, refs := range partitions {
		for _, ref := range refs {
			h, err := m.RefGet(ref)
			if err != nil {
				return lbool.Undef, nil, err
			}
			parts[n] = append(parts[n], h)
		}
	}
	backgroundTerms := make([]*Expr, 0, len(theory))
	var generated []*Expr
	defer func() {
		for i := len(generated) - 1; i >= 0; i-- {
			result = errors.Join(result, generated[i].Close())
		}
	}()
	for _, term := range theory {
		normalized, changed, e := g.interpolationAxiom(manager, term)
		if e != nil {
			return lbool.Undef, nil, e
		}
		backgroundTerms = append(backgroundTerms, normalized)
		if changed {
			generated = append(generated, normalized)
		}
	}
	background := make([]ast.ASTHandle, len(backgroundTerms))
	for n, term := range backgroundTerms {
		if err := term.open(); err != nil {
			return lbool.Undef, nil, err
		}
		if term.value.object.context != g.context {
			return lbool.Undef, nil, ErrContextMismatch
		}
		background[n], err = term.Raw()
		if err != nil {
			return lbool.Undef, nil, err
		}
	}
	if !incremental {
		if err = g.solver.Push(); err != nil {
			return lbool.Undef, nil, err
		}
		defer func() { result = errors.Join(result, g.solver.Pop(1)) }()
		for _, part := range parts {
			for _, h := range part {
				if err = g.solver.Assert(h); err != nil {
					return lbool.Undef, nil, err
				}
			}
		}
	}
	proof, err := g.solver.Proof()
	if err != nil {
		return lbool.Undef, nil, err
	}
	status = lbool.False
	if proof.IsNull() {
		status, err = g.solver.CheckSat(nil)
		if err != nil {
			return status, nil, err
		}
	}
	if status == lbool.False {
		proof, err = g.solver.Proof()
		if err != nil {
			return status, nil, err
		}
		refs, err := interp.ProofInterpolants(manager, proof, parts, parents, background, config)
		if err != nil {
			return status, nil, err
		}
		defer func() {
			for n := range refs {
				result = errors.Join(result, m.DelRef(&refs[n]))
			}
		}()
		var facts []*Expr
		defer func() {
			for _, fact := range facts {
				result = errors.Join(result, fact.Close())
			}
		}()
		for _, ref := range refs {
			h, err := m.RefGet(ref)
			if err != nil {
				return status, nil, err
			}
			fact, err := NewExpr(g.context, h)
			if err != nil {
				return status, nil, err
			}
			facts = append(facts, fact)
		}
		out, err = UnlinearizeInterpolants(tree, facts)
		if err != nil {
			return status, nil, err
		}
	}
	md, err := g.solver.AcquireModel()
	if err != nil {
		return status, nil, err
	}
	result = g.replaceDualModel(md, false)
	return
}
