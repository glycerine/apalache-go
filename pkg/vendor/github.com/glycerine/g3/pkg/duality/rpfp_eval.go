package duality

// Source: duality_rpfp.cpp:1497-1526; duality_wrapper.h:749-754.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"io"
)

func (g *RPFP) BindModelEvaluator(config model.ModelEvaluatorConfig) error {
	if err := g.open(); err != nil {
		return err
	}
	if config.Context == nil || config.Context.SymbolsField != g.context.manager.Symbols {
		return ErrContextMismatch
	}
	g.evaluatorConfig = config
	return nil
}
func (g *RPFP) evaluate(term *Expr) (out *Expr, result error) {
	defer func() {
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	if err := g.open(); err != nil {
		return nil, err
	}
	if g.dualModel == nil {
		return nil, ErrWrapperOwner
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	m := g.context.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	ev, err := model.NewModelEvaluator(g.dualModel, g.evaluatorConfig)
	if ev != nil {
		defer func() {
			closeErr := ev.Close()
			if closeErr != nil {
				g.pendingEvaluators = append(g.pendingEvaluators, ev)
			}
			result = errors.Join(result, closeErr)
		}()
	}
	if err != nil {
		return nil, err
	}
	if err = ev.SetModelCompletion(true); err != nil {
		return nil, err
	}
	h, err := term.Raw()
	if err != nil {
		return nil, err
	}
	err = ev.Evaluate(h, ref)
	if rewriter.IsRewriterException(err) {
		tags := g.evaluatorConfig.Trace
		if tags == nil {
			tags = diagnostic.BorrowProcessTraceTags()
		}
		failure := err
		_ = tags.EmitConditional("model_evaluator", "eval", "src/model/model.cpp", 88, func() bool { return true }, func(w io.Writer) error { _, e := io.WriteString(w, failure.Error()+"\n"); return e })
		err = nil
	}
	if err != nil {
		return nil, err
	}
	h, err = m.RefGet(ref)
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, h)
}
func (g *RPFP) Eval(edge *Edge, term *Expr) (_ *Expr, result error) {
	localized, err := g.Localize(edge, term)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, localized.Close()) }()
	return g.evaluate(localized)
}
func (g *RPFP) Empty(node *Node) (_ bool, result error) {
	b, vars, err := g.RedVars(node)
	if err != nil {
		return false, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	value, err := g.evaluate(b)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, value.Close()) }()
	h, err := value.Raw()
	if err != nil {
		return false, err
	}
	return !g.context.manager.IsTrue(h), nil
}
func (g *RPFP) EvalNode(node *Node) (_ *Expr, result error) {
	b, vars, err := g.RedVars(node)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, b.Close())
		for _, v := range vars {
			result = errors.Join(result, v.Close())
		}
	}()
	var values []*Expr
	defer func() {
		for _, v := range values {
			result = errors.Join(result, v.Close())
		}
	}()
	for _, v := range vars {
		value, err := g.evaluate(v)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	h, err := g.context.manager.RefGet(node.Name)
	if err != nil {
		return nil, err
	}
	decl, err := NewFuncDecl(g.context, h)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, decl.Close()) }()
	return decl.Call(values...)
}
