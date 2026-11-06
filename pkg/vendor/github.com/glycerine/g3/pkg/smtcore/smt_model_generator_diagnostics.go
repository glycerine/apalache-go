package smtcore

// Source: retained TRACE/CTRACE and SASSERT diagnostics in
// src/smt/smt_model_generator.cpp.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (g *SMTModelGenerator) traceTags() *diagnostic.TraceTags {
	if g != nil && g.trace != nil {
		return g.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (g *SMTModelGenerator) traceModel(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	if g == nil {
		return nil
	}
	tags := g.traceTags()
	err := tags.EmitConditional(tag, function, "src/smt/smt_model_generator.cpp", line, condition, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (g *SMTModelGenerator) displayModelExpression(out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(g.manager, out, out)
	if err != nil {
		return err
	}
	kind, err := g.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind == ast.ASTSort {
		return printer.DisplaySort(out, expression, 0)
	}
	if kind == ast.ASTFuncDecl {
		return printer.DisplayFuncDecl(out, expression, 0)
	}
	return printer.DisplayExpr(out, expression, 0)
}

func (g *SMTModelGenerator) sourceText(key smtModelSourceKey) (string, error) {
	if key.fresh != nil {
		return fmt.Sprintf("fresh!%d", key.fresh.index), nil
	}
	if key.node == nil {
		return "", ast.ErrASTDomain
	}
	id, err := key.node.OwnerID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%d", id), nil
}

func (g *SMTModelGenerator) traceProcessSource(line int, prefix string, key smtModelSourceKey, todo int) error {
	return g.traceModel("mg_top_sort", "process_source", line, func() bool { return true }, func(out io.Writer) error {
		text, err := g.sourceText(key)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "%s, is_fresh: %t %s, todo.size(): %d\n", prefix, key.fresh != nil, text, todo)
		return err
	})
}

func (g *SMTModelGenerator) traceDependency(parent *SMTEnode, child smtModelSourceKey, line int) error {
	return g.traceModel("mg_top_sort", "visit_children", line, func() bool { return true }, func(out io.Writer) error {
		id, err := parent.OwnerID()
		if err != nil {
			return err
		}
		text, err := g.sourceText(child)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "#%d -> %s\n", id, text)
		return err
	})
}

func (g *SMTModelGenerator) traceSortedSources(sources []SMTModelValueDependency, procedures map[*SMTEnode]SMTModelValueProcedure) error {
	return g.traceModel("sorted_sources", "mk_values", 310, func() bool { return true }, func(out io.Writer) error {
		for _, source := range sources {
			key, err := smtModelDependencyKey(source)
			if err != nil {
				return err
			}
			text, err := g.sourceText(key)
			if err != nil {
				return err
			}
			var sort ast.ASTHandle
			fresh := false
			if key.fresh != nil {
				sort = key.fresh.sort
				fresh = true
			} else {
				if err = g.sourceAssertion(320, key.node.root == key.node, "n->get_root() == n"); err != nil {
					return err
				}
				sort, err = g.manager.GetSort(key.node.owner)
				if err != nil {
					return err
				}
				procedure := procedures[key.node]
				if err = g.sourceAssertion(325, procedure != nil, "proc"); err != nil {
					return err
				}
				if procedure == nil {
					return ast.ErrASTDomain
				}
				fresh = procedure.IsFresh()
			}
			if _, err = fmt.Fprintf(out, "%s ", text); err != nil {
				return err
			}
			if err = g.displayModelExpression(out, sort); err != nil {
				return err
			}
			if key.fresh != nil {
				_, err = io.WriteString(out, "\n")
			} else {
				_, err = fmt.Fprintf(out, " is_fresh: %t\n", fresh)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *SMTModelGenerator) traceFreshValue(fresh *SMTExtraFreshValue, value ast.ASTHandle) error {
	if err := g.traceModel("model_fresh_bug", "mk_values", 336, func() bool { return true }, func(out io.Writer) error {
		if _, err := fmt.Fprintf(out, "mk fresh!%d : ", fresh.index); err != nil {
			return err
		}
		if err := g.displayModelExpression(out, fresh.sort); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	return g.traceModel("model_fresh_bug", "mk_values", 338, func() bool { return true }, func(out io.Writer) error {
		id := ^uint32(0)
		var err error
		if !value.IsNull() {
			id, err = g.manager.GetASTID(value)
		}
		if err == nil {
			_, err = fmt.Fprintf(out, "mk fresh!%d := #%d\n", fresh.index, id)
		}
		return err
	})
}

func (g *SMTModelGenerator) sourceAssertion(line int, condition bool, text string) error {
	if !diagnostic.DebugBuild || condition {
		return nil
	}
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	return diagnostic.DebugAssertionFailure(debug, "src/smt/smt_model_generator.cpp", int32(line), text)
}
