package core

// Source: src/tactic/core/ctx_simplify_tactic.cpp retained diagnostic sites.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (t *ContextSimplifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *ContextSimplifyTactic) assertContext(condition bool) error {
	on, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil {
		return err
	}
	if on && !condition {
		return ast.ErrASTDomain
	}
	return nil
}
func (t *ContextSimplifyTactic) checkContextCache(r *contextSimplifyImplementation) error {
	for key, cell := range r.cache {
		if len(cell.results) == 0 {
			return ast.ErrASTDomain
		}
		from, err := t.manager.RefGet(cell.key)
		if err != nil {
			return err
		}
		if from != key {
			return ast.ErrASTDomain
		}
		for _, entry := range cell.results {
			if entry.level > r.simp.ScopeLevel() {
				return ast.ErrASTDomain
			}
		}
	}
	return nil
}

func (t *ContextSimplifyTactic) checkDestroyedContextCache(r *contextSimplifyImplementation) error {
	if !ast.ASTSourceDebug || len(r.cache) == 0 {
		return nil
	}
	type remaining struct {
		id   uint32
		key  ast.ASTHandle
		cell *contextCacheCell
	}
	var cells []remaining
	for key, cell := range r.cache {
		id, err := t.manager.GetASTID(key)
		if err != nil {
			return err
		}
		cells = append(cells, remaining{id, key, cell})
	}
	sort.Slice(cells, func(i, j int) bool { return cells[i].id < cells[j].id })
	for _, x := range cells {
		if err := t.emit("ctx_simplify_tactic_bug", "~imp", 186, func(w io.Writer) error {
			if _, err := fmt.Fprintf(w, "i: %d\n", x.id); err != nil {
				return err
			}
			if err := t.writeContextExpr(w, x.key, false); err != nil {
				return err
			}
			if len(x.cell.results) == 0 {
				_, err := io.WriteString(w, "\nm_result: 0\n")
				return err
			}
			entry := &x.cell.results[len(x.cell.results)-1]
			_, err := fmt.Fprintf(w, "\nm_result: %p\nlvl: %d\n", entry, entry.level)
			return err
		}); err != nil {
			return err
		}
		if err := t.assertContext(x.cell.key == (ast.ASTRef{}) && len(x.cell.results) == 0); err != nil {
			return err
		}
	}
	return nil
}
func (t *ContextSimplifyTactic) traceNullArgument(h ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_bug", "simplify_app", 491, func(w io.Writer) error {
		if err := t.writeContextExpr(w, h, false); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceTags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}
func (t *ContextSimplifyTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.traceTags()
	err := tags.EmitConditional(tag, function, "src/tactic/core/ctx_simplify_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *ContextSimplifyTactic) writeContextExpr(w io.Writer, h ast.ASTHandle, bounded bool) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	if bounded {
		err = t.manager.DisplayBoundedLowLevel(stream, h, 3)
	} else {
		format, e := t.manager.SMT2PPExprTree(h, nil)
		if e != nil {
			return e
		}
		fm, e := t.manager.GetFormatManager()
		if e != nil {
			return e
		}
		held, e := fm.NewRef(format)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, fm.DelRef(&held)) }()
		empty := t.context.EmptyValue()
		err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals)
	}
	if err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}
func (t *ContextSimplifyTactic) traceShared(h ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_bug", "shared", 213, func(w io.Writer) error {
		if err := t.writeContextExpr(w, h, false); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceCache(from, to ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_cache", "cache_core", 234, func(w io.Writer) error {
		id, err := t.manager.GetASTID(from)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "caching %d @ %d\n", id, t.current.simp.ScopeLevel()); err != nil {
			return err
		}
		if err = t.writeContextExpr(w, from, false); err != nil {
			return err
		}
		if _, err = io.WriteString(w, "\n--->\n"); err != nil {
			return err
		}
		if err = t.writeContextExpr(w, to, false); err != nil {
			return err
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceRestore(key ast.ASTHandle, entry contextCacheResult, next ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_cache", "restore_cache", 279, func(w io.Writer) error {
		if _, err := fmt.Fprintf(w, "uncaching: %d\n", entry.level); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, key, false); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n--->\n"); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, entry.value, false); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\nrestoring:\n"); err != nil {
			return err
		}
		if next.IsNull() {
			if _, err := io.WriteString(w, "<null>"); err != nil {
				return err
			}
		} else if err := t.writeContextExpr(w, next, false); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceProcessing(h ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_detail", "simplify", 335, func(w io.Writer) error {
		if _, err := io.WriteString(w, "processing: "); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, h, true); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceSimplified(from, to ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_detail", "simplify", 352, func(w io.Writer) error {
		if _, err := io.WriteString(w, "result:\n"); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, from, true); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n---->\n"); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, to, true); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceITE(c, a, b ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic_ite_bug", "simplify_ite", 470, func(w io.Writer) error {
		if _, err := io.WriteString(w, "mk_ite\n"); err != nil {
			return err
		}
		for _, h := range []ast.ASTHandle{c, a, b} {
			if err := t.writeContextExpr(w, h, false); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}
func (t *ContextSimplifyTactic) traceProcess(h ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic", "process", 559, func(w io.Writer) error {
		if _, err := io.WriteString(w, "simplifying:\n"); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, h, false); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (t *ContextSimplifyTactic) traceProcessResult(from, to ast.ASTHandle) error {
	return t.emit("ctx_simplify_tactic", "process", 566, func(w io.Writer) error {
		if _, err := io.WriteString(w, "result\n"); err != nil {
			return err
		}
		if err := t.writeContextExpr(w, to, false); err != nil {
			return err
		}
		before, err := t.expressionSize(from)
		if err != nil {
			return err
		}
		after, err := t.expressionSize(to)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, " :num-steps %d\nold size: %d new size: %d\n", t.current.steps, before, after)
		return err
	})
}
