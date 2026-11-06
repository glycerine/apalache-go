package duality

// Source: duality_rpfp.cpp, VariableProjector::ProjectFormula.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// ProjectFormula applies the source cheap equality and inequality projection.
// The caller owns the returned expression; keep and formula are borrowed.
func (g *RPFP) ProjectFormula(keep []*Expr, formula *Expr, simplifier *interp.Manager) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	var literals, gaussian, projected []*Expr
	var output *Expr
	defer func() {
		for _, group := range [][]*Expr{projected, gaussian, literals} {
			for _, term := range group {
				result = errors.Join(result, term.Close())
			}
		}
		if result != nil && output != nil {
			result = errors.Join(result, output.Close())
			out = nil
		}
	}()
	var err error
	literals, err = CollectConjuncts(formula, true)
	if err != nil {
		return nil, err
	}
	timer, err := startProfile(g.context, "GaussElimCheap")
	if err != nil {
		return nil, err
	}
	gaussian, err = g.GaussElimCheap(keep, literals, simplifier)
	err = errors.Join(err, timer.Close())
	if err != nil {
		return nil, err
	}
	timer, err = startProfile(g.context, "FourierMotzkinCheap")
	if err != nil {
		return nil, err
	}
	projected, err = g.FourierMotzkinCheap(keep, gaussian, simplifier)
	err = errors.Join(err, timer.Close())
	if err != nil {
		return nil, err
	}
	m := g.context.manager
	args := make([]ast.ASTHandle, len(projected))
	for i, term := range projected {
		args[i], err = term.Raw()
		if err != nil {
			return nil, err
		}
	}
	var h ast.ASTHandle
	switch len(args) {
	case 0:
		h, err = m.MkTrue()
	case 1:
		h = args[0]
	default:
		h, err = m.MkAndArgs(args)
	}
	if err != nil {
		return nil, err
	}
	output, err = NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	return output, nil
}
