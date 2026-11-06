package duality

// Source: duality_rpfp.cpp:1447-1493.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// replaceDualModel consumes an acquired model reference. CheckUpdateModel
// preserves the previous model when the solver has no new model.
func (g *RPFP) replaceDualModel(next *model.ModelCore, keepOnNull bool) error {
	if next == nil && keepOnNull {
		return nil
	}
	if g.dualModel != nil {
		consumed, err := g.dualModel.DecRefConsumed()
		if consumed {
			g.dualModel = nil
		}
		if err != nil {
			if next != nil {
				err = errors.Join(err, next.DecRef())
			}
			return err
		}
	}
	g.dualModel = next
	return nil
}

// Check uses the source preliminary unassumed check before an assumption
// check. If core is nonnil, its entries follow the native solver core order.
// A check with no underapproximations leaves the supplied core unchanged.
func (g *RPFP) Check(root *Node, underapproximations []*Node, core *[]*Node) (status lbool.LBool, result error) {
	if err := g.solverReady(); err != nil {
		return lbool.Undef, err
	}
	timer, err := startProfile(g.context, "Check")
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	if err := g.ClearProofCore(); err != nil {
		return lbool.Undef, err
	}
	var flags []*Expr
	defer func() {
		for _, flag := range flags {
			result = errors.Join(result, flag.Close())
		}
	}()
	assumptions := make([]ast.ASTHandle, 0, len(underapproximations))
	for _, node := range underapproximations {
		flag, err := g.UnderapproxFlag(node)
		if err != nil {
			return lbool.Undef, err
		}
		flags = append(flags, flag)
		h, err := flag.Raw()
		if err != nil {
			return lbool.Undef, err
		}
		assumptions = append(assumptions, h)
	}
	if len(assumptions) > 0 {
		if _, err := g.solver.CheckSat(nil); err != nil {
			return lbool.Undef, err
		}
	}
	status, err = g.solver.CheckSat(assumptions)
	if err != nil {
		return status, err
	}
	if len(assumptions) > 0 && core != nil {
		var translated []*Node
		if status == lbool.False {
			native, err := g.solver.UnsatCore()
			if err != nil {
				return status, err
			}
			for _, h := range native {
				translated = append(translated, g.underapproxFlags[h])
			}
		}
		*core = translated
	}
	md, err := g.solver.AcquireModel()
	if err != nil {
		return status, err
	}
	return status, g.replaceDualModel(md, false)
}
func (g *RPFP) CheckUpdateModel(root *Node, assumptions []*Expr) (lbool.LBool, error) {
	if err := g.solverReady(); err != nil {
		return lbool.Undef, err
	}
	if err := g.ClearProofCore(); err != nil {
		return lbool.Undef, err
	}
	handles := make([]ast.ASTHandle, len(assumptions))
	for n, term := range assumptions {
		if err := term.open(); err != nil {
			return lbool.Undef, err
		}
		if term.value.object.context != g.context {
			return lbool.Undef, ErrContextMismatch
		}
		h, err := term.Raw()
		if err != nil {
			return lbool.Undef, err
		}
		handles[n] = h
	}
	status, err := g.solver.CheckSat(handles)
	if err != nil {
		return status, err
	}
	md, err := g.solver.AcquireModel()
	if err != nil {
		return status, err
	}
	return status, g.replaceDualModel(md, true)
}

// ClearProofCore releases the owning source set before a new solver result.
func (g *RPFP) ClearProofCore() error {
	if err := g.open(); err != nil {
		return err
	}
	for key, ref := range g.proofCore {
		err := g.context.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(g.proofCore, key)
		} else {
			g.proofCore[key] = ref
		}
		if err != nil {
			return err
		}
	}
	g.proofCore = nil
	return nil
}
