package duality

// Source: src/muz/duality/duality_dl_interface.cpp, dl_interface::get_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// AnnotationModel returns an owned model of the graph's current annotations.
// It abstracts parameter j to variable j, following the source's reversed
// expr_abstract signature, and publishes predicates in graph order.
func (g *RPFP) AnnotationModel() (out *model.Model, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	m := g.context.manager
	created, err := model.NewModel(m)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, created.Close())
			out = nil
		}
	}()
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, abstractor.Close()) }()
	value, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&value)) }()
	for _, node := range g.nodes {
		if err := g.registerAnnotationModel(created, abstractor, value, node); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (g *RPFP) registerAnnotationModel(md *model.Model, abstractor *ast.ExprAbstractor, value ast.ASTRef, node *Node) (result error) {
	m := g.context.manager
	decl, err := m.RefGet(node.Name)
	if err != nil {
		return err
	}
	formula, err := m.RefGet(node.Annotation.formula)
	if err != nil {
		return err
	}
	params := node.Annotation.individualParams
	bound := make([]ast.ASTHandle, len(params))
	for i := range params {
		bound[i], err = m.RefGet(params[len(params)-1-i])
		if err != nil {
			return err
		}
	}
	if err := abstractor.Abstract(0, bound, formula, value); err != nil {
		return err
	}
	term, err := m.RefGet(value)
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return md.Core().RegisterConstant(decl, term)
	}
	fi, err := model.NewFunctionInterpretation(m, uint32(len(params)))
	if err != nil {
		return err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, fi.Close())
		}
	}()
	if err := fi.SetElse(term); err != nil {
		return err
	}
	return md.Core().RegisterFunction(decl, fi)
}
