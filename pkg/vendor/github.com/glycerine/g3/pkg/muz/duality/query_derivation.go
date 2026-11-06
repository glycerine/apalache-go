package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

// WriteDerivation prints the reachable query's child-first rule steps.
func (b *Backend) WriteDerivation(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error) error {
	if err := b.open(); err != nil {
		return err
	}
	e := b.queries
	if e.active == nil || e.status != lbool.True {
		return graph.ErrWrapperOwner
	}
	if len(e.active.background) > 0 || len(e.config.Theory) > 0 {
		return graph.ErrProofWitness
	}
	return e.context.WithProfiler(e.config.Profiler, func() error {
		return e.active.engine.GetCounterexample().WriteDerivation(out, writeExpr, e.config.Factory)
	})
}

// WriteCertificate prints the source fixedpoint model for a safe result, or
// the derivation and filtered V2 counterexample model for a reachable result.
func (b *Backend) WriteCertificate(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error) (result error) {
	if err := b.open(); err != nil {
		return err
	}
	e := b.queries
	if e.active == nil || e.status == lbool.Undef {
		return graph.ErrWrapperOwner
	}
	if e.status == lbool.False {
		md, err := e.GetModel()
		if err != nil {
			return err
		}
		scope := &certificateModelScope{snapshot: md, runtime: e.config.Evaluator.Memory}
		defer func() { result = scope.finish(result) }()
		if _, err = io.WriteString(out, "(fixedpoint \n"); err != nil {
			return err
		}
		if err = md.Core().WriteSMT2(out, e.config.Evaluator.Context, e.config.Evaluator.Globals, 0); err != nil {
			return err
		}
		_, err = io.WriteString(out, ")\n")
		return err
	}
	background := append([]*graph.Expr(nil), e.config.Theory...)
	background = append(background, e.active.background...)
	return e.context.WithProfiler(e.config.Profiler, func() error {
		return e.active.engine.GetCounterexample().WriteCertificate(out, writeExpr, e.config.Factory, e.config.Evaluator.Context, e.config.Evaluator.Globals, background...)
	})
}
